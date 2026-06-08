package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	AppEnv           string
	ServiceName      string
	DatabaseURL      string
	DevUserEmail     string
	JWT              JWTConfig
	Google           GoogleConfig
	Logging          LoggingConfig
	Cron             CronConfig
	NotificationCron NotificationCronConfig
	FCM              FCMConfig
}

type CronConfig struct {
	DailyQuestEnabled   bool
	DailyQuestTime      string
	DailyQuestBatchSize int
	DailyQuestTimezone  string
}

type NotificationCronConfig struct {
	Enabled          bool
	IntervalSeconds  int
	BatchSize        int
	LookbackSeconds  int
	Timezone         string
}

type JWTConfig struct {
	Secret              string
	AccessTokenExpires  int
	RefreshTokenExpires int
}

type GoogleConfig struct {
	ClientID        string
	AndroidClientID string
	IOSClientID     string
}

type FCMConfig struct {
	Enabled         bool
	ProjectID       string
	CredentialsPath string
	CredentialsJSON string
	DryRun          bool
}

type LoggingConfig struct {
	Level           string
	Format          string
	Output          string
	SlowOperationMs int
	WriteToFiles    bool
	FileDir         string
	FileFolder      string
	FileName        string
	FileMaxSizeMB   int
	FileMaxBackups  int
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

	dailyQuestEnabled := getEnv("DAILY_QUEST_CRON_ENABLED", "false") == "true"
	dailyQuestTime := getEnv("DAILY_QUEST_CRON_TIME", "04:00")
	dailyQuestBatchSizeStr := getEnv("DAILY_QUEST_CRON_BATCH_SIZE", "5")
	dailyQuestTimezone := getEnv("DAILY_QUEST_CRON_TIMEZONE", "Asia/Ho_Chi_Minh")

	batchSize, err := strconv.Atoi(dailyQuestBatchSizeStr)
	if err != nil || batchSize <= 0 {
		batchSize = 5
	}

	isValidTime := true
	if len(dailyQuestTime) != 5 || dailyQuestTime[2] != ':' {
		isValidTime = false
	} else {
		hStr := dailyQuestTime[0:2]
		mStr := dailyQuestTime[3:5]
		h, err1 := strconv.Atoi(hStr)
		m, err2 := strconv.Atoi(mStr)
		if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
			isValidTime = false
		}
	}

	if !isValidTime {
		log.Printf("[Warning] Invalid DAILY_QUEST_CRON_TIME format: '%s', expected HH:mm. Disabling cron safely.", dailyQuestTime)
		dailyQuestEnabled = false
	}

	if dailyQuestEnabled {
		_, tzErr := time.LoadLocation(dailyQuestTimezone)
		if tzErr != nil {
			log.Printf("[Warning] Invalid DAILY_QUEST_CRON_TIMEZONE: '%s'. Disabling cron safely.", dailyQuestTimezone)
			dailyQuestEnabled = false
		}
	}

	serviceName := getEnv("SERVICE_NAME", "solo_quest_backend")

	notificationCronEnabled := getEnv("NOTIFICATION_CRON_ENABLED", "false") == "true"
	notificationCronInterval, _ := strconv.Atoi(getEnv("NOTIFICATION_CRON_INTERVAL_SECONDS", "60"))
	if notificationCronInterval <= 0 {
		notificationCronInterval = 60
	}
	notificationCronBatchSize, _ := strconv.Atoi(getEnv("NOTIFICATION_CRON_BATCH_SIZE", "5"))
	if notificationCronBatchSize <= 0 {
		notificationCronBatchSize = 5
	}
	notificationCronLookback, _ := strconv.Atoi(getEnv("NOTIFICATION_CRON_LOOKBACK_SECONDS", "90"))
	if notificationCronLookback <= 0 {
		notificationCronLookback = 90
	}
	notificationCronTimezone := getEnv("NOTIFICATION_CRON_TIMEZONE", "Asia/Ho_Chi_Minh")

	if notificationCronEnabled {
		_, tzErr := time.LoadLocation(notificationCronTimezone)
		if tzErr != nil {
			log.Printf("[Warning] Invalid NOTIFICATION_CRON_TIMEZONE: '%s'. Disabling notification cron.", notificationCronTimezone)
			notificationCronEnabled = false
		}
	}

	fcmEnabled := getEnv("FCM_ENABLED", "false") == "true"
	fcmDryRun := getEnv("FCM_DRY_RUN", "true") == "true"

	if fcmEnabled && getEnv("FCM_PROJECT_ID", "") == "" {
		log.Println("[Warning] FCM_ENABLED=true but FCM_PROJECT_ID is empty. Disabling FCM.")
		fcmEnabled = false
	}

	return &Config{
		Port:         getEnv("PORT", "9000"),
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
			ClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
			AndroidClientID: getEnv("GOOGLE_ANDROID_CLIENT_ID", ""),
			IOSClientID:     getEnv("GOOGLE_IOS_CLIENT_ID", ""),
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
		Cron: CronConfig{
			DailyQuestEnabled:   dailyQuestEnabled,
			DailyQuestTime:      dailyQuestTime,
			DailyQuestBatchSize: batchSize,
			DailyQuestTimezone:  dailyQuestTimezone,
		},
		NotificationCron: NotificationCronConfig{
			Enabled:         notificationCronEnabled,
			IntervalSeconds: notificationCronInterval,
			BatchSize:       notificationCronBatchSize,
			LookbackSeconds: notificationCronLookback,
			Timezone:        notificationCronTimezone,
		},
		FCM: FCMConfig{
			Enabled:         fcmEnabled,
			ProjectID:       getEnv("FCM_PROJECT_ID", ""),
			CredentialsPath: getEnv("FCM_CREDENTIALS_PATH", ""),
			CredentialsJSON: getEnv("FCM_CREDENTIALS_JSON", ""),
			DryRun:          fcmDryRun,
		},
	}
}

func (g GoogleConfig) AllowedAudiences() []string {
	values := []string{g.ClientID, g.AndroidClientID, g.IOSClientID}
	audiences := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		audiences = append(audiences, value)
	}
	return audiences
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
