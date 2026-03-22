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

func (r *ConversationDao) GetByConvIDAndUserID(ctx context.Context, convID string, userID uint64) (*model.Conversation, error) {
	var conv model.Conversation
	err := DB.WithContext(ctx).
		Where("conv_id = ? AND user_id = ? AND is_deleted = false", convID, userID).
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

func (r *ConversationDao) ListByUserIDPaged(ctx context.Context, userID uint64, page, pageSize int) ([]model.Conversation, int64, error) {
	db := DB.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("user_id = ? AND is_deleted = false", userID)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var convs []model.Conversation
	err := db.Order("updated_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&convs).Error
	if err != nil {
		return nil, 0, err
	}
	return convs, total, nil
}

func (r *ConversationDao) SoftDeleteByConvID(ctx context.Context, convID string, userID uint64) (int64, error) {
	tx := DB.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conv_id = ? AND user_id = ? AND is_deleted = false", convID, userID).
		Update("is_deleted", true)
	return tx.RowsAffected, tx.Error
}

func (r *ConversationDao) UpdateTitle(ctx context.Context, convId string, title string) error {
	return DB.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("conv_id = ?", convId).
		Update("title", title).Error
}
