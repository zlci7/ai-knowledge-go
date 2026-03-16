package router

import (
	"ai-knowledge-go/internal/api/handler"
	"ai-knowledge-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

func MemoryRoutes(rg *gin.RouterGroup) {
	memoryGroup := rg.Group("/memories")
	memoryGroup.Use(middleware.JWTAuth())
	{
		memoryGroup.POST("", handler.MemoryCreate)
		memoryGroup.GET("", handler.MemoryList)
		memoryGroup.DELETE("/:id", handler.MemoryDelete)
	}
}
