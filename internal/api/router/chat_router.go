package router

import (
	"ai-knowledge-go/internal/api/handler"

	"github.com/gin-gonic/gin"
)

func ChatRoutes(rg *gin.RouterGroup) {
	chatGroup := rg.Group("/chat")
	{
		chatGroup.POST("/send", handler.ChatSend)
		chatGroup.POST("/stream", handler.ChatStream)
	}
}
