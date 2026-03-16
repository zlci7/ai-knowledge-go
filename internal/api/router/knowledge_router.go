package router

import (
	"ai-knowledge-go/internal/api/handler"
	"ai-knowledge-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func KnowledgeRoutes(rg *gin.RouterGroup) {
	knowledgeGroup := rg.Group("/knowledge-bases")
	knowledgeGroup.Use(middleware.JWTAuth())
	{
		knowledgeGroup.POST("/:id/documents", handler.KnowledgeDocumentUpload)
		knowledgeGroup.GET("/:id/documents", handler.KnowledgeDocumentList)
		knowledgeGroup.DELETE("/:id/documents/:doc_id", handler.KnowledgeDocumentDelete)
	}
}
