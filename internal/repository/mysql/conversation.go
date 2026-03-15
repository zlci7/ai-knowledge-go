package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"
)

var Conversation = new(ConversationDao)

type ConversationDao struct{}

func (r *ConversationDao) Create(ctx context.Context, conv *model.Conversation) error {
	return DB.WithContext(ctx).Create(conv).Error
}

func (r *ConversationDao) GetByID(ctx context.Context, convId string) (*model.Conversation, error) {
	var conv model.Conversation
	err := DB.WithContext(ctx).
		Where("conv_id = ? AND is_deleted = false", convId).
		First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ConversationDao) ListByUserID(ctx context.Context, userID uint64) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := DB.WithContext(ctx).
		Where("user_id = ? AND is_deleted = false", userID).
		Order("updated_at DESC").
		Find(&convs).Error
	return convs, err
}

func (r *ConversationDao) SoftDelete(ctx context.Context, id, userID uint64) error {
	return DB.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conv_id = ? AND user_id = ?", id, userID).
		Update("is_deleted", true).Error
}

func (r *ConversationDao) UpdateTitle(ctx context.Context, convId string, title string) error {
	return DB.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conv_id = ?", convId).
		Update("title", title).Error
}
