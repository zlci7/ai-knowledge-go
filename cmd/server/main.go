package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"ai-knowledge-go/config"
	"ai-knowledge-go/internal/api/router"
	"ai-knowledge-go/internal/pkg/idgen"
	"ai-knowledge-go/internal/repository/mysql"
	"ai-knowledge-go/internal/repository/redis"
	"ai-knowledge-go/internal/repository/vector"
	"ai-knowledge-go/internal/service"
)

func main() {
	// 0. 性能优化：设置GOMAXPROCS（利用所有CPU核心）
	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Printf("✅ GOMAXPROCS设置为:%d\n", runtime.GOMAXPROCS(0))

	// 1. 初始化配置（在所有操作之前）
	if err := config.InitConfig("../../config"); err != nil { // ⬅️ 修改路径
		log.Fatalf("❌ 初始化配置失败: %v", err)
	}
	fmt.Println("✅ 配置加载成功！")

	// 2. 初始化数据库 (MySQL)
	mysql.InitMySQL()

	// 3. 初始化 Redis
	redis.InitRedis()

	// 4. 初始化雪花算法（生成订单号）
	if err := idgen.InitSnowflake(1); err != nil {
		log.Fatalf("❌ 初始化雪花算法失败: %v", err)
	}
	fmt.Println("✅ 雪花算法初始化成功！")

	// 4.1 初始化 Qdrant 及 collection
	if err := vector.InitQdrant(context.Background()); err != nil {
		log.Fatalf("❌ 初始化Qdrant失败: %v", err)
	}
	fmt.Println("✅ Qdrant 初始化成功！")

	// 4.2 启动长期记忆异步向量任务消费
	if err := service.MemoryAsync.Start(context.Background()); err != nil {
		log.Fatalf("❌ 启动长期记忆异步任务失败: %v", err)
	}
	fmt.Println("✅ 长期记忆异步任务已启动")

	// // 4.5 初始化布隆过滤器（防止缓存穿透）
	// if err := bloom.InitProductBloom(); err != nil {
	// 	log.Printf("⚠️  初始化商品布隆过滤器失败: %v", err)
	// }
	// if err := bloom.InitSeckillBloom(); err != nil {
	// 	log.Printf("⚠️  初始化秒杀布隆过滤器失败: %v", err)
	// }

	// // 4.6 初始化限流器
	// middleware.InitRateLimiters()
	// fmt.Println("✅ 限流器初始化成功！")

	// // 5. 启动秒杀订单消费者（异步写入MySQL） - 启动3个并发处理
	// for i := 1; i <= 3; i++ {
	// 	go consumer.ConsumeSeckillOrders()
	// 	fmt.Printf("✅ 秒杀订单消费者 #%d 已启动\n", i)
	// }

	// // 6. 启动订单超时扫描器（统一处理普通订单和秒杀订单）
	// consumer.StartSeckillOrderTimeoutScanner()
	// fmt.Println("✅ 订单超时扫描器已启动")

	// 7. 初始化 Gin 框架
	r := router.InitRouter()

	// 8. 启动服务（非阻塞）
	addr := config.AppConfig.Server.Port
	fmt.Printf("🚀 服务启动成功，监听地址：%s\n", addr)
	fmt.Println("📌 本地访问: http://localhost" + addr + "/ping")
	fmt.Println("📌 外部访问: http://192.168.100.128" + addr + "/ping")

	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatalf("❌ 启动服务失败: %v\n", err)
		}
	}()

	// 临时调试
	fmt.Println("JWT Secret Key:", config.AppConfig.Jwt.AccessSecret)
	fmt.Println("JWT Expire:", config.AppConfig.Jwt.AccessExpire)

	// 9. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 收到停止信号，正在关闭...")

	// 停止异步任务协程
	service.MemoryAsync.Stop()
	fmt.Println("✅ 长期记忆异步任务已停止")

	// 关闭数据库连接
	if sqlDB, err := mysql.DB.DB(); err == nil {
		sqlDB.Close()
		fmt.Println("✅ 数据库连接已关闭")
	}

	// 关闭 Redis 连接
	if redis.Rdb != nil {
		redis.Rdb.Close()
		fmt.Println("✅ Redis 连接已关闭")
	}

	fmt.Println("✅ 服务已关闭")
}
