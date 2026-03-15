package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"

	"github.com/gin-gonic/gin"
)

func ChatSend(c *gin.Context) {
	var req dto.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	resp, err := service.Chat.Chat(c.Request.Context(), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}
