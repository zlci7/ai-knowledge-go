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

func (d *MessageDao) ListByConversationIDPaged(ctx context.Context, convID string, page, pageSize int) ([]model.Message, int64, error) {
	db := DB.WithContext(ctx).
		Model(&model.Message{}).
		Where("conv_id = ?", convID)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var msgs []model.Message
	err := db.Order("created_at ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&msgs).Error
	if err != nil {
		return nil, 0, err
	}

	return msgs, total, nil
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
