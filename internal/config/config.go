package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	AppEnv       string
	ServiceName  string
	DatabaseURL  string
	DevUserEmail string
	JWT          JWTConfig
	Google       GoogleConfig
	Logging      LoggingConfig
}

type JWTConfig struct {
	Secret               string
	AccessTokenExpires   int
	RefreshTokenExpires  int
}

type GoogleConfig struct {
	ClientID string
}

type LoggingConfig struct {
	Level              string
	Format             string
	Output             string
	SlowOperationMs    int
	WriteToFiles       bool
	FileDir            string
	FileFolder         string
	FileName           string
	FileMaxSizeMB      int
	FileMaxBackups     int
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	accessExpires, _ := strconv.Atoi(getEnv("JWT_ACCESS_TOKEN_EXPIRES_MINUTES", "60"))
	refreshExpires, _ := strconv.Atoi(getEnv("JWT_REFRESH_TOKEN_EXPIRES_DAYS", "30"))
	slowOperationMs, _ := strconv.Atoi(getEnv("LOG_SLOW_OPERATION_MS", "1500"))
	fileMaxSizeMB, _ := strconv.Atoi(getEnv("LOG_FILE_MAX_SIZE_MB", "50"))
	fileMaxBackups, _ := strconv.Atoi(getEnv("LOG_FILE_MAX_BACKUPS", "7"))

	serviceName := getEnv("SERVICE_NAME", "soloquest-backend")

	return &Config{
		Port:         getEnv("PORT", "8080"),
		AppEnv:       getEnv("APP_ENV", "development"),
		ServiceName:  serviceName,
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres@localhost:5432/soloquest?sslmode=disable"),
		DevUserEmail: getEnv("DEV_USER_EMAIL", "minhthanh@gmail.com"),
		JWT: JWTConfig{
			Secret:              getEnv("JWT_SECRET", "change_me"),
			AccessTokenExpires:  accessExpires,
			RefreshTokenExpires: refreshExpires,
		},
		Google: GoogleConfig{
			ClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		},
		Logging: LoggingConfig{
			Level:           getEnv("LOG_LEVEL", "info"),
			Format:          getEnv("LOG_FORMAT", "json"),
			Output:          getEnv("LOG_OUTPUT", "stdout"),
			SlowOperationMs: slowOperationMs,
			WriteToFiles:    getEnv("IS_WRITE_LOG_TO_FILES", "false") == "true",
			FileDir:         getEnv("LOG_FILE_DIR", "logs"),
			FileFolder:      getEnv("LOG_FILE_FOLDER", serviceName),
			FileName:        getEnv("LOG_FILE_NAME", serviceName+".log"),
			FileMaxSizeMB:   fileMaxSizeMB,
			FileMaxBackups:  fileMaxBackups,
		},
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
