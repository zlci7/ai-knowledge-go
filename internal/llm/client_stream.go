package llm

import (
	"ai-knowledge-go/config"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GenerateFromMessagesStream streams LLM deltas to out.
// out is always closed when this function returns.
func GenerateFromMessagesStream(ctx context.Context, messages []Message, out chan<- StreamChunk) {
	defer close(out)

	requestBody := RequestBody{
		Model:    config.AppConfig.Dashscope.LLMModel,
		Messages: messages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		emitChunk(ctx, out, StreamChunk{Err: fmt.Errorf("marshal request: %w", err)})
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		emitChunk(ctx, out, StreamChunk{Err: fmt.Errorf("create request: %w", err)})
		return
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.Dashscope.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// Client canceled: no need to surface as server error.
		if ctx.Err() != nil {
			return
		}
		emitChunk(ctx, out, StreamChunk{Err: fmt.Errorf("send request: %w", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		emitChunk(ctx, out, StreamChunk{
			Err: fmt.Errorf("llm http status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
		})
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			emitChunk(ctx, out, StreamChunk{Done: true})
			return
		}

		var chunkResp streamResponseBody
		if err := json.Unmarshal([]byte(payload), &chunkResp); err != nil {
			// Skip malformed partial line and continue streaming.
			continue
		}

		if chunkResp.Error != nil {
			emitChunk(ctx, out, StreamChunk{Err: fmt.Errorf("llm error: %s", chunkResp.Error.Message)})
			return
		}
		if len(chunkResp.Choices) == 0 {
			continue
		}

		choice := chunkResp.Choices[0]
		delta := choice.Delta.Content
		if delta == "" {
			delta = choice.Message.Content
		}
		if delta != "" {
			if !emitChunk(ctx, out, StreamChunk{Delta: delta}) {
				return
			}
		}

		if choice.FinishReason != "" {
			emitChunk(ctx, out, StreamChunk{Done: true})
			return
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return
		}
		emitChunk(ctx, out, StreamChunk{Err: fmt.Errorf("read stream: %w", err)})
		return
	}

	if ctx.Err() == nil {
		emitChunk(ctx, out, StreamChunk{Done: true})
	}
}

func emitChunk(ctx context.Context, out chan<- StreamChunk, chunk StreamChunk) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}
