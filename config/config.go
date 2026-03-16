package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 配置结构体
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	OSS       OSSConfig       `mapstructure:"oss"`
	Jwt       JwtConfig       `mapstructure:"jwt"`
	Dashscope DashscopeConfig `mapstructure:"dashscope"`
	Qdrant    QdrantConfig    `mapstructure:"qdrant"`
	Memory    MemoryConfig    `mapstructure:"memory"`
	Knowledge KnowledgeConfig `mapstructure:"knowledge"`
	Tika      TikaConfig      `mapstructure:"tika"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	MySQL     string `mapstructure:"mysql"`
	RedisAddr string `mapstructure:"redis_addr"`
	RedisPw   string `mapstructure:"redis_pw"`
}

type OSSConfig struct {
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Endpoint  string `mapstructure:"endpoint"`
}

type JwtConfig struct {
	AccessSecret string `mapstructure:"access_secret"`
	AccessExpire int64  `mapstructure:"access_expire"`
}

type DashscopeConfig struct {
	APIKey             string `mapstructure:"api_key"`
	LLMModel           string `mapstructure:"llm_model"`
	EmbeddingModel     string `mapstructure:"embedding_model"`
	EmbeddingDimension int    `mapstructure:"embedding_dimension"`
	BaseURL            string `mapstructure:"base_url"`
}

type QdrantConfig struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	Collection          string `mapstructure:"collection"`
	MemoryCollection    string `mapstructure:"memory_collection"`
	KnowledgeCollection string `mapstructure:"knowledge_collection"`
}

type MemoryConfig struct {
	Async MemoryAsyncConfig `mapstructure:"async"`
}

type MemoryAsyncConfig struct {
	RetryMax         int    `mapstructure:"retry_max"`
	RetryBaseSeconds int    `mapstructure:"retry_base_seconds"`
	QueueKeyPrefix   string `mapstructure:"queue_key_prefix"`
}

type KnowledgeConfig struct {
	DefaultKBID      uint64 `mapstructure:"default_kb_id"`
	UploadMaxSizeMB  int64  `mapstructure:"upload_max_size_mb"`
	UploadTimeoutSec int    `mapstructure:"upload_timeout_sec"`
	StorageDir       string `mapstructure:"storage_dir"`
}

type TikaConfig struct {
	URL string `mapstructure:"url"`
}

// 全局配置实例
var AppConfig *Config

// InitConfig 初始化配置
func InitConfig(configPath string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 将配置映射到结构体
	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}
	applyDefaults(AppConfig)

	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Dashscope.EmbeddingModel == "" {
		cfg.Dashscope.EmbeddingModel = "text-embedding-v3"
	}
	if cfg.Dashscope.EmbeddingDimension <= 0 {
		cfg.Dashscope.EmbeddingDimension = 1024
	}
	if cfg.Qdrant.Host == "" {
		cfg.Qdrant.Host = "127.0.0.1"
	}
	if cfg.Qdrant.Port <= 0 {
		cfg.Qdrant.Port = 6333
	}
	if cfg.Qdrant.MemoryCollection == "" {
		if cfg.Qdrant.Collection != "" {
			cfg.Qdrant.MemoryCollection = cfg.Qdrant.Collection
		} else {
			cfg.Qdrant.MemoryCollection = "long_term_memories"
		}
	}
	if cfg.Qdrant.KnowledgeCollection == "" {
		cfg.Qdrant.KnowledgeCollection = "knowledge_chunks"
	}
	if cfg.Memory.Async.RetryMax <= 0 {
		cfg.Memory.Async.RetryMax = 3
	}
	if cfg.Memory.Async.RetryBaseSeconds <= 0 {
		cfg.Memory.Async.RetryBaseSeconds = 2
	}
	if cfg.Memory.Async.QueueKeyPrefix == "" {
		cfg.Memory.Async.QueueKeyPrefix = "memory:vector"
	}
	if cfg.Knowledge.DefaultKBID == 0 {
		cfg.Knowledge.DefaultKBID = 1
	}
	if cfg.Knowledge.UploadMaxSizeMB <= 0 {
		cfg.Knowledge.UploadMaxSizeMB = 10
	}
	if cfg.Knowledge.UploadTimeoutSec <= 0 {
		cfg.Knowledge.UploadTimeoutSec = 60
	}
	if cfg.Knowledge.StorageDir == "" {
		cfg.Knowledge.StorageDir = "components/uploads/documents"
	}
	if cfg.Tika.URL == "" {
		cfg.Tika.URL = "http://127.0.0.1:9998"
	}
}
