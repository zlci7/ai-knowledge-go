package vo

import "ai-knowledge-go/internal/model"

// UserRegisterResp 注册响应
type UserRegisterResp struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
}

// UserLoginResp 登录响应
type UserLoginResp struct {
	Token    string   `json:"token"`
	UserInfo UserInfo `json:"user_info"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID uint `json:"user_id"`
	// Phone    string `json:"phone"`
	Username string `json:"username"`
}

// NewUserInfo 从 Model 构造 VO
func NewUserInfo(user *model.User) UserInfo {
	return UserInfo{
		UserID:   uint(user.ID),
		Username: user.Username,
	}
}
