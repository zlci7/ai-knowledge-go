package dto

type KnowledgeBaseURI struct {
	ID uint64 `uri:"id" binding:"required,gt=0"`
}

type KnowledgeDocumentURI struct {
	ID    uint64 `uri:"id" binding:"required,gt=0"`
	DocID uint64 `uri:"doc_id" binding:"required,gt=0"`
}

type DocumentListReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	DocType  string `form:"doc_type"`
	Project  string `form:"project"`
	Tag      string `form:"tag"`
}

type DocumentUploadReq struct {
	DocType string   `json:"doc_type"`
	Project string   `json:"project"`
	Tags    []string `json:"tags"`
}
