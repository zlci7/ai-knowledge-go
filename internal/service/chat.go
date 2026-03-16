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
	"time"
)

type ChatService struct{}

var Chat = new(ChatService)

func (s *ChatService) Chat(ctx context.Context, req dto.ChatReq) (*vo.ChatResp, error) {

	// 创建新会话
	if req.ConversationID == "" {
		//需要先根据req.Message通过llm创建一个标题
		title, err := llm.GenerateNewTitle(req.Message)
		if err != nil {
			return nil, err
		}
		//创建会话
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

	unlock, err := redis.AcquireConversationLock(ctx, req.ConversationID, 10*time.Second)
	if err != nil {
		return nil, err
	}
	defer unlock()

	//获取上下文
	msgs, summary, err := redis.AddAndGetContext(ctx, req.ConversationID, redis.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		return nil, err
	}

	// 保存用户消息到 mysql（放在 AddAndGetContext 后，避免 Redis 冷启动时回填 + 追加导致重复）
	err = Save2Mysql(ctx, req.ConversationID, "user", req.Message)
	if err != nil {
		return nil, err
	}

	//构建 LLM 上下文 []Message：system role是系统，summary role是摘要，user role是用户，assistant role是助手
	llmContext := []llm.Message{{Role: "system", Content: "You are a helpful assistant."}}
	if summary != "" {
		llmContext = append(llmContext, llm.Message{Role: "system", Content: "[对话摘要]: " + summary})
	}
	for _, msg := range msgs {
		llmContext = append(llmContext, llm.Message{Role: msg.Role, Content: msg.Content})
	}
	//因为redis已经拼到msg后面了，这里不好处理了
	// llmContext = append(llmContext, llm.Message{Role: "user", Content: req.Message})

	//调用 LLM 生成回复
	reply, err := llm.GenerateFromMessages(llmContext)
	if err != nil {
		return nil, err
	}

	//保存到mysql
	err = Save2Mysql(ctx, req.ConversationID, "assistant", reply)
	if err != nil {
		return nil, err
	}

	//保存到redis
	err = redis.SaveAssistantReply(ctx, req.ConversationID, redis.Message{
		Role:    "assistant",
		Content: reply,
	})
	if err != nil {
		return nil, err
	}

	return &vo.ChatResp{
		ConversationID: req.ConversationID,
		Reply:          reply,
	}, nil
}
