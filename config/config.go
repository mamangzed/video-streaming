package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSS3Bucket        string
	Port               string
	MaxFileSize        int64
	FFmpegPath         string
	EnableVideoProcessing bool
}

var AppConfig *Config

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️  No .env file found, using system environment variables")
	}

	AppConfig = &Config{
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		AWSS3Bucket:        getEnv("AWS_S3_BUCKET", ""),
		Port:               getEnv("PORT", "8080"),
		MaxFileSize:        parseFileSize(getEnv("MAX_FILE_SIZE", "500MB")), // Increased to 500MB
		FFmpegPath:         getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
		EnableVideoProcessing: getEnvBool("ENABLE_VIDEO_PROCESSING", true),
	}

	// Validate required fields - but don't fail, just warn
	if AppConfig.AWSAccessKeyID == "" || AppConfig.AWSSecretAccessKey == "" || AppConfig.AWSS3Bucket == "" {
		log.Println("⚠️  AWS credentials not configured. Running in local mode.")
		log.Println("   Set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_S3_BUCKET for S3 functionality")
		log.Println("   Use /api/v1/upload-local for testing without S3")
	} else {
		log.Println("✅ AWS credentials configured")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if value == "true" || value == "1" {
			return true
		}
		return false
	}
	return defaultValue
}

func parseFileSize(sizeStr string) int64 {
	if len(sizeStr) < 2 {
		return 500 * 1024 * 1024 // Default 500MB
	}

	size, err := strconv.ParseInt(sizeStr[:len(sizeStr)-2], 10, 64)
	if err != nil {
		return 500 * 1024 * 1024 // Default 500MB
	}

	unit := sizeStr[len(sizeStr)-2:]
	switch unit {
	case "KB":
		return size * 1024
	case "MB":
		return size * 1024 * 1024
	case "GB":
		return size * 1024 * 1024 * 1024
	default:
		return 500 * 1024 * 1024 // Default 500MB
	}
}