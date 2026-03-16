package service

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/llm"
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/repository/mysql"
	redisRepo "ai-knowledge-go/internal/repository/redis"
	"ai-knowledge-go/internal/repository/vector"
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

type MemoryAsyncService struct {
	cancel func()
	wg     sync.WaitGroup
}

var MemoryAsync = new(MemoryAsyncService)

func (s *MemoryAsyncService) Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	if err := s.enqueuePendingJobs(ctx); err != nil {
		return err
	}

	s.wg.Add(2)
	go s.schedulerLoop(ctx)
	go s.consumerLoop(ctx)
	return nil
}

func (s *MemoryAsyncService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *MemoryAsyncService) schedulerLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := redisRepo.MoveDueMemoryVectorJobs(ctx, 100)
			if err != nil {
				log.Printf("memory async scheduler move jobs failed: %v", err)
			}
		}
	}
}

func (s *MemoryAsyncService) consumerLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := redisRepo.PopReadyMemoryVectorJob(ctx, 1*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("memory async consume pop failed: %v", err)
			continue
		}
		if job == nil {
			continue
		}

		if err := s.handleJob(ctx, *job); err != nil {
			log.Printf("memory async handle job failed id=%s op=%s memory_id=%d err=%v", job.JobID, job.Op, job.MemoryID, err)
			s.retryOrDLQ(ctx, *job, err)
		}
	}
}

func (s *MemoryAsyncService) handleJob(ctx context.Context, job redisRepo.MemoryVectorJob) error {
	switch job.Op {
	case redisRepo.MemoryVectorOpUpsert:
		embedding, err := llm.GenerateEmbedding(ctx, job.Content)
		if err != nil {
			return fmt.Errorf("generate embedding: %w", err)
		}
		mem := &model.LongTermMemory{
			ID:        job.MemoryID,
			UserID:    job.UserID,
			Content:   job.Content,
			Category:  model.MemoryCategory(job.Category),
			CreatedAt: time.Unix(job.CreatedAt, 0),
		}
		if err := vector.Qdrant.UpsertMemoryPoint(ctx, mem, embedding); err != nil {
			return fmt.Errorf("upsert qdrant point: %w", err)
		}
		vectorID := strconv.FormatUint(job.MemoryID, 10)
		if err := mysql.Memory.MarkVectorSynced(ctx, job.MemoryID, vectorID); err != nil {
			return fmt.Errorf("mark vector synced: %w", err)
		}
		return nil
	case redisRepo.MemoryVectorOpDelete:
		if err := vector.Qdrant.DeleteMemoryPoint(ctx, job.MemoryID); err != nil {
			return fmt.Errorf("delete qdrant point: %w", err)
		}
		if err := mysql.Memory.MarkVectorDeleted(ctx, job.MemoryID); err != nil {
			return fmt.Errorf("mark vector deleted: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown memory vector op: %s", job.Op)
	}
}

func (s *MemoryAsyncService) retryOrDLQ(ctx context.Context, job redisRepo.MemoryVectorJob, cause error) {
	retryMax := config.AppConfig.Memory.Async.RetryMax
	nextAttempt := job.Attempt + 1
	errMsg := truncateErr(cause.Error(), 1000)

	_ = mysql.Memory.UpdateVectorRetry(ctx, job.MemoryID, nextAttempt, errMsg)

	job.Attempt = nextAttempt
	job.LastError = errMsg

	if nextAttempt < retryMax {
		delay := backoffDuration(nextAttempt)
		if err := redisRepo.EnqueueMemoryVectorJob(ctx, job, delay); err != nil {
			log.Printf("memory async re-enqueue failed memory_id=%d err=%v", job.MemoryID, err)
			_ = redisRepo.PushMemoryVectorDLQ(ctx, job)
			_ = mysql.Memory.MarkVectorFailed(ctx, job.MemoryID, nextAttempt, truncateErr(fmt.Sprintf("re-enqueue failed: %v", err), 1000))
		}
		return
	}

	if err := redisRepo.PushMemoryVectorDLQ(ctx, job); err != nil {
		log.Printf("memory async push dlq failed memory_id=%d err=%v", job.MemoryID, err)
	}
	if err := mysql.Memory.MarkVectorFailed(ctx, job.MemoryID, nextAttempt, errMsg); err != nil {
		log.Printf("memory async mark failed status failed memory_id=%d err=%v", job.MemoryID, err)
	}
}

func (s *MemoryAsyncService) enqueuePendingJobs(ctx context.Context) error {
	memories, err := mysql.Memory.ListPendingForReplay(ctx, 500)
	if err != nil {
		return fmt.Errorf("list pending memories for replay: %w", err)
	}
	for _, memory := range memories {
		job := redisRepo.MemoryVectorJob{
			MemoryID:  memory.ID,
			UserID:    memory.UserID,
			Content:   memory.Content,
			Category:  string(memory.Category),
			Attempt:   memory.VectorRetryCount,
			CreatedAt: memory.CreatedAt.Unix(),
		}
		switch memory.VectorStatus {
		case model.MemoryVectorStatusDeleting:
			job.Op = redisRepo.MemoryVectorOpDelete
		default:
			job.Op = redisRepo.MemoryVectorOpUpsert
		}
		if err := redisRepo.EnqueueMemoryVectorJob(ctx, job, time.Second); err != nil {
			log.Printf("memory async enqueue pending job failed memory_id=%d err=%v", memory.ID, err)
		}
	}
	return nil
}

func backoffDuration(attempt int) time.Duration {
	base := config.AppConfig.Memory.Async.RetryBaseSeconds
	if base <= 0 {
		base = 2
	}
	// attempt=1 => base seconds, attempt=2 => 2*base, attempt=3 => 4*base
	multiplier := 1 << (attempt - 1)
	return time.Duration(base*multiplier) * time.Second
}

func truncateErr(errMsg string, maxLen int) string {
	if len(errMsg) <= maxLen {
		return errMsg
	}
	return errMsg[:maxLen]
}
