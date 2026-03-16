package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/pkg/idgen"
	"ai-knowledge-go/internal/repository/mysql"
	"ai-knowledge-go/internal/repository/redis"
	"context"
	"errors"
	"strings"
	"time"
)

const streamConversationLockTTL = 5 * time.Minute

type ChatStreamEvent struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Delta          string `json:"delta,omitempty"`
	Error          string `json:"error,omitempty"`
}

func (s *ChatService) ChatStream(ctx context.Context, req dto.ChatReq) (<-chan ChatStreamEvent, error) {
	if req.ConversationID == "" {
		title, err := llm.GenerateNewTitle(req.Message)
		if err != nil {
			return nil, err
		}
		convID := idgen.GenStringID()
		err = mysql.Conversation.Create(ctx, &model.Conversation{
			ConvID:    convID,
			Title:     title,
			UserID:    0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			IsDeleted: false,
		})
		if err != nil {
			return nil, err
		}
		req.ConversationID = convID
	}

	unlock, err := redis.AcquireConversationLock(ctx, req.ConversationID, streamConversationLockTTL)
	if err != nil {
		return nil, err
	}

	msgs, summary, err := redis.AddAndGetContext(ctx, req.ConversationID, redis.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		unlock()
		return nil, err
	}

	if err := Save2Mysql(ctx, req.ConversationID, "user", req.Message); err != nil {
		unlock()
		return nil, err
	}

	llmContext := []llm.Message{{Role: "system", Content: "You are a helpful assistant."}}
	if summary != "" {
		llmContext = append(llmContext, llm.Message{Role: "system", Content: "[对话摘要]: " + summary})
	}
	for _, msg := range msgs {
		llmContext = append(llmContext, llm.Message{Role: msg.Role, Content: msg.Content})
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

		if strings.TrimSpace(reply) == "" {
			emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
				Type:           "error",
				ConversationID: convID,
				Error:          "empty assistant reply",
			})
			return
		}

		if err := Save2Mysql(ctx, convID, "assistant", reply); err != nil {
			emitStreamEvent(ctx, streamEvents, ChatStreamEvent{
				Type:           "error",
				ConversationID: convID,
				Error:          err.Error(),
			})
			return
		}

		if err := redis.SaveAssistantReply(ctx, convID, redis.Message{
			Role:    "assistant",
			Content: reply,
		}); err != nil {
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

func emitStreamEvent(ctx context.Context, out chan<- ChatStreamEvent, event ChatStreamEvent) bool {
	select {
	case out <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
