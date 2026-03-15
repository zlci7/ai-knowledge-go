package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"
)

var Message = new(MessageDao)

type MessageDao struct{}

func (d *MessageDao) Create(ctx context.Context, msg *model.Message) error {
	return DB.WithContext(ctx).Create(msg).Error
}

func (d *MessageDao) ListByConversationID(ctx context.Context, convID string) ([]model.Message, error) {
	var msgs []model.Message
	err := DB.WithContext(ctx).
		Where("conv_id = ?", convID).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// GetRecentMessages returns the most recent N messages for a conversation, ordered oldest-first.
func (d *MessageDao) GetRecentMessages(ctx context.Context, convID string, limit int) ([]model.Message, error) {
	var msgs []model.Message
	err := DB.WithContext(ctx).
		Where("conv_id = ?", convID).
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
