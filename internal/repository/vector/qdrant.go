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

// InitQdrant 初始化全局 Qdrant 客户端并确保集合存在
func InitQdrant(ctx context.Context) error {
	baseURL := fmt.Sprintf("http://%s:%d", config.AppConfig.Qdrant.Host, config.AppConfig.Qdrant.Port)
	Qdrant.baseURL = strings.TrimRight(baseURL, "/")
	Qdrant.collection = config.AppConfig.Qdrant.Collection
	Qdrant.client = &http.Client{Timeout: 10 * time.Second}
	return Qdrant.EnsureCollection(ctx)
}

// EnsureCollection 确保指定的 Qdrant 向量集合已创建，不存在则创建
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

// UpsertMemoryPoint 在 Qdrant 中插入或更新一条长时记忆向量及其负载信息
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

// DeleteMemoryPoint 根据记忆 ID 从 Qdrant 中删除对应向量点
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

// SearchMemoryPoints 使用查询向量为指定用户检索相似的长时记忆点
func (r *QdrantRepository) SearchMemoryPoints(ctx context.Context, userID uint64, queryVector []float32, limit int, scoreThreshold float64) ([]MemorySearchResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", r.baseURL, r.collection)
	reqBody := map[string]any{
		"vector":          queryVector,
		"limit":           limit,
		"score_threshold": scoreThreshold,
		"with_payload":    true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "user_id",
					"match": map[string]any{
						"value": userID,
					},
				},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal qdrant search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("create qdrant search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read qdrant search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant search status=%d body=%s", resp.StatusCode, string(respData))
	}

	var parsed struct {
		Result []struct {
			ID      any            `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respData, &parsed); err != nil {
		return nil, fmt.Errorf("parse qdrant search response: %w", err)
	}

	results := make([]MemorySearchResult, 0, len(parsed.Result))
	for _, item := range parsed.Result {
		result := MemorySearchResult{
			Score: item.Score,
		}
		if content, ok := item.Payload["content"].(string); ok {
			result.Content = content
		}
		if category, ok := item.Payload["category"].(string); ok {
			result.Category = category
		}
		switch v := item.Payload["memory_id"].(type) {
		case float64:
			result.MemoryID = uint64(v)
		case int64:
			result.MemoryID = uint64(v)
		}
		if result.MemoryID == 0 {
			switch idv := item.ID.(type) {
			case float64:
				result.MemoryID = uint64(idv)
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// doJSON 将任意请求体编码为 JSON 并发送 HTTP 请求，校验 Qdrant 响应状态
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
