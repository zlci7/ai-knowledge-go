package service

import (
	"ai-knowledge-go/internal/llm"
	"context"
	"fmt"
	"strings"
)

func RewriteForRetrieval(ctx context.Context, userMessage string) (string, error) {
	prompt := `你是查询重写器。请将用户问题改写成一个“适合向量检索的单句查询”，要求：
1. 消除指代歧义，补全主语和目标；
2. 保留原意，不要引入新事实；
3. 只输出改写结果，不要解释。`

	rewritten, err := llm.GenerateFromSinglePromptWithContext(ctx, prompt+"\n用户问题："+userMessage)
	if err != nil {
		return "", err
	}

	rewritten = strings.TrimSpace(rewritten)
	if rewritten == "" {
		return "", fmt.Errorf("rewritten query is empty")
	}
	return rewritten, nil
}
