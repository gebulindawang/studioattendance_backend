package config

import (
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
)

var (
	AppPort            string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	JWTSecret          string
	AttendanceLocation *time.Location

	// 阿里云 OSS
	OSSEndpoint        string
	OSSRegion          string
	OSSBucket          string
	OSSAccessKeyID     string
	OSSAccessKeySecret string

	// DeepSeek
	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	DeepSeekModel   string
)

func Load() error {

	_ = godotenv.Load()

	AppPort = getEnv("APP_PORT", "8080")
	DBHost = getEnv("DB_HOST", "127.0.0.1")
	DBPort = getEnv("DB_PORT", "3306")
	DBUser = getEnv("DB_USER", "root")
	DBPassword = getEnv("DB_PASSWORD", "")
	DBName = getEnv("DB_NAME", "studioattendance")
	JWTSecret = getEnv("JWT_SECRET", "")
	if len(JWTSecret) < 32 {
		return fmt.Errorf("请设置至少 32 字节的 JWT_SECRET")
	}
	var err error
	AttendanceLocation, err = time.LoadLocation(getEnv("ATTENDANCE_TIME_ZONE", "Asia/Shanghai"))
	if err != nil {
		return fmt.Errorf("考勤时区无效: %w", err)
	}

	OSSEndpoint = getEnv("OSS_ENDPOINT", "")
	OSSRegion = getEnv("OSS_REGION", "")
	OSSBucket = getEnv("OSS_BUCKET", "")
	OSSAccessKeyID = getEnv("OSS_ACCESS_KEY_ID", "")
	OSSAccessKeySecret = getEnv("OSS_ACCESS_KEY_SECRET", "")

	DeepSeekAPIKey = getEnv("DEEPSEEK_API_KEY", "")
	DeepSeekBaseURL = getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
	DeepSeekModel = getEnv("DEEPSEEK_MODEL", "deepseek-flash")
	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
