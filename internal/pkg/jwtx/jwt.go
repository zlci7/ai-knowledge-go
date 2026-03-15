package jwtx

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// GetToken 生成 JWT Token
// secretKey: 密钥 (来自配置文件)
// iat: 当前时间戳 (Seconds)
// seconds: 过期时间 (Seconds)
// uid: 用户ID
func GetToken(secretKey string, iat, seconds, uid int64) (string, error) {
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	// 这个 key "uid" 非常重要，后续在 API 网关解析时会用到
	claims["uid"] = uid

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ParseToken 解析 Token
func ParseToken(tokenString string, secretKey string) (jwt.MapClaims, error) {
	// 1. 解析 token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 校验签名算法是否匹配
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	// 2. 验证 token 有效性并提取 Claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
