package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"
)

var Memory = new(MemoryDao)

type MemoryDao struct{}

func (d *MemoryDao) Create(ctx context.Context, memory *model.LongTermMemory) error {
	return DB.WithContext(ctx).Create(memory).Error
}

func (d *MemoryDao) ListByUserID(ctx context.Context, userID uint64) ([]model.LongTermMemory, error) {
	var memories []model.LongTermMemory
	err := DB.WithContext(ctx).
		Where("user_id = ? AND is_deleted = false", userID).
		Order("updated_at DESC").
		Find(&memories).Error
	return memories, err
}

func (d *MemoryDao) SoftDeleteByID(ctx context.Context, id, userID uint64) (int64, error) {
	tx := DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ? AND user_id = ? AND is_deleted = false", id, userID).
		Updates(map[string]any{
			"is_deleted":    true,
			"vector_status": model.MemoryVectorStatusDeleting,
		})
	return tx.RowsAffected, tx.Error
}

func (d *MemoryDao) UpdateVectorRetry(ctx context.Context, id uint64, retryCount int, lastError string) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_retry_count": retryCount,
			"vector_last_error":  lastError,
		}).Error
}

func (d *MemoryDao) MarkVectorSynced(ctx context.Context, id uint64, vectorID string) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_id":          vectorID,
			"vector_status":      model.MemoryVectorStatusSynced,
			"vector_last_error":  "",
			"vector_retry_count": 0,
		}).Error
}

func (d *MemoryDao) MarkVectorDeleted(ctx context.Context, id uint64) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_status":      model.MemoryVectorStatusDeleted,
			"vector_last_error":  "",
			"vector_retry_count": 0,
		}).Error
}

func (d *MemoryDao) MarkVectorFailed(ctx context.Context, id uint64, retryCount int, lastError string) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_status":      model.MemoryVectorStatusFailed,
			"vector_retry_count": retryCount,
			"vector_last_error":  lastError,
		}).Error
}

func (d *MemoryDao) MarkVectorPending(ctx context.Context, id uint64, lastError string) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_status":      model.MemoryVectorStatusPending,
			"vector_last_error":  lastError,
			"vector_retry_count": 0,
		}).Error
}

func (d *MemoryDao) MarkVectorDeleting(ctx context.Context, id uint64, lastError string) error {
	return DB.WithContext(ctx).
		Model(&model.LongTermMemory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"vector_status":      model.MemoryVectorStatusDeleting,
			"vector_last_error":  lastError,
			"vector_retry_count": 0,
		}).Error
}

func (d *MemoryDao) ListPendingForReplay(ctx context.Context, limit int) ([]model.LongTermMemory, error) {
	var memories []model.LongTermMemory
	err := DB.WithContext(ctx).
		Where("vector_status IN ?", []model.MemoryVectorStatus{
			model.MemoryVectorStatusPending,
			model.MemoryVectorStatusDeleting,
		}).
		Order("updated_at ASC").
		Limit(limit).
		Find(&memories).Error
	return memories, err
}
