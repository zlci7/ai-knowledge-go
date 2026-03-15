package response

import (
	"ai-knowledge-go/internal/pkg/xerr"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, Response{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

// Error 失败响应
// - 如果 errMsg 为空，自动从 xerr 获取标准消息
// - 如果 errMsg 不为空，使用自定义消息
func Error(c *gin.Context, errCode uint32, errMsg string) {
	// 如果没传消息，就从 xerr 查标准消息
	if errMsg == "" {
		errMsg = xerr.MapErrMsg(errCode)
	}

	c.JSON(200, Response{
		Code:    int(errCode),
		Message: errMsg,
		Data:    nil,
	})
}
