package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

func KnowledgeDocumentUpload(c *gin.Context) {
	var uri dto.KnowledgeBaseURI
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, xerr.DOCUMENT_PARAM_ERROR, "file is required")
		return
	}

	tagsRaw := strings.TrimSpace(c.PostForm("tags"))
	tags := make([]string, 0)
	if tagsRaw != "" {
		if err := json.Unmarshal([]byte(tagsRaw), &tags); err != nil {
			response.Error(c, xerr.DOCUMENT_PARAM_ERROR, "tags must be a JSON array")
			return
		}
	}

	req := dto.DocumentUploadReq{
		DocType: strings.TrimSpace(c.PostForm("doc_type")),
		Project: strings.TrimSpace(c.PostForm("project")),
		Tags:    tags,
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Knowledge.Upload(c.Request.Context(), userID, uri.ID, req, file)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func KnowledgeDocumentList(c *gin.Context) {
	var uri dto.KnowledgeBaseURI
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	var req dto.DocumentListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Knowledge.List(c.Request.Context(), userID, uri.ID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func KnowledgeDocumentDelete(c *gin.Context) {
	var uri dto.KnowledgeDocumentURI
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Knowledge.Delete(c.Request.Context(), userID, uri.ID, uri.DocID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}
