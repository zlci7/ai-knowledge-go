package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/llm"
	"context"
	"errors"
	"strings"
	"time"
)

// streamConversationLockTTL 为流式输出预留更长锁时间，覆盖完整生成周期。
const streamConversationLockTTL = 5 * time.Minute

// ChatStreamEvent 定义流式响应事件格式，供 SSE/WebSocket 统一消费。
type ChatStreamEvent struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Delta          string `json:"delta,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ChatStream 以事件流方式返回模型增量输出，并在结束后持久化完整回复。
func (s *ChatService) ChatStream(ctx context.Context, userID uint64, req dto.ChatReq) (<-chan ChatStreamEvent, error) {
	llmContext, unlock, err := s.prepareChatContext(ctx, userID, &req, streamConversationLockTTL)
	if err != nil {
		return nil, err
	}

	streamEvents := make(chan ChatStreamEvent, 32)
	llmChunks := make(chan llm.StreamChunk, 32)

	go llm.GenerateFromMessagesStream(ctx, llmContext, llmChunks)

	go func(convID string, unlockFn func()) {
		defer close(streamEvents)
		defer unlockFn()

		if !emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
			Type:           "meta",
			ConversationID: convID,
		}) {
			return
		}

		var replyBuilder strings.Builder
		hasDone := false

		for chunk := range llmChunks {
			if chunk.Err != nil {
				if errors.Is(chunk.Err, context.Canceled) || errors.Is(chunk.Err, context.DeadlineExceeded) {
					return
				}
				emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
					Type:           "error",
					ConversationID: convID,
					Error:          chunk.Err.Error(),
				})
				return
			}

			if chunk.Delta != "" {
				replyBuilder.WriteString(chunk.Delta)
				if !emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
					Type:           "delta",
					ConversationID: convID,
					Delta:          chunk.Delta,
				}) {
					return
				}
			}

			if chunk.Done {
				hasDone = true
				break
			}
		}

		reply := replyBuilder.String()
		if !hasDone && strings.TrimSpace(reply) == "" {
			emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
				Type:           "error",
				ConversationID: convID,
				Error:          "llm stream ended unexpectedly",
			})
			return
		}

		if err := s.persistAssistantReply(ctx, convID, reply); err != nil {
			emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
				Type:           "error",
				ConversationID: convID,
				Error:          err.Error(),
			})
			return
		}

		emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
			Type:           "done",
			ConversationID: convID,
		})
	}(req.ConversationID, unlock)

	return streamEvents, nil
}

// emitStreamEvent 在上下文未取消时发送流事件，发送失败返回 false 终止上游流程。
func emitStreamEvent(ctx context.Context, out chan<- ChatStreamEvent, event ChatStreamEvent) bool {
	select {
	case out <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
