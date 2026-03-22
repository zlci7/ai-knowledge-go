package router

import (
	"ai-knowledge-go/internal/api/handler"
	"ai-knowledge-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func ConversationRoutes(rg *gin.RouterGroup) {
	conversationGroup := rg.Group("/conversations")
	conversationGroup.Use(middleware.JWTAuth())
	{
		conversationGroup.GET("", handler.ConversationList)
		conversationGroup.GET("/:id/messages", handler.ConversationMessageList)
		conversationGroup.DELETE("/:id", handler.ConversationDelete)
	}
}
