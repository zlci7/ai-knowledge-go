package llm

import (
	"ai-knowledge-go/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := embeddingRequestBody{
		Model: config.AppConfig.Dashscope.EmbeddingModel,
		Input: text,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	baseURL := strings.TrimRight(config.AppConfig.Dashscope.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	url := baseURL + "/embeddings"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.Dashscope.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var result embeddingResponseBody
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("embedding error: %s", result.Error.Message)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding response empty")
	}

	embedding := make([]float32, 0, len(result.Data[0].Embedding))
	for _, v := range result.Data[0].Embedding {
		embedding = append(embedding, float32(v))
	}
	return embedding, nil
}
