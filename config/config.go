package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
	TomlPath      string
	ServiceName   string
}

func Load() (*Config, error) {
	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// .env 文件不存在不是错误，只是记录日志
		log.Printf("未找到 .env 文件，使用环境变量或默认值: %v", err)
	}

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		TomlPath:      getEnv("TOML_PATH", "/etc/frp/frpc.toml"),
		ServiceName:   getEnv("SERVICE_NAME", "frpc"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

