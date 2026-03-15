package router

import (
	"ai-knowledge-go/internal/api/handler"
	"ai-knowledge-go/internal/middleware"
	"ai-knowledge-go/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由
func UserRoutes(rg *gin.RouterGroup) {
	userGroup := rg.Group("/user")
	{
		// 公开接口（不需要登录）
		userGroup.POST("/register", handler.UserRegister)
		userGroup.POST("/login", handler.UserLogin)

		// 需要认证的接口
		auth := userGroup.Group("")
		auth.Use(middleware.JWTAuth()) // JWT 认证中间件
		auth.GET("/ping", func(c *gin.Context) {
			userID := c.GetUint("user_id")
			response.Success(c, gin.H{"user_id": userID})
		})
		// {
		// 	auth.GET("/profile", userHandler.GetUserProfile)
		// 	auth.PUT("/profile", userHandler.UpdateUserProfile)
		// 	auth.POST("/logout", userHandler.UserLogout)
		// }
	}
}
