package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
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

type ChatService struct{}

// Chat 提供单轮非流式对话能力。
var Chat = new(ChatService)

// chatConversationLockTTL 控制会话锁时长，避免并发写入同一会话导致上下文错乱。
const chatConversationLockTTL = 10 * time.Second

// Chat 处理用户消息，生成助手回复并写回存储层。
func (s *ChatService) Chat(ctx context.Context, userID uint64, req dto.ChatReq) (*vo.ChatResp, error) {
	llmContext, unlock, err := s.prepareChatContext(ctx, userID, &req, chatConversationLockTTL)
	if err != nil {
		return nil, err
	}
	defer unlock()

	reply, err := llm.GenerateFromMessages(llmContext)
	if err != nil {
		return nil, err
	}

	if err := s.persistAssistantReply(ctx, req.ConversationID, reply); err != nil {
		return nil, err
	}

	return &vo.ChatResp{
		ConversationID: req.ConversationID,
		Reply:          reply,
	}, nil
}

// prepareChatContext 确保会话存在并构建本轮调用 LLM 所需的上下文消息。
func (s *ChatService) prepareChatContext(ctx context.Context, userID uint64, req *dto.ChatReq, lockTTL time.Duration) ([]llm.Message, func(), error) {
	if err := s.ensureConversation(ctx, userID, req); err != nil {
		return nil, nil, err
	}

	unlock, err := redis.AcquireConversationLock(ctx, req.ConversationID, lockTTL)
	if err != nil {
		return nil, nil, err
	}

	msgs, summary, err := redis.AddAndGetContext(ctx, req.ConversationID, redis.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		unlock()
		return nil, nil, err
	}

	// 放在 AddAndGetContext 后，避免 Redis 冷启动时回填 + 追加导致重复。
	if err := Save2Mysql(ctx, req.ConversationID, "user", req.Message); err != nil {
		unlock()
		return nil, nil, err
	}

	return s.buildLLMContext(ctx, userID, req.Message, msgs, summary), unlock, nil
}

// ensureConversation 在请求未携带会话 ID 时创建新会话并回填到请求对象。
func (s *ChatService) ensureConversation(ctx context.Context, userID uint64, req *dto.ChatReq) error {
	if req.ConversationID != "" {
		return nil
	}

	title, err := llm.GenerateNewTitle(req.Message)
	if err != nil {
		return err
	}

	convID := idgen.GenStringID()
	if err := mysql.Conversation.Create(ctx, &model.Conversation{
		ConvID:    convID,
		Title:     title,
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDeleted: false,
	}); err != nil {
		return err
	}

	req.ConversationID = convID
	return nil
}

// buildLLMContext 组装系统提示、长期记忆、摘要与历史消息，形成完整模型输入。
func (s *ChatService) buildLLMContext(ctx context.Context, userID uint64, userMessage string, msgs []redis.Message, summary string) []llm.Message {
	llmContext := []llm.Message{{Role: "system", Content: "You are a helpful assistant."}}

	retrievalCtx, cancel := context.WithTimeout(ctx, longTermMemoryBudgetTimeout*time.Millisecond)
	longTermMemories, err := retrieveLongTermMemories(retrievalCtx, userID, userMessage)
	cancel()
	if err == nil && len(longTermMemories) > 0 {
		longTermMemoryPrompt := formatLongTermMemorySystemPrompt(longTermMemories)
		if longTermMemoryPrompt != "" {
			llmContext = append(llmContext, llm.Message{Role: "system", Content: longTermMemoryPrompt})
		}
	}

	if summary != "" {
		llmContext = append(llmContext, llm.Message{Role: "system", Content: "[对话摘要]: " + summary})
	}

	// RAG 注入位：当前未实现检索时不注入占位内容，避免脏提示词。
	ragContext := ""
	if ragContext != "" {
		llmContext = append(llmContext, llm.Message{Role: "system", Content: "[RAG召回]: " + ragContext})
	}

	for _, msg := range msgs {
		llmContext = append(llmContext, llm.Message{Role: msg.Role, Content: msg.Content})
	}

	return llmContext
}

// persistAssistantReply 校验并落库助手回复，同时同步写入 Redis 会话上下文。
func (s *ChatService) persistAssistantReply(ctx context.Context, convID, reply string) error {
	if strings.TrimSpace(reply) == "" {
		return errors.New("empty assistant reply")
	}

	if err := Save2Mysql(ctx, convID, "assistant", reply); err != nil {
		return err
	}

	return redis.SaveAssistantReply(ctx, convID, redis.Message{
		Role:    "assistant",
		Content: reply,
	})
}
