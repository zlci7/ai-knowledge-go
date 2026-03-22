package dto

// ChatReq 聊天请求
type ChatReq struct {
	ConversationID string `json:"conversation_id"` // 空字符串表示新建会话
	Message        string `json:"message" binding:"required"`
}
