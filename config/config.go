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
	Dashscope DashscopeConfig `mapstructure:"dashscope"` // 新增
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
	APIKey   string `mapstructure:"api_key"`
	LLMModel string `mapstructure:"llm_model"`
	BaseURL  string `mapstructure:"base_url"`
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

	return nil
}
