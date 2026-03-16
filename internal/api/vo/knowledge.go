package vo

import (
	"ai-knowledge-go/internal/model"
	"encoding/json"
	"time"
)

type DocumentUploadResp struct {
	DocID        uint64 `json:"doc_id"`
	Status       string `json:"status"`
	ChunkCount   int    `json:"chunk_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	ProcessingMS int64  `json:"processing_ms"`
}

type DocumentDeleteResp struct {
	DocID  uint64 `json:"doc_id"`
	Status string `json:"status"`
}

type DocumentItem struct {
	ID         uint64    `json:"id"`
	FileName   string    `json:"file_name"`
	FileType   string    `json:"file_type"`
	FileSize   int64     `json:"file_size"`
	DocType    string    `json:"doc_type"`
	Project    string    `json:"project"`
	Tags       []string  `json:"tags"`
	ChunkCount int       `json:"chunk_count"`
	Status     string    `json:"status"`
	ErrorMsg   string    `json:"error_message,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type DocumentListResp struct {
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
	Items    []DocumentItem `json:"items"`
}

func NewDocumentItem(doc model.Document) DocumentItem {
	return DocumentItem{
		ID:         doc.ID,
		FileName:   doc.FileName,
		FileType:   doc.FileType,
		FileSize:   doc.FileSize,
		DocType:    doc.DocType,
		Project:    doc.Project,
		Tags:       parseTags(doc.Tags),
		ChunkCount: doc.ChunkCount,
		Status:     string(doc.Status),
		ErrorMsg:   doc.ErrorMessage,
		CreatedAt:  doc.CreatedAt,
	}
}

func parseTags(tagsJSON string) []string {
	if tagsJSON == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return []string{}
	}
	return tags
}
