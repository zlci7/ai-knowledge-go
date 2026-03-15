package redis

import (
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/repository/mysql"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AddAndGetContext 添加用户消息并获取上下文
// convID 会话ID
// userMsg 用户消息
// 返回值：
// 1. 消息列表
// 2. 摘要
// 3. 错误
func AddAndGetContext(ctx context.Context, convID string, userMsg Message) ([]Message, string, error) {
	msgKey := fmt.Sprintf("stm:%s:messages", convID)
	sumKey := fmt.Sprintf("stm:%s:summary", convID)
	windowSize := 10
	k := 4
	ttl := 24 * 3600 * time.Second // 24小时过期

	// 1. 尝试获取当前 Redis 中的消息
	val, _ := Rdb.LRange(ctx, msgKey, 0, -1).Result()

	// 2. 兜底逻辑：如果 Redis 为空，从 MySQL 加载历史
	if len(val) == 0 {
		mysqlMsgs, err := mysql.Message.GetRecentMessages(ctx, convID, 10)
		if err != nil {
			return nil, "", err
		}
		for _, m := range mysqlMsgs {
			data, _ := json.Marshal(m)
			Rdb.RPush(ctx, msgKey, data)
		}
		// 重新拉取一次
		val, _ = Rdb.LRange(ctx, msgKey, 0, -1).Result()
	}

	// 3. 将当前用户消息加入队列
	userData, _ := json.Marshal(userMsg)
	Rdb.RPush(ctx, msgKey, userData)
	Rdb.Expire(ctx, msgKey, ttl)
	val = append(val, string(userData)) // 手动更新本地副本，减少一次请求

	// 4. 获取当前摘要
	currentSummary, _ := Rdb.Get(ctx, sumKey).Result()

	// 5. 窗口检查：如果超过 windowSize，触发压缩
	if len(val) > windowSize {
		// 取出最旧的 K 条进行总结
		toSummarizeRaw := val[:k]
		var toSummarizeMsgs []llm.Message
		for _, s := range toSummarizeRaw {
			var m llm.Message
			json.Unmarshal([]byte(s), &m)
			toSummarizeMsgs = append(toSummarizeMsgs, m)
		}
		// 调用 LLM 生成新摘要
		newSummary, err := llm.GenerateNewSummary(currentSummary, toSummarizeMsgs)
		if err == nil {
			// 更新摘要并移除旧消息
			Rdb.Set(ctx, sumKey, newSummary, ttl)
			Rdb.LTrim(ctx, msgKey, int64(k), -1)

			// 更新返回给 Service 的变量
			currentSummary = newSummary
			val = val[k:]
		}
	}

	// 6. 解析最终的消息数组返回
	var finalMsgs []Message
	for _, s := range val {
		var m Message
		json.Unmarshal([]byte(s), &m)
		finalMsgs = append(finalMsgs, m)
	}

	return finalMsgs, currentSummary, nil
}

// SaveAssistantReply 保存 AI 回复并续期
func SaveAssistantReply(ctx context.Context, convID string, assistantMsg Message) error {
	msgKey := fmt.Sprintf("stm:%s:messages", convID)
	ttl := 24 * 3600 * time.Second

	data, err := json.Marshal(assistantMsg)
	if err != nil {
		return err
	}

	// 插入 AI 回复并续期
	pipe := Rdb.Pipeline()
	pipe.RPush(ctx, msgKey, data)
	pipe.Expire(ctx, msgKey, ttl)

	_, err = pipe.Exec(ctx)
	return err
}
