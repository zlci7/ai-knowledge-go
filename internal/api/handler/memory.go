package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"

	"github.com/gin-gonic/gin"
)

func MemoryCreate(c *gin.Context) {
	var req dto.MemoryCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Memory.Create(c.Request.Context(), userID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func MemoryList(c *gin.Context) {
	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Memory.List(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func MemoryDelete(c *gin.Context) {
	var req dto.MemoryDeleteReq
	if err := c.ShouldBindUri(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	if err := service.Memory.Delete(c.Request.Context(), userID, req.ID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
