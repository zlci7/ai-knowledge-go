package model

import "time"

type DocumentStatus string

const (
	DocumentStatusUploading  DocumentStatus = "uploading"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusReady      DocumentStatus = "ready"
	DocumentStatusFailed     DocumentStatus = "failed"
	DocumentStatusDeleting   DocumentStatus = "deleting"
	DocumentStatusDeleted    DocumentStatus = "deleted"
)

type Document struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	KnowledgeBaseID uint64         `gorm:"not null;index:idx_kb" json:"knowledge_base_id"`
	FileName        string         `gorm:"size:255;not null" json:"file_name"`
	FileType        string         `gorm:"size:32" json:"file_type"`
	MimeType        string         `gorm:"size:128" json:"mime_type"`
	FileSize        int64          `gorm:"not null" json:"file_size"`
	FilePath        string         `gorm:"size:500;not null" json:"file_path"`
	DocType         string         `gorm:"size:50;index:idx_doc_type" json:"doc_type"`
	Project         string         `gorm:"size:100;index:idx_project" json:"project"`
	Tags            string         `gorm:"type:json" json:"tags"`
	ChunkCount      int            `gorm:"not null;default:0" json:"chunk_count"`
	Status          DocumentStatus `gorm:"size:20;not null;default:uploading;index:idx_status" json:"status"`
	ErrorMessage    string         `gorm:"type:text" json:"error_message"`
	UploadedBy      uint64         `gorm:"not null;index:idx_uploaded_by" json:"uploaded_by"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (Document) TableName() string {
	return "documents"
}
