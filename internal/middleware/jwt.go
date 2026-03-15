package middleware

import (
	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/pkg/jwtx"
	"ai-knowledge-go/internal/pkg/response"
	"ai-knowledge-go/internal/pkg/xerr"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT 认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1️⃣ 从 HTTP Header 获取 Token
		token := c.GetHeader("Authorization")
		if token == "" {
			response.Error(c, xerr.TOKEN_NOT_EXIST, "")
			c.Abort() // 拦截请求，不再执行后续 Handler
			return
		}

		// 2️⃣ 调用你的 ParseToken 解析
		claims, err := jwtx.ParseToken(token, config.AppConfig.Jwt.AccessSecret)
		if err != nil {
			response.Error(c, xerr.TOKEN_INVALID, "")
			c.Abort()
			return
		}

		// 3️⃣ 提取 UserID（这里假设是 float64 类型，因为 JSON 默认数字是 float64）
		uid, ok := claims["uid"].(float64)
		if !ok {
			response.Error(c, xerr.TOKEN_USER_ID_ERROR, "")
			c.Abort()
			return
		}

		// 4️⃣ ✨核心步骤：注入到 Context ✨
		c.Set("user_id", uint(uid))

		// 5️⃣ 继续执行后续的 Handler
		c.Next()
	}
}
