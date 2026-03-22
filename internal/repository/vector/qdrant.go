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
	"strconv"
	"strings"
	"time"
)

const (
	knowledgeDenseVectorName  = "dense"
	knowledgeSparseVectorName = "bm25"
)

var Qdrant = new(QdrantRepository)
var KnowledgeQdrant = new(KnowledgeQdrantRepository)

// InitQdrant 初始化全局 Qdrant 客户端并确保集合存在
func InitQdrant(ctx context.Context) error {
	baseURL := fmt.Sprintf("http://%s:%d", config.AppConfig.Qdrant.Host, config.AppConfig.Qdrant.Port)
	trimmedBaseURL := strings.TrimRight(baseURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}

	Qdrant.baseURL = trimmedBaseURL
	Qdrant.collection = config.AppConfig.Qdrant.MemoryCollection
	Qdrant.client = client

	KnowledgeQdrant.baseURL = trimmedBaseURL
	KnowledgeQdrant.collection = config.AppConfig.Qdrant.KnowledgeCollection
	KnowledgeQdrant.client = client
	KnowledgeQdrant.denseName = ""
	KnowledgeQdrant.sparseName = knowledgeSparseVectorName
	KnowledgeQdrant.sparseOn = false

	if err := Qdrant.EnsureCollection(ctx); err != nil {
		return err
	}
	return KnowledgeQdrant.EnsureCollection(ctx)
}

// EnsureCollection 确保指定的 Qdrant 向量集合已创建，不存在则创建
func (r *QdrantRepository) EnsureCollection(ctx context.Context) error {
	return ensureDenseCollection(ctx, r.client, r.baseURL, r.collection)
}

// EnsureCollection 确保知识文档集合存在，并探测是否支持 sparse 向量。
func (r *KnowledgeQdrantRepository) EnsureCollection(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if r.collection == "" {
		return fmt.Errorf("qdrant collection is empty")
	}

	status, body, err := getCollectionMeta(ctx, r.client, r.baseURL, r.collection)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		r.detectKnowledgeCapabilities(body)
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("qdrant get collection status: %d body=%s", status, string(body))
	}

	createURL := fmt.Sprintf("%s/collections/%s", r.baseURL, r.collection)
	reqBody := map[string]any{
		"vectors": map[string]any{
			knowledgeDenseVectorName: map[string]any{
				"size":     config.AppConfig.Dashscope.EmbeddingDimension,
				"distance": "Cosine",
			},
		},
		"sparse_vectors": map[string]any{
			knowledgeSparseVectorName: map[string]any{},
		},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal qdrant create knowledge collection body: %w", err)
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
		respBody, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("create qdrant collection status=%d body=%s", createResp.StatusCode, string(respBody))
	}

	r.denseName = knowledgeDenseVectorName
	r.sparseName = knowledgeSparseVectorName
	r.sparseOn = true
	return nil
}

func ensureDenseCollection(ctx context.Context, client *http.Client, baseURL, collection string) error {
	if client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if collection == "" {
		return fmt.Errorf("qdrant collection is empty")
	}

	status, body, err := getCollectionMeta(ctx, client, baseURL, collection)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("qdrant get collection status: %d body=%s", status, string(body))
	}

	createURL := fmt.Sprintf("%s/collections/%s", baseURL, collection)
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

	createResp, err := client.Do(createReq)
	if err != nil {
		return fmt.Errorf("create qdrant collection: %w", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode < 200 || createResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("create qdrant collection status=%d body=%s", createResp.StatusCode, string(respBody))
	}
	return nil
}

func getCollectionMeta(ctx context.Context, client *http.Client, baseURL, collection string) (int, []byte, error) {
	getURL := fmt.Sprintf("%s/collections/%s", baseURL, collection)
	req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create qdrant get collection request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("get qdrant collection: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}

func (r *KnowledgeQdrantRepository) detectKnowledgeCapabilities(respBody []byte) {
	r.denseName = ""
	r.sparseName = knowledgeSparseVectorName
	r.sparseOn = false

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return
	}
	result, _ := parsed["result"].(map[string]any)
	configObj, _ := result["config"].(map[string]any)
	params, _ := configObj["params"].(map[string]any)

	if vectorsRaw, ok := params["vectors"]; ok {
		switch vectors := vectorsRaw.(type) {
		case map[string]any:
			if _, hasSize := vectors["size"]; hasSize {
				r.denseName = ""
			} else if _, hasDense := vectors[knowledgeDenseVectorName]; hasDense {
				r.denseName = knowledgeDenseVectorName
			} else {
				for key := range vectors {
					r.denseName = key
					break
				}
			}
		}
	}

	if sparseRaw, ok := params["sparse_vectors"]; ok {
		if sparse, ok := sparseRaw.(map[string]any); ok && len(sparse) > 0 {
			if _, hasBM25 := sparse[knowledgeSparseVectorName]; hasBM25 {
				r.sparseName = knowledgeSparseVectorName
			} else {
				for key := range sparse {
					r.sparseName = key
					break
				}
			}
			r.sparseOn = true
		}
	}
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

// UpsertKnowledgeChunks 批量写入文档分块向量
func (r *KnowledgeQdrantRepository) UpsertKnowledgeChunks(ctx context.Context, chunks []KnowledgeChunkPoint) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if len(chunks) == 0 {
		return nil
	}

	points := make([]map[string]any, 0, len(chunks))
	for _, chunk := range chunks {
		point := map[string]any{
			"id": chunk.ID,
			"payload": map[string]any{
				"kb_id":       chunk.KBID,
				"doc_id":      chunk.DocID,
				"doc_name":    chunk.DocName,
				"doc_type":    chunk.DocType,
				"project":     chunk.Project,
				"tags":        chunk.Tags,
				"chunk_index": chunk.ChunkIndex,
				"content":     chunk.Content,
				"created_at":  time.Now().Format(time.RFC3339),
			},
		}

		if r.denseName == "" {
			point["vector"] = chunk.Vector
		} else {
			vectors := map[string]any{
				r.denseName: chunk.Vector,
			}
			if r.sparseOn && len(chunk.Sparse.Indices) > 0 && len(chunk.Sparse.Indices) == len(chunk.Sparse.Values) {
				vectors[r.sparseName] = map[string]any{
					"indices": chunk.Sparse.Indices,
					"values":  chunk.Sparse.Values,
				}
			}
			point["vector"] = vectors
		}

		points = append(points, point)
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", r.baseURL, r.collection)
	reqBody := map[string]any{"points": points}
	return doJSONWithClient(ctx, r.client, "PUT", url, reqBody)
}

// DeleteKnowledgeByDocument 删除指定文档的所有分块向量
func (r *KnowledgeQdrantRepository) DeleteKnowledgeByDocument(ctx context.Context, kbID, docID uint64) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", r.baseURL, r.collection)
	reqBody := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "kb_id",
					"match": map[string]any{
						"value": kbID,
					},
				},
				{
					"key": "doc_id",
					"match": map[string]any{
						"value": docID,
					},
				},
			},
		},
	}
	return doJSONWithClient(ctx, r.client, "POST", url, reqBody)
}

// DeleteKnowledgePoints 按 point IDs 删除，用于上传失败回滚
func (r *KnowledgeQdrantRepository) DeleteKnowledgePoints(ctx context.Context, pointIDs []uint64) error {
	if r.client == nil {
		return fmt.Errorf("qdrant client is not initialized")
	}
	if len(pointIDs) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", r.baseURL, r.collection)
	reqBody := map[string]any{
		"points": pointIDs,
	}
	return doJSONWithClient(ctx, r.client, "POST", url, reqBody)
}

// SearchKnowledgeChunks 按知识库执行 dense 检索。
func (r *KnowledgeQdrantRepository) SearchKnowledgeChunks(ctx context.Context, kbID uint64, queryVector []float32, limit int, scoreThreshold float64) ([]KnowledgeSearchResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}

	vectorBody := any(queryVector)
	if r.denseName != "" {
		vectorBody = map[string]any{
			"name":   r.denseName,
			"vector": queryVector,
		}
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", r.baseURL, r.collection)
	reqBody := map[string]any{
		"vector":          vectorBody,
		"limit":           limit,
		"score_threshold": scoreThreshold,
		"with_payload":    true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "kb_id",
					"match": map[string]any{
						"value": kbID,
					},
				},
			},
		},
	}

	respData, err := doJSONRequest(ctx, r.client, "POST", url, reqBody)
	if err != nil {
		return nil, err
	}
	return parseKnowledgeSearchResults(respData, "dense")
}

// SearchKnowledgeChunksSparse 按知识库执行 sparse(BM25) 检索。
func (r *KnowledgeQdrantRepository) SearchKnowledgeChunksSparse(ctx context.Context, kbID uint64, sparse SparseVector, limit int) ([]KnowledgeSearchResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}
	if !r.sparseOn || len(sparse.Indices) == 0 || len(sparse.Indices) != len(sparse.Values) {
		return nil, nil
	}

	url := fmt.Sprintf("%s/collections/%s/points/query", r.baseURL, r.collection)
	reqBody := map[string]any{
		"query": map[string]any{
			"indices": sparse.Indices,
			"values":  sparse.Values,
		},
		"using":        r.sparseName,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "kb_id",
					"match": map[string]any{
						"value": kbID,
					},
				},
			},
		},
	}

	respData, err := doJSONRequest(ctx, r.client, "POST", url, reqBody)
	if err != nil {
		return nil, err
	}
	return parseKnowledgeSearchResults(respData, "sparse")
}

func parseKnowledgeSearchResults(respData []byte, source string) ([]KnowledgeSearchResult, error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(respData, &envelope); err != nil {
		return nil, fmt.Errorf("parse qdrant search response: %w", err)
	}

	type rawItem struct {
		ID      any            `json:"id"`
		Score   float64        `json:"score"`
		Payload map[string]any `json:"payload"`
	}

	items := make([]rawItem, 0)
	if len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, &items); err != nil {
			var wrapped struct {
				Points []rawItem `json:"points"`
			}
			if err2 := json.Unmarshal(envelope.Result, &wrapped); err2 != nil {
				return nil, fmt.Errorf("parse qdrant search items: %w", err)
			}
			items = wrapped.Points
		}
	}

	results := make([]KnowledgeSearchResult, 0, len(items))
	for _, item := range items {
		result := KnowledgeSearchResult{
			Score:  item.Score,
			Source: source,
		}
		result.PointID = anyToUint64(item.ID)

		if item.Payload != nil {
			if content, ok := item.Payload["content"].(string); ok {
				result.Content = content
			}
			if docName, ok := item.Payload["doc_name"].(string); ok {
				result.DocName = docName
			}
			result.DocID = anyToUint64(item.Payload["doc_id"])
		}
		if result.DocID == 0 {
			result.DocID = result.PointID
		}
		if result.PointID == 0 {
			result.PointID = result.DocID
		}
		results = append(results, result)
	}
	return results, nil
}

func anyToUint64(v any) uint64 {
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case float32:
		return uint64(n)
	case int:
		return uint64(n)
	case int64:
		return uint64(n)
	case uint64:
		return n
	case json.Number:
		if parsed, err := n.Int64(); err == nil {
			return uint64(parsed)
		}
	case string:
		if parsed, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64); err == nil {
			return parsed
		}
	}
	return 0
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
	return doJSONWithClient(ctx, r.client, method, url, body)
}

func doJSONRequest(ctx context.Context, client *http.Client, method, url string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal qdrant search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("create qdrant request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read qdrant response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant status=%d body=%s", resp.StatusCode, string(respData))
	}
	return respData, nil
}

func doJSONWithClient(ctx context.Context, client *http.Client, method, url string, body any) error {
	_, err := doJSONRequest(ctx, client, method, url, body)
	return err
}
