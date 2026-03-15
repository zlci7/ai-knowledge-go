package dto

// ChatReq 聊天请求
type ChatReq struct {
	ConversationID uint64 `json:"conversation_id"` // 0 表示新建会话
	Message        string `json:"message" binding:"required"`
}
