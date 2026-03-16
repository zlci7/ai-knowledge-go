package dto

type MemoryCreateReq struct {
	Content  string `json:"content" binding:"required"`
	Category string `json:"category" binding:"required,oneof=preference fact rule"`
}

type MemoryDeleteReq struct {
	ID uint64 `uri:"id" binding:"required,gt=0"`
}
