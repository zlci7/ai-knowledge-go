package xerr

import "fmt"

// 常用通用错误码
const (
	OK = 200
	//通用错误
	SERVER_COMMON_ERROR = 100001 // 服务器开小差了
	REUQEST_PARAM_ERROR = 100002 // 参数错误
	TOKEN_EXPIRE_ERROR  = 100003 // Token过期
	DB_ERROR            = 100004 // 数据库错误
	TOO_MANY_REQUESTS   = 100005

	// 用户模块错误码 (200xxx)
	USER_ALREADY_EXISTS = 200001 // 用户已注册
	USER_NOT_FOUND      = 200002 // 用户不存在
	USER_PASSWORD_ERROR = 200003 // 密码错误
	USER_ENCRYPT_ERROR  = 200004 // 密码加密失败
	USER_SAVE_ERROR     = 200005 // 用户保存失败
	USER_ID_GET_ERROR   = 200006 // 用户ID获取失败
	USER_NOT_LOGIN      = 200007 // 用户未登录
	USER_CREATE_ERROR   = 200008 // 用户创建失败
	TOKEN_GEN_ERROR     = 200009 // Token生成失败
	TOKEN_NOT_EXIST     = 200010 // Token不存在
	TOKEN_INVALID       = 200011 // Token无效、
	TOKEN_USER_ID_ERROR = 200012 // 用户ID获取失败

	// 长期记忆模块错误码 (300xxx)
	MEMORY_CREATE_ERROR = 300001 // 长期记忆创建失败
	MEMORY_LIST_ERROR   = 300002 // 长期记忆查询失败
	MEMORY_DELETE_ERROR = 300003 // 长期记忆删除失败
	MEMORY_NOT_FOUND    = 300004 // 长期记忆不存在

	// 知识文档模块错误码 (400xxx)
	DOCUMENT_UPLOAD_ERROR       = 400001 // 文档上传失败
	DOCUMENT_LIST_ERROR         = 400002 // 文档查询失败
	DOCUMENT_DELETE_ERROR       = 400003 // 文档删除失败
	DOCUMENT_NOT_FOUND          = 400004 // 文档不存在
	KNOWLEDGE_BASE_NOT_FOUND    = 400005 // 知识库不存在
	DOCUMENT_PARAM_ERROR        = 400006 // 文档参数错误
	DOCUMENT_UNSUPPORTED_FORMAT = 400007 // 文档格式不支持
)

// CodeError 自定义错误结构体
type CodeError struct {
	errCode uint32
	errMsg  string
}

// 1. 获取错误码
func (e *CodeError) GetErrCode() uint32 {
	return e.errCode
}

// 2. 获取错误信息
func (e *CodeError) GetErrMsg() string {
	return e.errMsg
}

// 3. 实现 error 接口
func (e *CodeError) Error() string {
	return fmt.Sprintf("ErrCode:%d，ErrMsg:%s", e.errCode, e.errMsg)
}

// NewErrCode 工厂方法：通过错误码创建错误
func NewErrCode(errCode uint32) *CodeError {
	return &CodeError{errCode: errCode, errMsg: MapErrMsg(errCode)}
}

// NewErrMsg 工厂方法：创建自定义消息的错误
func NewErrMsg(errMsg string) *CodeError {
	return &CodeError{errCode: SERVER_COMMON_ERROR, errMsg: errMsg}
}
