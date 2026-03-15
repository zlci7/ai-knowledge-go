// handler/user_handler.go
package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"

	"github.com/gin-gonic/gin"
)

// UserRegister 用户注册
func UserRegister(c *gin.Context) {
	// 1. 绑定请求参数
	var req dto.UserRegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	// 2. 调用 Service
	resp, err := service.User.Register(req)
	if err != nil {
		// 统一错误处理
		handleServiceError(c, err)
		return
	}

	// 3. 返回成功
	response.Success(c, resp)
}

// UserLogin 用户登录
func UserLogin(c *gin.Context) {
	// 1. 绑定请求参数
	var req dto.UserLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	// 2. 调用 Service
	resp, err := service.User.Login(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	// 3. 返回成功
	response.Success(c, resp)
}
