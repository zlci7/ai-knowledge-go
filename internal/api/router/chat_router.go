package router

import (
	"ai-knowledge-go/internal/api/handler"
	"ai-knowledge-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func ChatRoutes(rg *gin.RouterGroup) {
	chatGroup := rg.Group("/chat")
	chatGroup.Use(middleware.JWTAuth())
	{
		chatGroup.POST("/send", handler.ChatSend)
		chatGroup.POST("/stream", handler.ChatStream)
	}
}
