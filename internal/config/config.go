package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName         string
	AppEnv          string
	Port            string
	ShutdownTimeout time.Duration
	JWTSecret       string
	JWTExpiresIn    time.Duration

	DatabaseURL string

	RedisAddr     string
	RedisStream   string
	RedisGroup    string
	ConsumerID    string

	S3Endpoint        string
	S3Region          string
	S3AccessKey       string
	S3SecretKey       string
	S3UseSSL          bool
	S3BucketUploads   string
	S3BucketProcessed string
}

func Load() (Config, error) {
	cfg := Config{
		AppName:           getEnv("APP_NAME", "aurora-engine"),
		AppEnv:            getEnv("APP_ENV", "development"),
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgresql://aurora:aurora@localhost:5432/aurora?sslmode=disable"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream:       getEnv("REDIS_STREAM", "video-events"),
		RedisGroup:        getEnv("REDIS_GROUP", "media-workers"),
		ConsumerID:        getEnv("CONSUMER_ID", defaultHostname()),
		S3Endpoint:        getEnv("S3_ENDPOINT", "localhost:9000"),
		S3Region:          getEnv("S3_REGION", "eu-central-1"),
		S3AccessKey:       getEnv("S3_ACCESS_KEY", "rustfsadmin"),
		S3SecretKey:       getEnv("S3_SECRET_KEY", "rustfsadmin"),
		S3BucketUploads:   getEnv("S3_BUCKET_UPLOADS", "aurora-uploads"),
		S3BucketProcessed: getEnv("S3_BUCKET_PROCESSED", "aurora-processed"),
	}

	shutdownSeconds, err := getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 10)
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout = time.Duration(shutdownSeconds) * time.Second

	jwtMinutes, err := getEnvInt("JWT_EXPIRES_MINUTES", 60)
	if err != nil {
		return Config{}, err
	}
	cfg.JWTExpiresIn = time.Duration(jwtMinutes) * time.Minute
	cfg.JWTSecret = getEnv("JWT_SECRET", "change-me-in-production")

	useSSL, err := getEnvBool("S3_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}
	cfg.S3UseSSL = useSSL

	return cfg, nil
}

func defaultHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	raw := getEnv(key, strconv.FormatBool(fallback))
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return value, nil
}
