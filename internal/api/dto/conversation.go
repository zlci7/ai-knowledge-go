package dto

type ConversationURI struct {
	ID string `uri:"id" binding:"required"`
}

type ConversationListReq struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

type ConversationMessageListReq struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}
