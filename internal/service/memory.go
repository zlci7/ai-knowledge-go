package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/repository/mysql"
	redisRepo "ai-knowledge-go/internal/repository/redis"
	"context"
	"fmt"
)

type MemoryService struct{}

// Memory 提供长期记忆的增删查能力。
var Memory = new(MemoryService)

// Create 创建一条长期记忆，并投递向量化入库任务到异步队列。
func (s *MemoryService) Create(ctx context.Context, userID uint64, req dto.MemoryCreateReq) (*vo.MemoryItem, error) {
	memory := &model.LongTermMemory{
		UserID:       userID,
		Content:      req.Content,
		Category:     model.MemoryCategory(req.Category),
		VectorStatus: model.MemoryVectorStatusPending,
		Source:       model.MemorySourceManual,
	}

	if err := mysql.Memory.Create(ctx, memory); err != nil {
		return nil, xerr.NewErrCode(xerr.MEMORY_CREATE_ERROR)
	}
	job := redisRepo.MemoryVectorJob{
		Op:       redisRepo.MemoryVectorOpUpsert,
		MemoryID: memory.ID,
		UserID:   memory.UserID,
		Content:  memory.Content,
		Category: string(memory.Category),
		Attempt:  0,
	}
	if err := redisRepo.EnqueueMemoryVectorJob(ctx, job, 0); err != nil {
		_ = mysql.Memory.MarkVectorFailed(ctx, memory.ID, 0, fmt.Sprintf("enqueue upsert job failed: %v", err))
		return nil, xerr.NewErrCode(xerr.MEMORY_CREATE_ERROR)
	}

	resp := vo.NewMemoryItem(memory)
	return &resp, nil
}

// List 查询用户的长期记忆列表，并转换为接口返回对象。
func (s *MemoryService) List(ctx context.Context, userID uint64) ([]vo.MemoryItem, error) {
	memories, err := mysql.Memory.ListByUserID(ctx, userID)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.MEMORY_LIST_ERROR)
	}

	resp := make([]vo.MemoryItem, 0, len(memories))
	for i := range memories {
		resp = append(resp, vo.NewMemoryItem(&memories[i]))
	}
	return resp, nil
}

// Delete 软删除指定记忆，并异步删除向量库中的对应向量点。
func (s *MemoryService) Delete(ctx context.Context, userID, id uint64) error {
	rows, err := mysql.Memory.SoftDeleteByID(ctx, id, userID)
	if err != nil {
		return xerr.NewErrCode(xerr.MEMORY_DELETE_ERROR)
	}
	if rows == 0 {
		return xerr.NewErrCode(xerr.MEMORY_NOT_FOUND)
	}
	job := redisRepo.MemoryVectorJob{
		Op:       redisRepo.MemoryVectorOpDelete,
		MemoryID: id,
		UserID:   userID,
		Attempt:  0,
	}
	if err := redisRepo.EnqueueMemoryVectorJob(ctx, job, 0); err != nil {
		_ = mysql.Memory.MarkVectorFailed(ctx, id, 0, fmt.Sprintf("enqueue delete job failed: %v", err))
		return xerr.NewErrCode(xerr.MEMORY_DELETE_ERROR)
	}
	return nil
}
