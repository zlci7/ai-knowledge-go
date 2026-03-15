package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

func (r *MessageRepository) ListByConversationID(ctx context.Context, convID uint64) ([]model.Message, error) {
	var msgs []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// GetRecentMessages returns the most recent N messages for a conversation, ordered oldest-first.
func (r *MessageRepository) GetRecentMessages(ctx context.Context, convID uint64, limit int) ([]model.Message, error) {
	var msgs []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Order("created_at DESC").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}
