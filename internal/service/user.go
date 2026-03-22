package service

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/api/dto"
	"ai-knowledge-go/internal/api/vo"
	"ai-knowledge-go/internal/model"
	"ai-knowledge-go/internal/pkg/encrypt"
	"ai-knowledge-go/internal/pkg/jwtx"
	"ai-knowledge-go/internal/pkg/xerr"
	"ai-knowledge-go/internal/repository/mysql"
	"time"
)

type UserService struct{}

// User 提供用户注册与登录等身份相关服务。
var User = new(UserService)

// Register 完成用户名校验、密码加密与用户创建，并返回注册结果。
func (s *UserService) Register(req dto.UserRegisterReq) (*vo.UserRegisterResp, error) {
	// 1. 业务校验
	exist, err := mysql.User.ExistOrNotByUsername(req.Username)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.DB_ERROR)
	}
	if exist {
		return nil, xerr.NewErrCode(xerr.USER_ALREADY_EXISTS)
	}

	// 2. 密码加密（业务逻辑）
	password, err := encrypt.EncryptPassword(req.Password)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.USER_ENCRYPT_ERROR)
	}

	// 3. DTO -> Model 转换（在 Service 层完成）
	userModel := &model.User{
		Username: req.Username,
		Password: password,
	}

	// 4. 调用 DAO 保存（传 Model）
	if err := mysql.User.CreateUser(userModel); err != nil {
		return nil, xerr.NewErrCode(xerr.USER_CREATE_ERROR)
	}

	// 5. Model -> VO 转换（在 Service 层完成）
	resp := &vo.UserRegisterResp{
		Username: userModel.Username,
		UserID:   uint(userModel.ID),
	}

	// 6. 返回 VO（不是 Model）
	return resp, nil
}

// Login 校验账户密码并签发访问令牌，返回登录响应对象。
func (s *UserService) Login(req dto.UserLoginReq) (*vo.UserLoginResp, error) {
	// 1. 查找用户（DAO 返回 Model）
	user, err := mysql.User.GetUserByUsername(req.Username)
	if err != nil {
		return nil, xerr.NewErrCode(xerr.USER_NOT_FOUND)
	}

	// 2. 密码校验
	if !encrypt.ValidatePassword(req.Password, user.Password) {
		return nil, xerr.NewErrCode(xerr.USER_PASSWORD_ERROR)
	}

	// 3. 生成 Token
	token, err := jwtx.GetToken(config.AppConfig.Jwt.AccessSecret, time.Now().Unix(), config.AppConfig.Jwt.AccessExpire, int64(user.ID))

	if err != nil {
		return nil, xerr.NewErrCode(xerr.TOKEN_GEN_ERROR)
	}

	// 4. Model -> VO 转换
	resp := &vo.UserLoginResp{
		Token:    token,
		UserInfo: vo.NewUserInfo(user), // 使用 VO 的构造函数
	}

	// 5. 返回 VO
	return resp, nil
}
