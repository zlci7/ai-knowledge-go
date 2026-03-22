package service

import (
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/pkg/idgen"
	"ai-knowledge-go/internal/repository/mysql"
	"ai-knowledge-go/internal/repository/redis"
	"context"
	"fmt"
	"time"
)

// Save2Mysql 将一条会话消息持久化到 MySQL，并使用 Redis 自增序列保证顺序。
func Save2Mysql(ctx context.Context, convID string, role model.MessageRole, content string) error {

	// 通过 Redis 生成会话内递增序号，避免依赖 MySQL 自增带来的跨实例顺序问题。
	seqKey := fmt.Sprintf("stm:%s:seq", convID)
	// 获取本条消息的序号。
	seq, err := redis.Rdb.Incr(ctx, seqKey).Result()
	if err != nil {
		return err
	}

	err = mysql.Message.Create(ctx, &model.Message{
		MsgID:     idgen.GenStringID(),
		ConvID:    convID,
		Role:      role,
		Content:   content,
		Seq:       seq,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}
