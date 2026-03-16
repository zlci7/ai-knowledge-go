package mysql

import (
	"context"

	"ai-knowledge-go/internal/model"
)

var Document = new(DocumentDao)

type DocumentDao struct{}

type DocumentListQuery struct {
	Page     int
	PageSize int
	Status   string
	DocType  string
	Project  string
	Tag      string
}

func (d *DocumentDao) Create(ctx context.Context, doc *model.Document) error {
	return DB.WithContext(ctx).Create(doc).Error
}

func (d *DocumentDao) GetByID(ctx context.Context, kbID, docID uint64) (*model.Document, error) {
	var doc model.Document
	err := DB.WithContext(ctx).
		Where("id = ? AND knowledge_base_id = ?", docID, kbID).
		First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (d *DocumentDao) UpdateStatus(ctx context.Context, docID uint64, status model.DocumentStatus, chunkCount int, errMsg string) error {
	return DB.WithContext(ctx).
		Model(&model.Document{}).
		Where("id = ?", docID).
		Updates(map[string]any{
			"status":        status,
			"chunk_count":   chunkCount,
			"error_message": errMsg,
		}).Error
}

func (d *DocumentDao) UpdateFilePath(ctx context.Context, docID uint64, filePath string) error {
	return DB.WithContext(ctx).
		Model(&model.Document{}).
		Where("id = ?", docID).
		Update("file_path", filePath).Error
}

func (d *DocumentDao) MarkDeleting(ctx context.Context, kbID, docID uint64) (int64, error) {
	tx := DB.WithContext(ctx).
		Model(&model.Document{}).
		Where("id = ? AND knowledge_base_id = ? AND status <> ?", docID, kbID, model.DocumentStatusDeleted).
		Updates(map[string]any{
			"status":        model.DocumentStatusDeleting,
			"error_message": "",
		})
	return tx.RowsAffected, tx.Error
}

func (d *DocumentDao) MarkDeleted(ctx context.Context, docID uint64) error {
	return DB.WithContext(ctx).
		Model(&model.Document{}).
		Where("id = ?", docID).
		Updates(map[string]any{
			"status":        model.DocumentStatusDeleted,
			"error_message": "",
		}).Error
}

func (d *DocumentDao) ListByKB(ctx context.Context, kbID uint64, query DocumentListQuery) ([]model.Document, int64, error) {
	db := DB.WithContext(ctx).Model(&model.Document{}).
		Where("knowledge_base_id = ? AND status <> ?", kbID, model.DocumentStatusDeleted)

	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.DocType != "" {
		db = db.Where("doc_type = ?", query.DocType)
	}
	if query.Project != "" {
		db = db.Where("project = ?", query.Project)
	}
	if query.Tag != "" {
		db = db.Where("JSON_CONTAINS(tags, JSON_ARRAY(?))", query.Tag)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	var docs []model.Document
	err := db.Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&docs).Error
	return docs, total, err
}
