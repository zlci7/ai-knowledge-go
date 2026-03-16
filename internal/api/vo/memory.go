package vo

import (
	"ai-knowledge-go/internal/model"
	"time"
)

type MemoryItem struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewMemoryItem(memory *model.LongTermMemory) MemoryItem {
	return MemoryItem{
		ID:        memory.ID,
		UserID:    memory.UserID,
		Content:   memory.Content,
		Category:  string(memory.Category),
		Source:    string(memory.Source),
		CreatedAt: memory.CreatedAt,
		UpdatedAt: memory.UpdatedAt,
	}
}
