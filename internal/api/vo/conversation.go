package vo

import (
	"ai-knowledge-go/internal/model"
	"time"
)

type ConversationItem struct {
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ConversationListResp struct {
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
	Items    []ConversationItem `json:"items"`
}

type ConversationMessageItem struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Seq       int64     `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
}

type ConversationMessageListResp struct {
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Total    int64                     `json:"total"`
	Items    []ConversationMessageItem `json:"items"`
}

type ConversationDeleteResp struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
}

func NewConversationItem(conv model.Conversation) ConversationItem {
	return ConversationItem{
		ConversationID: conv.ConvID,
		Title:          conv.Title,
		Summary:        conv.Summary,
		CreatedAt:      conv.CreatedAt,
		UpdatedAt:      conv.UpdatedAt,
	}
}

func NewConversationMessageItem(msg model.Message) ConversationMessageItem {
	return ConversationMessageItem{
		Role:      string(msg.Role),
		Content:   msg.Content,
		Seq:       msg.Seq,
		CreatedAt: msg.CreatedAt,
	}
}
