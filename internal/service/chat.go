package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
	"ai-knowledge-go/internal/llm"
)

type ChatService struct{}

var Chat = new(ChatService)

func (s *ChatService) Chat(req dto.ChatReq) (*vo.ChatResp, error) {
	// 发送单轮对话请求到 LLM，返回模型的文字回复。
	reply, err := llm.GenerateFromSinglePrompt(req.Message)
	if err != nil {
		return nil, err
	}
	return &vo.ChatResp{
		ConversationID: req.ConversationID,
		Reply:          reply,
	}, nil
}
