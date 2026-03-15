package model

import "time"

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

type Message struct {
	ID         uint64      `gorm:"primaryKey;autoIncrement" json:"-"`
	MsgID      string      `gorm:"index;not null" json:"msg_id"`
	ConvID     string      `gorm:"index;not null" json:"conv_id"`
	Role       MessageRole `gorm:"size:20;not null" json:"role"`
	Content    string      `gorm:"type:text;not null" json:"content"`
	Seq        int64       `gorm:"index;not null" json:"seq"`
	TokenCount int64       `json:"token_count,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}
