package redis

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/pkg/idgen"
	"context"
	"encoding/json"
	"fmt"
	goredis "github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

type MemoryVectorOperation string

const (
	MemoryVectorOpUpsert MemoryVectorOperation = "upsert"
	MemoryVectorOpDelete MemoryVectorOperation = "delete"
)

type MemoryVectorJob struct {
	JobID     string                `json:"job_id"`
	Op        MemoryVectorOperation `json:"op"`
	MemoryID  uint64                `json:"memory_id"`
	UserID    uint64                `json:"user_id"`
	Content   string                `json:"content,omitempty"`
	Category  string                `json:"category,omitempty"`
	Attempt   int                   `json:"attempt"`
	NextRunAt int64                 `json:"next_run_at"`
	CreatedAt int64                 `json:"created_at"`
	LastError string                `json:"last_error,omitempty"`
}

type memoryQueueKeys struct {
	DelayZSet string
	ReadyList string
	DLQList   string
}

func EnqueueMemoryVectorJob(ctx context.Context, job MemoryVectorJob, delay time.Duration) error {
	if job.JobID == "" {
		job.JobID = genMemoryVectorJobID()
	}
	now := time.Now()
	if job.CreatedAt == 0 {
		job.CreatedAt = now.Unix()
	}
	job.NextRunAt = now.Add(delay).Unix()

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal memory vector job: %w", err)
	}

	keys := getMemoryQueueKeys()
	return Rdb.ZAdd(ctx, keys.DelayZSet, goredis.Z{
		Score:  float64(job.NextRunAt),
		Member: string(payload),
	}).Err()
}

func MoveDueMemoryVectorJobs(ctx context.Context, limit int64) (int, error) {
	keys := getMemoryQueueKeys()
	now := strconv.FormatInt(time.Now().Unix(), 10)
	items, err := Rdb.ZRangeByScore(ctx, keys.DelayZSet, &goredis.ZRangeBy{
		Min:    "-inf",
		Max:    now,
		Offset: 0,
		Count:  limit,
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	moved := 0
	for _, item := range items {
		removed, err := Rdb.ZRem(ctx, keys.DelayZSet, item).Result()
		if err != nil {
			return moved, err
		}
		if removed == 0 {
			continue
		}
		if err := Rdb.LPush(ctx, keys.ReadyList, item).Err(); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

func PopReadyMemoryVectorJob(ctx context.Context, timeout time.Duration) (*MemoryVectorJob, error) {
	keys := getMemoryQueueKeys()
	values, err := Rdb.BRPop(ctx, timeout, keys.ReadyList).Result()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}
		return nil, err
	}
	if len(values) < 2 {
		return nil, nil
	}

	var job MemoryVectorJob
	if err := json.Unmarshal([]byte(values[1]), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func PushMemoryVectorDLQ(ctx context.Context, job MemoryVectorJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal dlq job: %w", err)
	}
	keys := getMemoryQueueKeys()
	return Rdb.LPush(ctx, keys.DLQList, payload).Err()
}

func getMemoryQueueKeys() memoryQueueKeys {
	prefix := config.AppConfig.Memory.Async.QueueKeyPrefix
	return memoryQueueKeys{
		DelayZSet: prefix + ":delay_zset",
		ReadyList: prefix + ":ready_list",
		DLQList:   prefix + ":dlq_list",
	}
}

func genMemoryVectorJobID() string {
	jobID := idgen.GenStringID()
	if jobID != "" {
		return jobID
	}
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}
