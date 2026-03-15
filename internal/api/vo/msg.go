package vo

// ChatResp 聊天响应
type ChatResp struct {
	ConversationID uint64 `json:"conversation_id"`
	Reply          string `json:"reply"`
}
