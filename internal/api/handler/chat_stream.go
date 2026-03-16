package handler

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/service"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ChatStream(c *gin.Context) {
	var req dto.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, xerr.REUQEST_PARAM_ERROR, "")
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, xerr.SERVER_COMMON_ERROR, "streaming is not supported")
		return
	}

	eventCh, err := service.Chat.ChatStream(c.Request.Context(), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	for event := range eventCh {
		payload, err := json.Marshal(event)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", event.Type)
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		flusher.Flush()
	}
}
