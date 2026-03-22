package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"

	"github.com/gin-gonic/gin"
)

func ConversationList(c *gin.Context) {
	var req dto.ConversationListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Conversation.List(c.Request.Context(), userID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func ConversationMessageList(c *gin.Context) {
	var uri dto.ConversationURI
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	var req dto.ConversationMessageListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Conversation.ListMessages(c.Request.Context(), userID, uri.ID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

func ConversationDelete(c *gin.Context) {
	var uri dto.ConversationURI
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	userID := uint64(c.GetUint("user_id"))
	resp, err := service.Conversation.Delete(c.Request.Context(), userID, uri.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}
