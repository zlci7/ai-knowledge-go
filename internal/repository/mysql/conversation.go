package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"

	"gorm.io/gorm"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conv *model.Conversation) error {
	return r.db.WithContext(ctx).Create(conv).Error
}

func (r *ConversationRepository) GetByID(ctx context.Context, id uint64) (*model.Conversation, error) {
	var conv model.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND is_deleted = false", id).
		First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ConversationRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.Conversation, error) {
	var convs []model.Conversation
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_deleted = false", userID).
		Order("updated_at DESC").
		Find(&convs).Error
	return convs, err
}

func (r *ConversationRepository) SoftDelete(ctx context.Context, id, userID uint64) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_deleted", true).Error
}

func (r *ConversationRepository) UpdateTitle(ctx context.Context, id uint64, title string) error {
	return r.db.WithContext(ctx).
		Model(&model.Conversation{}).
		Where("id = ?", id).
		Update("title", title).Error
}
