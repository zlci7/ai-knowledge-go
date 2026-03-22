package service

import (
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/repository/vector"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// ragTopK 控制融合后的最终召回条数。
	ragTopK = 4
	// ragDenseTopK 控制 dense 通道候选数。
	ragDenseTopK = 4
	// ragSparseTopK 控制 sparse 通道候选数。
	ragSparseTopK = 4
	// ragScoreThreshold 控制召回相似度下限，避免注入弱相关内容。
	ragScoreThreshold = 0.65
	// ragTimeout 为 RAG 检索预算，超过即降级跳过。
	ragTimeout = 600
	// ragRRFK 为 RRF 融合常数。
	ragRRFK = 60.0
	// ragChunkMaxChars 控制单条知识片段最大字符数，防止提示词膨胀。
	ragChunkMaxChars = 280
	// ragPromptMaxChars 控制 RAG 提示词总字符数上限。
	ragPromptMaxChars = 1600
)

// retrieveKnowledgeChunks 进行知识库向量召回，并对结果做清洗、去重、截断。
func retrieveKnowledgeChunks(ctx context.Context, kbID uint64, userMessage string) ([]vector.KnowledgeSearchResult, error) {
	query := strings.TrimSpace(userMessage)
	if query == "" {
		return nil, nil
	}

	rewritten, err := RewriteForRetrieval(ctx, query)
	if err != nil {
		return nil, err
	}
	rewritten = strings.TrimSpace(rewritten)
	if rewritten == "" {
		return nil, nil
	}

	embedding, err := llm.GenerateEmbedding(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	denseHits, denseErr := vector.KnowledgeQdrant.SearchKnowledgeChunks(ctx, kbID, embedding, ragDenseTopK, ragScoreThreshold)
	sparseHits, sparseErr := vector.KnowledgeQdrant.SearchKnowledgeChunksSparse(ctx, kbID, encodeBM25Sparse(rewritten), ragSparseTopK)
	if denseErr != nil && sparseErr != nil {
		return nil, errors.Join(denseErr, sparseErr)
	}

	merged := mergeKnowledgeHitsRRF(denseHits, sparseHits, ragRRFK)
	if len(merged) > ragTopK {
		merged = merged[:ragTopK]
	}

	results := make([]vector.KnowledgeSearchResult, 0, len(merged))
	for _, hit := range merged {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		hit.Content = truncateRunes(content, ragChunkMaxChars)
		results = append(results, hit)
	}
	return results, nil
}

// formatKnowledgeRAGSystemPrompt 将知识库召回内容格式化为受约束的 system prompt。
func formatKnowledgeRAGSystemPrompt(chunks []vector.KnowledgeSearchResult) string {
	if len(chunks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[知识库召回-仅在相关时使用]\n")
	b.WriteString("请遵守以下规则：\n")
	b.WriteString("1. 仅在与当前问题相关时使用，不相关请忽略。\n")
	b.WriteString("2. 不得编造召回片段中不存在的信息。\n")
	b.WriteString("3. 若与用户当前输入冲突，以用户当前输入为准。\n")
	b.WriteString("可参考的知识片段：\n")

	for i, chunk := range chunks {
		line := fmt.Sprintf("%d. %s", i+1, chunk.Content)
		if strings.TrimSpace(chunk.DocName) != "" {
			line = fmt.Sprintf("%d. [%s] %s", i+1, strings.TrimSpace(chunk.DocName), chunk.Content)
		}
		line += "\n"

		if b.Len()+len(line) > ragPromptMaxChars {
			break
		}
		b.WriteString(line)
	}

	return strings.TrimSpace(b.String())
}

func truncateRunes(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func mergeKnowledgeHitsRRF(denseHits, sparseHits []vector.KnowledgeSearchResult, k float64) []vector.KnowledgeSearchResult {
	type agg struct {
		item  vector.KnowledgeSearchResult
		score float64
	}

	merged := make(map[uint64]*agg, len(denseHits)+len(sparseHits))
	accumulate := func(hits []vector.KnowledgeSearchResult, source string) {
		for rank, hit := range hits {
			pointID := hit.PointID
			if pointID == 0 {
				continue
			}
			rrf := 1.0 / (k + float64(rank+1))
			existing, ok := merged[pointID]
			if !ok {
				copied := hit
				copied.Source = source
				merged[pointID] = &agg{item: copied, score: rrf}
				continue
			}
			existing.score += rrf
			if existing.item.Content == "" && hit.Content != "" {
				existing.item.Content = hit.Content
			}
			if existing.item.DocName == "" && hit.DocName != "" {
				existing.item.DocName = hit.DocName
			}
			if existing.item.Source != source {
				existing.item.Source = "hybrid"
			}
		}
	}

	accumulate(denseHits, "dense")
	accumulate(sparseHits, "sparse")

	results := make([]vector.KnowledgeSearchResult, 0, len(merged))
	for _, item := range merged {
		item.item.RRFScore = item.score
		item.item.Score = item.score
		results = append(results, item.item)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].RRFScore == results[j].RRFScore {
			return results[i].PointID < results[j].PointID
		}
		return results[i].RRFScore > results[j].RRFScore
	})
	return results
}
