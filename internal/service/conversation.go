package service

import (
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/repository/mysql"
	"context"
	"errors"

	"gorm.io/gorm"
)

type ConversationService struct{}

// Conversation 提供会话列表、消息查询与删除能力。
var Conversation = new(ConversationService)

const (
	defaultConversationPageSize = 20
	maxConversationPageSize     = 100
	defaultMessagePageSize      = 50
	maxMessagePageSize          = 200
)

func (s *ConversationService) List(ctx context.Context, userID uint64, req dto.ConversationListReq) (*vo.ConversationListResp, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultConversationPageSize
	}
	if pageSize > maxConversationPageSize {
		pageSize = maxConversationPageSize
	}

	convs, total, err := mysql.Conversation.ListByUserIDPaged(ctx, userID, page, pageSize)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.CONVERSATION_LIST_ERROR)
	}

	items := make([]vo.ConversationItem, 0, len(convs))
	for _, conv := range convs {
		items = append(items, vo.NewConversationItem(conv))
	}

	return &vo.ConversationListResp{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Items:    items,
	}, nil
}

func (s *ConversationService) ListMessages(ctx context.Context, userID uint64, convID string, req dto.ConversationMessageListReq) (*vo.ConversationMessageListResp, error) {
	_, err := mysql.Conversation.GetByConvIDAndUserID(ctx, convID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.NewErrCode(xerr.CONVERSATION_NOT_FOUND)
		}
		return nil, xerr.NewErrCode(xerr.CONVERSATION_MESSAGE_LIST_ERROR)
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultMessagePageSize
	}
	if pageSize > maxMessagePageSize {
		pageSize = maxMessagePageSize
	}

	msgs, total, err := mysql.Message.ListByConversationIDPaged(ctx, convID, page, pageSize)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.CONVERSATION_MESSAGE_LIST_ERROR)
	}

	items := make([]vo.ConversationMessageItem, 0, len(msgs))
	for _, msg := range msgs {
		items = append(items, vo.NewConversationMessageItem(msg))
	}

	return &vo.ConversationMessageListResp{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Items:    items,
	}, nil
}

func (s *ConversationService) Delete(ctx context.Context, userID uint64, convID string) (*vo.ConversationDeleteResp, error) {
	rows, err := mysql.Conversation.SoftDeleteByConvID(ctx, convID, userID)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.CONVERSATION_DELETE_ERROR)
	}
	if rows == 0 {
		return nil, xerr.NewErrCode(xerr.CONVERSATION_NOT_FOUND)
	}

	return &vo.ConversationDeleteResp{
		ConversationID: convID,
		Status:         "deleted",
	}, nil
}
