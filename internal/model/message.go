package model

import "time"

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

type Message struct {
	ID             uint64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64      `gorm:"index;not null" json:"conversation_id"`
	Role           MessageRole `gorm:"size:20;not null" json:"role"`
	Content        string      `gorm:"type:text;not null" json:"content"`
	TokenCount     int         `json:"token_count,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}
