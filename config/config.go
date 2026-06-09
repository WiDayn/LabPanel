package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	AppTitle             string
	Port                 string
	JWTSecret            string
	AdminUsername        string
	AdminHashedPassword  string
	TomlPath             string
	AppService           string
	FrpService           string
	FrpcPath             string
	DocsPath             string
	UploadPath           string
	LxcImage             string
	LxcBackupDir         string
	LxcGroupsPath        string
	MetricsDBPath        string
	MetricsRetentionDays int
}

func Load() (*Config, error) {
	if err := migrateDotEnvPassword(); err != nil {
		log.Printf("迁移管理员密码配置失败: %v", err)
	}

	// 加载 .env 文件（如果存在）
	if err := godotenv.Overload(); err != nil {
		// .env 文件不存在不是错误，只是记录日志
		log.Printf("未找到 .env 文件，使用环境变量或默认值: %v", err)
	}

	frpcPath := getEnv("FRPC_PATH", "")
	if frpcPath == "" {
		log.Printf("警告: 未找到frpc，使用默认路径 %s，请通过FRPC_PATH环境变量设置正确的路径", frpcPath)
	}

	cfg := &Config{
		AppTitle:             getEnv("APP_TITLE", "LabPanel 管理面板"),
		Port:                 getEnv("PORT", "8080"),
		JWTSecret:            getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AdminUsername:        getEnv("ADMIN_USERNAME", "admin"),
		AdminHashedPassword:  getAdminHashedPassword(),
		TomlPath:             getEnv("TOML_PATH", "/etc/frp/frpc.toml"),
		AppService:           normalizeServiceName(getEnv("APP_SERVICE", "labpanel")),
		FrpService:           normalizeServiceName(getEnvWithFallback("FRP_SERVICE", "SERVICE_NAME", "frpc")),
		FrpcPath:             frpcPath,
		DocsPath:             getEnv("DOCS_PATH", "./docs"),
		UploadPath:           getEnv("UPLOAD_PATH", "./uploads"),
		LxcImage:             getEnv("LXC_IMAGE", "ubuntu:22.04"),
		LxcBackupDir:         getEnv("LXC_BACKUP_DIR", "./backups"),
		LxcGroupsPath:        getEnv("LXC_GROUPS_PATH", "./data/lxc_groups.json"),
		MetricsDBPath:        getEnv("METRICS_DB_PATH", "./data/metrics.db"),
		MetricsRetentionDays: getEnvInt("METRICS_RETENTION_DAYS", 30),
	}

	return cfg, nil
}

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

func getAdminHashedPassword() string {
	if value := os.Getenv("ADMIN_HASHED_PASSWORD"); value != "" {
		return value
	}

	legacyPassword := getEnv("ADMIN_PASSWORD", "admin")
	hashed, err := HashPassword(legacyPassword)
	if err != nil {
		log.Printf("生成默认管理员密码哈希失败: %v", err)
		return ""
	}
	return hashed
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

func getEnvInt(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("环境变量 %s=%q 不是有效整数，使用默认值 %d", key, value, defaultValue)
		return defaultValue
	}
	if parsed < 0 {
		return 0
	}
	return parsed
}

func normalizeServiceName(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(name), ".service")
}
