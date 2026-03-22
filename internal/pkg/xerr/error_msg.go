package xerr

var message map[uint32]string

func init() {
	message = make(map[uint32]string)
	message[OK] = "SUCCESS"
	message[SERVER_COMMON_ERROR] = "服务器开小差了,请稍后再试"
	message[REUQEST_PARAM_ERROR] = "参数错误"
	message[TOKEN_EXPIRE_ERROR] = "token失效，请重新登陆"
	message[DB_ERROR] = "数据库繁忙,请稍后再试"
	message[TOO_MANY_REQUESTS] = "请求过于频繁,请稍后再试"

	// --- 用户模块错误 200xxx ---
	message[USER_ALREADY_EXISTS] = "用户已注册"
	message[USER_NOT_FOUND] = "用户不存在"
	message[USER_PASSWORD_ERROR] = "密码错误"
	message[USER_ENCRYPT_ERROR] = "密码加密失败"
	message[USER_SAVE_ERROR] = "用户保存失败"
	message[USER_ID_GET_ERROR] = "用户ID获取失败"
	message[USER_NOT_LOGIN] = "用户未登录"
	message[USER_CREATE_ERROR] = "用户创建失败"
	message[TOKEN_GEN_ERROR] = "Token生成失败"
	message[TOKEN_NOT_EXIST] = "Token不存在"
	message[TOKEN_INVALID] = "Token无效"
	message[TOKEN_USER_ID_ERROR] = "用户ID获取失败"

	// --- 长期记忆模块错误 300xxx ---
	message[MEMORY_CREATE_ERROR] = "长期记忆创建失败"
	message[MEMORY_LIST_ERROR] = "长期记忆查询失败"
	message[MEMORY_DELETE_ERROR] = "长期记忆删除失败"
	message[MEMORY_NOT_FOUND] = "记忆不存在"

	// --- 会话模块错误 350xxx ---
	message[CONVERSATION_LIST_ERROR] = "会话列表查询失败"
	message[CONVERSATION_MESSAGE_LIST_ERROR] = "会话消息查询失败"
	message[CONVERSATION_DELETE_ERROR] = "会话删除失败"
	message[CONVERSATION_NOT_FOUND] = "会话不存在或无权限"

	// --- 知识文档模块错误 400xxx ---
	message[DOCUMENT_UPLOAD_ERROR] = "文档上传失败"
	message[DOCUMENT_LIST_ERROR] = "文档查询失败"
	message[DOCUMENT_DELETE_ERROR] = "文档删除失败"
	message[DOCUMENT_NOT_FOUND] = "文档不存在"
	message[KNOWLEDGE_BASE_NOT_FOUND] = "知识库不存在"
	message[DOCUMENT_PARAM_ERROR] = "文档参数错误"
	message[DOCUMENT_UNSUPPORTED_FORMAT] = "文档格式不支持"

}

func MapErrMsg(errcode uint32) string {
	if msg, ok := message[errcode]; ok {
		return msg
	} else {
		return "服务器开小差了,请稍后再试"
	}
}

// 判断是否为自定义错误
func IsCodeErr(errcode uint32) bool {
	if _, ok := message[errcode]; ok {
		return true
	} else {
		return false
	}
}
