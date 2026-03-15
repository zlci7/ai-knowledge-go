package handler

import (
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"

	"github.com/gin-gonic/gin"
)

// handleServiceError 统一处理 Service 层错误（避免重复代码）
func handleServiceError(c *gin.Context, err error) {
	if codeErr, ok := err.(*xerr.CodeError); ok {
		// 传递自定义错误消息（如果有的话）
		response.Error(c, codeErr.GetErrCode(), codeErr.GetErrMsg())
	} else {
		response.Error(c, xerr.SERVER_COMMON_ERROR, "")
	}
}
