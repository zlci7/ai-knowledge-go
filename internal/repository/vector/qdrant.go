package vector

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var Qdrant = new(QdrantRepository)

type QdrantRepository struct {
	baseURL    string
	collection string
	client     *http.Client
}

func InitQdrant(ctx context.Context) error {
	baseURL := fmt.Sprintf("http://%s:%d", config.AppConfig.Qdrant.Host, config.AppConfig.Qdrant.Port)
	Qdrant.baseURL = strings.TrimRight(baseURL, "/")
	Qdrant.collection = config.AppConfig.Qdrant.Collection
	Qdrant.client = &http.Client{Timeout: 10 * time.Second}
	return Qdrant.EnsureCollection(ctx)
}

func (r *QdrantRepository) EnsureCollection(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	getURL := fmt.Sprintf("%s/collections/%s", r.baseURL, r.collection)
	req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return fmt.Errorf("create qdrant get collection request: %w", err)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("get qdrant collection: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("qdrant get collection status: %d", resp.StatusCode)
	}

	createURL := fmt.Sprintf("%s/collections/%s", r.baseURL, r.collection)
	reqBody := map[string]any{
		"vectors": map[string]any{
			"size":     config.AppConfig.Dashscope.EmbeddingDimension,
			"distance": "Cosine",
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal qdrant create collection body: %w", err)
	}

	createReq, err := http.NewRequestWithContext(ctx, "PUT", createURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create qdrant create collection request: %w", err)
	}
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := r.client.Do(createReq)
	if err != nil {
		return fmt.Errorf("create qdrant collection: %w", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
		body, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("create qdrant collection status=%d body=%s", createResp.StatusCode, string(body))
	}
	return nil
}

func (r *QdrantRepository) UpsertMemoryPoint(ctx context.Context, memory *model.LongTermMemory, vector []float32) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", r.baseURL, r.collection)
	reqBody := map[string]any{
		"points": []map[string]any{
			{
				"id":     memory.ID,
				"vector": vector,
				"payload": map[string]any{
					"memory_id":  memory.ID,
					"user_id":    memory.UserID,
					"category":   string(memory.Category),
					"content":    memory.Content,
					"created_at": memory.CreatedAt.Format(time.RFC3339),
				},
			},
		},
	}
	return r.doJSON(ctx, "PUT", url, reqBody)
}

func (r *QdrantRepository) DeleteMemoryPoint(ctx context.Context, memoryID uint64) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", r.baseURL, r.collection)
	reqBody := map[string]any{
		"points": []uint64{memoryID},
	}
	return r.doJSON(ctx, "POST", url, reqBody)
}

func (r *QdrantRepository) doJSON(ctx context.Context, method, url string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal qdrant request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create qdrant request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant status=%d body=%s", resp.StatusCode, string(respData))
	}
	return nil
}
