package service

import (
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/repository/vector"
	"context"
	"fmt"
	"strings"
)

const (
	// longTermMemoryTopK 控制长期记忆向量召回条数。
	longTermMemoryTopK          = 3
	// longTermMemoryScoreThr 控制召回相似度下限，避免注入弱相关记忆。
	longTermMemoryScoreThr      = 0.75
	// longTermMemoryBudgetTimeout 为长期记忆检索分配的最大耗时预算（毫秒）。
	longTermMemoryBudgetTimeout = 600 // milliseconds
)

// retrieveLongTermMemories 将用户问题改写后执行向量检索，并返回可用记忆文本。
func retrieveLongTermMemories(ctx context.Context, userID uint64, userMessage string) ([]string, error) {
	rewritten, err := RewriteForRetrieval(ctx, userMessage)
	if err != nil {
		return nil, err
	}

	embedding, err := llm.GenerateEmbedding(ctx, rewritten)
	if err != nil {
		return nil, err
	}

	hits, err := vector.Qdrant.SearchMemoryPoints(ctx, userID, embedding, longTermMemoryTopK, longTermMemoryScoreThr)
	if err != nil {
		return nil, err
	}

	memories := make([]string, 0, len(hits))
	for _, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		memories = append(memories, content)
	}
	return memories, nil
}

// formatLongTermMemorySystemPrompt 将召回记忆格式化为系统提示词，约束模型使用范围。
func formatLongTermMemorySystemPrompt(memories []string) string {
	if len(memories) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[长期记忆-仅在相关时使用]\n")
	b.WriteString("请遵守以下规则：\n")
	b.WriteString("1. 仅当记忆与当前问题相关时使用；不相关请忽略。\n")
	b.WriteString("2. 不得编造未提供的记忆内容。\n")
	b.WriteString("3. 如果记忆与用户当前输入冲突，以当前输入为准。\n")
	b.WriteString("可参考的长期记忆：\n")
	for i, m := range memories {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, m))
	}
	return strings.TrimSpace(b.String())
}
