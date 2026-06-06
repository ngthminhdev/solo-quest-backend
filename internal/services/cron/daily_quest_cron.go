package cron

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/pkg/logger"
)

type DailyQuestGenerator interface {
	GenerateToday(
		ctx context.Context,
		userID uuid.UUID,
		req quest_generation.GenerateTodayRequest,
	) (*quest_generation.GenerateTodayResult, error)
}

type DailyQuestCronResult struct {
	TargetDate     string           `json:"target_date"`
	ProcessedUsers int              `json:"processed_users"`
	GeneratedCount int              `json:"generated_count"`
	ExistingCount  int              `json:"existing_count"`
	FallbackCount  int              `json:"fallback_count"`
	FailedCount    int              `json:"failed_count"`
	DurationMs     int64            `json:"duration_ms"`
	UserResults    []UserCronResult `json:"user_results"`
}

type UserCronResult struct {
	UserID           uuid.UUID `json:"user_id"`
	Date             string    `json:"date"`
	Source           string    `json:"source"`
	Inserted         bool      `json:"inserted"`
	ExistingReturned bool      `json:"existing_returned"`
	FallbackUsed     bool      `json:"fallback_used"`
	AIErrorType      string    `json:"ai_error_type,omitempty"`
	ErrorType        string    `json:"error_type,omitempty"`
}

type DailyQuestCron struct {
	db        *gorm.DB
	generator DailyQuestGenerator
	cfg       *config.Config
	running   int32
	mu        sync.Mutex
	stopChan  chan struct{}
}

func NewDailyQuestCron(
	db *gorm.DB,
	generator DailyQuestGenerator,
	cfg *config.Config,
) *DailyQuestCron {
	return &DailyQuestCron{
		db:        db,
		generator: generator,
		cfg:       cfg,
	}
}

func (c *DailyQuestCron) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cfg.Cron.DailyQuestEnabled {
		logger.L.Info("Daily quest cron is disabled, not starting scheduler")
		return
	}

	if c.stopChan != nil {
		logger.L.Warn("Daily quest cron scheduler is already running")
		return
	}

	c.stopChan = make(chan struct{})
	loc, err := time.LoadLocation(c.cfg.Cron.DailyQuestTimezone)
	if err != nil {
		loc = time.UTC
	}

	logger.L.Info("Starting daily quest cron scheduler",
		zap.String("target_time", c.cfg.Cron.DailyQuestTime),
		zap.String("timezone", c.cfg.Cron.DailyQuestTimezone),
		zap.Int("batch_size", c.cfg.Cron.DailyQuestBatchSize),
	)

	go func() {
		for {
			now := time.Now()
			delay, err := CalculateNextRunDelay(now, c.cfg.Cron.DailyQuestTime, loc)
			if err != nil {
				logger.L.Error("Failed to calculate next daily quest cron run delay", zap.Error(err))
				select {
				case <-c.stopChan:
					return
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Minute):
					continue
				}
			}

			logger.L.Info("Daily quest cron scheduled", zap.Duration("delay_until_run", delay))

			timer := time.NewTimer(delay)
			select {
			case <-c.stopChan:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				targetDate := time.Now().In(loc)
				logger.L.Info("Executing scheduled daily quest cron job", zap.Time("target_date", targetDate))
				_, runErr := c.RunOnce(ctx, targetDate)
				if runErr != nil {
					logger.L.Error("Scheduled daily quest cron run failed", zap.Error(runErr))
				}
			}
		}
	}()
}

func (c *DailyQuestCron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
		logger.L.Info("Stopped daily quest cron scheduler")
	}
}

func (c *DailyQuestCron) RunOnce(ctx context.Context, targetDate time.Time) (*DailyQuestCronResult, error) {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return nil, fmt.Errorf("cron job is already running")
	}
	defer atomic.StoreInt32(&c.running, 0)

	batchSize := c.cfg.Cron.DailyQuestBatchSize
	if batchSize <= 0 {
		batchSize = 5
	}

	dateStr := timeutil.FormatDateVN(targetDate)
	startTime := time.Now()

	logger.L.Info("Cron run started",
		zap.String("cron_start", startTime.Format(time.RFC3339)),
		zap.String("target_date", dateStr),
		zap.Int("batch_size", batchSize),
	)

	var userResults []UserCronResult
	var processedCount, generatedCount, existingCount, fallbackCount, failedCount int

	offset := 0
	for {
		var users []models.UserProfile
		// Fetch users in batches who completed onboarding
		err := c.db.Where("has_completed_onboarding = ?", true).
			Order("id ASC").
			Limit(batchSize).
			Offset(offset).
			Find(&users).Error
		if err != nil {
			return nil, fmt.Errorf("failed to fetch users: %w", err)
		}
		if len(users) == 0 {
			break
		}

		for _, user := range users {
			preferAI := true
			force := false
			replacePending := true
			req := quest_generation.GenerateTodayRequest{
				Date:               &dateStr,
				PreferAI:           &preferAI,
				Force:              &force,
				ReplacePendingOnly: &replacePending,
			}

			// Add context timeout per user with buffer: AI_QUEST_TIMEOUT_SECONDS + buffer
			aiCfg := ai.LoadConfig()
			timeoutSecs := aiCfg.QuestTimeoutSeconds
			if timeoutSecs <= 0 {
				timeoutSecs = aiCfg.TimeoutSeconds
			}
			if timeoutSecs <= 0 {
				timeoutSecs = 10 // fallback default
			}
			userTimeout := time.Duration(timeoutSecs+10) * time.Second

			userCtx, cancel := context.WithTimeout(ctx, userTimeout)

			res, genErr := c.generator.GenerateToday(userCtx, user.ID, req)
			cancel()

			userRes := UserCronResult{
				UserID: user.ID,
				Date:   dateStr,
			}

			if genErr != nil {
				failedCount++
				userRes.ErrorType = "failed"
				logger.L.Error("Cron quest generation failed for user",
					zap.String("user_id", user.ID.String()),
					zap.String("date", dateStr),
					zap.Error(genErr),
				)
			} else {
				userRes.Source = res.Source
				userRes.Inserted = res.Inserted
				userRes.ExistingReturned = res.ExistingReturned
				userRes.FallbackUsed = res.FallbackUsed
				userRes.AIErrorType = res.AIErrorType

				if res.ExistingReturned {
					existingCount++
				} else {
					generatedCount += res.GeneratedCount
					if res.FallbackUsed {
						fallbackCount++
					}
				}

				logger.L.Info("Cron quest generation succeeded for user",
					zap.String("user_id", user.ID.String()),
					zap.String("date", dateStr),
					zap.String("source", res.Source),
					zap.Bool("inserted", res.Inserted),
					zap.Bool("existing_returned", res.ExistingReturned),
					zap.Bool("fallback_used", res.FallbackUsed),
					zap.String("ai_error_type", res.AIErrorType),
				)
			}

			userResults = append(userResults, userRes)
			processedCount++
		}

		offset += len(users)
	}

	durationMs := time.Since(startTime).Milliseconds()
	cronResult := &DailyQuestCronResult{
		TargetDate:     dateStr,
		ProcessedUsers: processedCount,
		GeneratedCount: generatedCount,
		ExistingCount:  existingCount,
		FallbackCount:  fallbackCount,
		FailedCount:    failedCount,
		DurationMs:     durationMs,
		UserResults:    userResults,
	}

	logger.L.Info("Cron run ended",
		zap.String("cron_end", time.Now().Format(time.RFC3339)),
		zap.String("target_date", dateStr),
		zap.Int("batch_size", batchSize),
		zap.Int("users_processed", processedCount),
		zap.Int("generated_count", generatedCount),
		zap.Int("existing_count", existingCount),
		zap.Int("fallback_count", fallbackCount),
		zap.Int("failed_count", failedCount),
		zap.Int64("duration_ms", durationMs),
	)

	return cronResult, nil
}

func CalculateNextRunDelay(now time.Time, cronTime string, loc *time.Location) (time.Duration, error) {
	var hour, minute int
	_, err := fmt.Sscanf(cronTime, "%d:%d", &hour, &minute)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cron time '%s': %w", cronTime, err)
	}

	nowInLoc := now.In(loc)

	targetToday := time.Date(
		nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(),
		hour, minute, 0, 0, loc,
	)

	if targetToday.Before(nowInLoc) || targetToday.Equal(nowInLoc) {
		targetToday = targetToday.AddDate(0, 0, 1)
	}

	return targetToday.Sub(nowInLoc), nil
}
