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

func Save2Mysql(ctx context.Context, convID string, role model.MessageRole, content string) error {

	//改进：通过redis生成seqId，避免mysql自增id的缺点
	//获取唯一redis key
	seqKey := fmt.Sprintf("stm:%s:seq", convID)
	//获取序列号
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
