package dto

// UserRegisterReq 注册请求
type UserRegisterReq struct {
	// Phone    string `json:"phone" binding:"required,len=11,numeric"`
	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// UserLoginReq 登录请求
type UserLoginReq struct {
	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=6,max=20"`
}
