package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppTitle      string
	Port          string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
	TomlPath      string
	AppService    string
	FrpService    string
	FrpcPath      string
	DocsPath      string
	UploadPath    string
	LxcImage      string
	LxcBackupDir  string
	LxcGroupsPath string
}

func Load() (*Config, error) {
	// 加载 .env 文件（如果存在）
	if err := godotenv.Load(); err != nil {
		// .env 文件不存在不是错误，只是记录日志
		log.Printf("未找到 .env 文件，使用环境变量或默认值: %v", err)
	}

	frpcPath := getEnv("FRPC_PATH", "")
	if frpcPath == "" {
		log.Printf("警告: 未找到frpc，使用默认路径 %s，请通过FRPC_PATH环境变量设置正确的路径", frpcPath)
	}

	cfg := &Config{
		AppTitle:      getEnv("APP_TITLE", "LabPanel 管理面板"),
		Port:          getEnv("PORT", "8080"),
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		TomlPath:      getEnv("TOML_PATH", "/etc/frp/frpc.toml"),
		AppService:    normalizeServiceName(getEnv("APP_SERVICE", "labpanel")),
		FrpService:    normalizeServiceName(getEnvWithFallback("FRP_SERVICE", "SERVICE_NAME", "frpc")),
		FrpcPath:      frpcPath,
		DocsPath:      getEnv("DOCS_PATH", "./docs"),
		UploadPath:    getEnv("UPLOAD_PATH", "./uploads"),
		LxcImage:      getEnv("LXC_IMAGE", "ubuntu:22.04"),
		LxcBackupDir:  getEnv("LXC_BACKUP_DIR", "./backups"),
		LxcGroupsPath: getEnv("LXC_GROUPS_PATH", "./lxc_groups.json"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvWithFallback(key, fallbackKey, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return getEnv(fallbackKey, defaultValue)
}

func normalizeServiceName(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(name), ".service")
}
