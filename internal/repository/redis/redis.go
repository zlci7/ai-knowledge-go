package redis

import (
	"ai-knowledge-go/config"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client

func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:         config.AppConfig.Database.RedisAddr,
		Password:     config.AppConfig.Database.RedisPw,
		DB:           0,
		PoolSize:     100, // 增加连接池大小（默认10）
		MinIdleConns: 20,  // 最小空闲连接
	})

	// 测试连接
	_, err := Rdb.Ping(context.Background()).Result()
	if err != nil {
		panic("❌ Redis连接失败: " + err.Error())
	}

	fmt.Println("✅ Redis 连接成功！") // ⬅️ 加上这行
}
