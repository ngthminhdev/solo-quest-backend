package cron

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/config"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

type NotificationSender interface {
	SendToUser(ctx context.Context, userID uuid.UUID, title, body string, data map[string]string) error
	IsEnabled() bool
}

type NotificationCronResult struct {
	ProcessedUsers int      `json:"processed_users"`
	SentCount      int      `json:"sent_count"`
	SkippedCount   int      `json:"skipped_count"`
	FailedCount    int      `json:"failed_count"`
	DurationMs     int64    `json:"duration_ms"`
	Errors         []string `json:"errors,omitempty"`
}

type NotificationCron struct {
	db             *gorm.DB
	notifSender    NotificationSender
	cfg            config.NotificationCronConfig
	running        int32
	mu             sync.Mutex
	stopChan       chan struct{}
	cursor         int
	cursorMu       sync.Mutex
}

func NewNotificationCron(db *gorm.DB, notifSender NotificationSender, cfg config.NotificationCronConfig) *NotificationCron {
	return &NotificationCron{
		db:          db,
		notifSender: notifSender,
		cfg:         cfg,
	}
}

func (c *NotificationCron) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cfg.Enabled {
		logger.L.Info("Notification cron is disabled, not starting scheduler")
		return
	}

	if c.stopChan != nil {
		logger.L.Warn("Notification cron scheduler is already running")
		return
	}

	c.stopChan = make(chan struct{})
	interval := time.Duration(c.cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}

	logger.L.Info("Starting notification cron scheduler",
		zap.Duration("interval", interval),
		zap.Int("batch_size", c.cfg.BatchSize),
		zap.Int("lookback_seconds", c.cfg.LookbackSeconds),
		zap.String("timezone", c.cfg.Timezone),
	)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopChan:
				logger.L.Info("Notification cron scheduler stopped")
				return
			case <-ctx.Done():
				logger.L.Info("Notification cron context cancelled")
				return
			case <-ticker.C:
				logger.L.Debug("Notification cron tick")
				result, err := c.RunOnce(ctx)
				if err != nil {
					logger.L.Error("Notification cron tick failed", zap.Error(err))
				} else {
					logger.L.Info("Notification cron tick completed",
						zap.Int("processed_users", result.ProcessedUsers),
						zap.Int("sent", result.SentCount),
						zap.Int("skipped", result.SkippedCount),
						zap.Int("failed", result.FailedCount),
						zap.Int64("duration_ms", result.DurationMs),
					)
				}
			}
		}
	}()
}

func (c *NotificationCron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
		logger.L.Info("Stopped notification cron scheduler")
	}
}

func (c *NotificationCron) RunOnce(ctx context.Context) (*NotificationCronResult, error) {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return nil, fmt.Errorf("notification cron is already running")
	}
	defer atomic.StoreInt32(&c.running, 0)

	batchSize := c.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 5
	}

	startTime := time.Now()

	result := &NotificationCronResult{}

	userIDs, err := c.getActiveUserBatch(batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	if len(userIDs) == 0 {
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	result.ProcessedUsers = len(userIDs)

	loc := timeutil.LocationVN
	if c.cfg.Timezone != "" {
		if l, err := time.LoadLocation(c.cfg.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now()
	lookback := time.Duration(c.cfg.LookbackSeconds) * time.Second
	windowStart := now.Add(-lookback)

	for _, userID := range userIDs {
		userSent, userSkipped, userFailed, userErrors := c.processUser(ctx, userID, now, windowStart, loc)
		result.SentCount += userSent
		result.SkippedCount += userSkipped
		result.FailedCount += userFailed
		result.Errors = append(result.Errors, userErrors...)
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result, nil
}

func (c *NotificationCron) getActiveUserBatch(limit int) ([]uuid.UUID, error) {
	var userIDs []uuid.UUID

	err := c.db.Model(&models.DeviceToken{}).
		Select("DISTINCT device_tokens.user_id").
		Joins("JOIN app_settings ON app_settings.user_id = device_tokens.user_id").
		Where("device_tokens.is_active = true AND device_tokens.deleted_at IS NULL").
		Where("app_settings.notifications_enabled = true").
		Order("device_tokens.user_id ASC").
		Limit(limit).
		Offset(c.getCursor()).
		Pluck("device_tokens.user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}

	if len(userIDs) > 0 {
		c.advanceCursor(len(userIDs))
	}

	return userIDs, nil
}

func (c *NotificationCron) getCursor() int {
	c.cursorMu.Lock()
	defer c.cursorMu.Unlock()
	return c.cursor
}

func (c *NotificationCron) advanceCursor(count int) {
	c.cursorMu.Lock()
	defer c.cursorMu.Unlock()
	c.cursor += count

	var total int64
	c.db.Model(&models.DeviceToken{}).
		Where("is_active = true AND deleted_at IS NULL").
		Distinct("user_id").
		Count(&total)

	if int64(c.cursor) >= total {
		c.cursor = 0
	}
}

func (c *NotificationCron) processUser(ctx context.Context, userID uuid.UUID, now, windowStart time.Time, loc *time.Location) (sent, skipped, failed int, errors []string) {
	settings, err := c.getAppSettings(userID)
	if err != nil {
		logger.L.Error("Failed to get app settings for user", zap.String("user_id", userID.String()), zap.Error(err))
		return 0, 0, 0, []string{fmt.Sprintf("user %s: %v", userID, err)}
	}

	userLoc := c.resolveUserTimezone(settings)
	userNow := now.In(userLoc)
	localDate := timeutil.FormatDateVN(userNow)

	if c.isInQuietHours(settings, userNow) {
		logger.L.Debug("User is in quiet hours, skipping non-critical reminders",
			zap.String("user_id", userID.String()),
		)
		// Skip all reminders during quiet hours for this user
		return 0, 0, 0, nil
	}

	s1, k1, f1, e1 := c.processQuestReminders(ctx, userID, now, windowStart, loc, localDate)
	s2, k2, f2, e2 := c.processSnoozeReminders(ctx, userID, now, windowStart, loc, localDate)
	s3, k3, f3, e3 := c.processReminderSettings(ctx, userID, userNow, now, loc, localDate, settings)

	sent = s1 + s2 + s3
	skipped = k1 + k2 + k3
	failed = f1 + f2 + f3
	errors = append(errors, e1...)
	errors = append(errors, e2...)
	errors = append(errors, e3...)

	return
}

func (c *NotificationCron) getAppSettings(userID uuid.UUID) (*models.AppSettings, error) {
	var settings models.AppSettings
	err := c.db.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (c *NotificationCron) resolveUserTimezone(settings *models.AppSettings) *time.Location {
	if settings.Timezone != "" {
		if loc, err := time.LoadLocation(settings.Timezone); err == nil {
			return loc
		}
	}
	return timeutil.LocationVN
}

func (c *NotificationCron) isInQuietHours(settings *models.AppSettings, userNow time.Time) bool {
	if !settings.QuietHoursEnabled {
		return false
	}

	if settings.QuietStartTime == nil || settings.QuietEndTime == nil {
		return false
	}

	return isTimeInRange(userNow, *settings.QuietStartTime, *settings.QuietEndTime)
}

func (c *NotificationCron) processQuestReminders(ctx context.Context, userID uuid.UUID, now, windowStart time.Time, loc *time.Location, localDate string) (sent, skipped, failed int, errors []string) {
	var quests []models.Quest

	err := c.db.Where("user_id = ?", userID).
		Where("status IN ?", []string{string(models.QuestStatusPending), string(models.QuestStatusActive)}).
		Where("reminder_time IS NOT NULL").
		Where("reminder_time <= ?", now).
		Where("reminder_time >= ?", windowStart).
		Find(&quests).Error
	if err != nil {
		return 0, 0, 0, []string{fmt.Sprintf("user %s quest reminder query: %v", userID, err)}
	}

	for _, quest := range quests {
		if quest.ReminderTime == nil {
			continue
		}

		reminderTimeStr := quest.ReminderTime.Format(time.RFC3339)
		idempotencyKey := fmt.Sprintf("quest_reminder:%s:%s", quest.ID.String(), reminderTimeStr)

		if c.isAlreadyProcessed(idempotencyKey) {
			skipped++
			continue
		}

		title := quest.Title
		body := fmt.Sprintf("Đến giờ làm nhiệm vụ: %s", quest.Title)
		data := buildQuestReminderPayload(userID, quest.ID, string(quest.Type), localDate)

		if c.sendNotification(ctx, userID, idempotencyKey, "quest_reminder", &quest.ID, nil, title, body, data) {
			sent++
		} else {
			failed++
		}
	}

	return
}

func (c *NotificationCron) processSnoozeReminders(ctx context.Context, userID uuid.UUID, now, windowStart time.Time, loc *time.Location, localDate string) (sent, skipped, failed int, errors []string) {
	var quests []models.Quest

	err := c.db.Where("user_id = ?", userID).
		Where("status = ?", string(models.QuestStatusSnoozed)).
		Where("snoozed_until IS NOT NULL").
		Where("snoozed_until <= ?", now).
		Where("snoozed_until >= ?", windowStart).
		Find(&quests).Error
	if err != nil {
		return 0, 0, 0, []string{fmt.Sprintf("user %s snooze reminder query: %v", userID, err)}
	}

	for _, quest := range quests {
		if quest.SnoozedUntil == nil {
			continue
		}

		snoozedUntilStr := quest.SnoozedUntil.Format(time.RFC3339)
		idempotencyKey := fmt.Sprintf("quest_snooze:%s:%s", quest.ID.String(), snoozedUntilStr)

		if c.isAlreadyProcessed(idempotencyKey) {
			skipped++
			continue
		}

		title := "Đến giờ quay lại nhiệm vụ"
		body := quest.Title
		data := buildQuestSnoozePayload(userID, quest.ID, string(quest.Type), localDate)

		if c.sendNotification(ctx, userID, idempotencyKey, "quest_snooze", &quest.ID, nil, title, body, data) {
			sent++
		} else {
			failed++
		}
	}

	return
}

func (c *NotificationCron) processReminderSettings(ctx context.Context, userID uuid.UUID, userNow, systemNow time.Time, loc *time.Location, localDate string, appSettings *models.AppSettings) (sent, skipped, failed int, errors []string) {
	var settings []models.ReminderSetting

	err := c.db.Where("user_id = ?", userID).
		Where("status = ?", string(models.ReminderStatusEnabled)).
		Find(&settings).Error
	if err != nil {
		return 0, 0, 0, []string{fmt.Sprintf("user %s reminder settings query: %v", userID, err)}
	}

	for _, setting := range settings {
		switch setting.Frequency {
		case models.ReminderFrequencyFixed:
			s, k, f := c.processFixedReminder(ctx, userID, &setting, userNow, systemNow, localDate, appSettings)
			sent += s
			skipped += k
			failed += f
		case models.ReminderFrequencyInterval:
			s, k, f := c.processIntervalReminder(ctx, userID, &setting, userNow, systemNow, localDate)
			sent += s
			skipped += k
			failed += f
		case models.ReminderFrequencyRandomInRange:
			s, k, f := c.processRandomInRangeReminder(ctx, userID, &setting, userNow, systemNow, localDate)
			sent += s
			skipped += k
			failed += f
		}
	}

	return
}

func (c *NotificationCron) processFixedReminder(ctx context.Context, userID uuid.UUID, setting *models.ReminderSetting, userNow, systemNow time.Time, localDate string, appSettings *models.AppSettings) (sent, skipped, failed int) {
	if setting.StartTime == nil || *setting.StartTime == "" {
		return 0, 0, 0
	}

	startTime := *setting.StartTime
	parsedTime, err := parseHHMMToToday(startTime, userNow)
	if err != nil {
		logger.L.Warn("Failed to parse start_time for fixed reminder",
			zap.String("user_id", userID.String()),
			zap.String("type", string(setting.Type)),
			zap.String("start_time", startTime),
			zap.Error(err),
		)
		return 0, 0, 0
	}

	if userNow.Before(parsedTime) {
		return 0, 0, 0
	}

	// Only send if within a reasonable window (within the current cron lookback)
	if systemNow.Sub(parsedTime) > time.Duration(c.cfg.LookbackSeconds)*time.Second {
		return 0, 0, 0
	}

	occurrenceTime := startTime
	idempotencyKey := fmt.Sprintf("reminder_setting:%s:%s:%s:%s", userID.String(), string(setting.Type), localDate, occurrenceTime)

	if c.isAlreadyProcessed(idempotencyKey) {
		skipped++
		return
	}

	content := getReminderContent(setting.Type)
	data := buildReminderSettingPayload(userID, localDate, setting, occurrenceTime)

	reminderID := setting.ID
	if c.sendNotification(ctx, userID, idempotencyKey, "reminder_setting", nil, &reminderID, content.Title, content.Body, data) {
		sent++
	} else {
		failed++
	}
	return
}

func (c *NotificationCron) processIntervalReminder(ctx context.Context, userID uuid.UUID, setting *models.ReminderSetting, userNow, systemNow time.Time, localDate string) (sent, skipped, failed int) {
	if setting.StartTime == nil || setting.EndTime == nil || *setting.StartTime == "" || *setting.EndTime == "" {
		return 0, 0, 0
	}
	if setting.IntervalMinutes == nil || *setting.IntervalMinutes <= 0 {
		return 0, 0, 0
	}

	startH, startM, err := parseHHMM(*setting.StartTime)
	if err != nil {
		return 0, 0, 0
	}
	endH, endM, err := parseHHMM(*setting.EndTime)
	if err != nil {
		return 0, 0, 0
	}

	startDay := time.Date(userNow.Year(), userNow.Month(), userNow.Day(), startH, startM, 0, 0, userNow.Location())
	endDay := time.Date(userNow.Year(), userNow.Month(), userNow.Day(), endH, endM, 0, 0, userNow.Location())

	interval := time.Duration(*setting.IntervalMinutes) * time.Minute
	maxPerDay := 999
	if setting.MaxPerDay != nil && *setting.MaxPerDay > 0 {
		maxPerDay = *setting.MaxPerDay
	}

	count := 0
	for t := startDay; !t.After(endDay) && count < maxPerDay; t = t.Add(interval) {
		count++

		if userNow.Before(t) {
			continue
		}

		if systemNow.Sub(t) > time.Duration(c.cfg.LookbackSeconds)*time.Second {
			continue
		}

		occurrenceTime := fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
		idempotencyKey := fmt.Sprintf("reminder_setting:%s:%s:%s:%s", userID.String(), string(setting.Type), localDate, occurrenceTime)

		if c.isAlreadyProcessed(idempotencyKey) {
			skipped++
			continue
		}

		content := getReminderContent(setting.Type)
		data := buildReminderSettingPayload(userID, localDate, setting, occurrenceTime)

		reminderID := setting.ID
		if c.sendNotification(ctx, userID, idempotencyKey, "reminder_setting", nil, &reminderID, content.Title, content.Body, data) {
			sent++
		} else {
			failed++
		}

		if count >= maxPerDay {
			break
		}
	}

	return
}

func (c *NotificationCron) processRandomInRangeReminder(ctx context.Context, userID uuid.UUID, setting *models.ReminderSetting, userNow, systemNow time.Time, localDate string) (sent, skipped, failed int) {
	if setting.StartTime == nil || setting.EndTime == nil || *setting.StartTime == "" || *setting.EndTime == "" {
		return 0, 0, 0
	}

	var occurrences []string
	if setting.IntervalMinutes != nil && *setting.IntervalMinutes > 0 {
		occurrences = generateJitteredIntervalOccurrences(userID, string(setting.Type), localDate, *setting.StartTime, *setting.EndTime, *setting.IntervalMinutes, setting.MaxPerDay, 10)
	} else {
		maxPerDay := 3
		if setting.MaxPerDay != nil && *setting.MaxPerDay > 0 {
			maxPerDay = *setting.MaxPerDay
		}
		occurrences = generateStableRandomOccurrences(userID, string(setting.Type), localDate, *setting.StartTime, *setting.EndTime, maxPerDay)
	}

	for _, occTime := range occurrences {
		parsedTime, err := parseHHMMToToday(occTime, userNow)
		if err != nil {
			continue
		}

		if userNow.Before(parsedTime) {
			continue
		}

		if systemNow.Sub(parsedTime) > time.Duration(c.cfg.LookbackSeconds)*time.Second {
			continue
		}

		idempotencyKey := fmt.Sprintf("reminder_setting:%s:%s:%s:%s", userID.String(), string(setting.Type), localDate, occTime)

		if c.isAlreadyProcessed(idempotencyKey) {
			skipped++
			continue
		}

		content := getReminderContent(setting.Type)
		data := buildReminderSettingPayload(userID, localDate, setting, occTime)

		reminderID := setting.ID
		if c.sendNotification(ctx, userID, idempotencyKey, "reminder_setting", nil, &reminderID, content.Title, content.Body, data) {
			sent++
		} else {
			failed++
		}
	}

	return
}

func (c *NotificationCron) isAlreadyProcessed(idempotencyKey string) bool {
	var count int64
	c.db.Model(&models.NotificationLog{}).
		Where("idempotency_key = ?", idempotencyKey).
		Where("status IN ?", []string{string(models.NotificationLogStatusPending), string(models.NotificationLogStatusSent)}).
		Count(&count)
	return count > 0
}

func (c *NotificationCron) sendNotification(ctx context.Context, userID uuid.UUID, idempotencyKey, eventType string, questID, reminderSettingID *uuid.UUID, title, body string, data map[string]string) bool {
	now := time.Now()
	logEntry := &models.NotificationLog{
		UserID:            userID,
		QuestID:           questID,
		ReminderSettingID: reminderSettingID,
		EventType:         eventType,
		Channel:           "fcm",
		IdempotencyKey:    idempotencyKey,
		Title:             title,
		Body:              body,
		Status:            models.NotificationLogStatusPending,
		ScheduledAt:       &now,
	}

	if data != nil {
		if err := logEntry.SetPayload(data); err != nil {
			logger.L.Warn("Failed to set payload on notification log entry", zap.Error(err))
		}
	}

	if err := c.db.Create(logEntry).Error; err != nil {
		// If duplicate key error, skip
		logger.L.Warn("Failed to create notification log (may be duplicate)",
			zap.String("idempotency_key", idempotencyKey),
			zap.Error(err),
		)
		return false
	}

	err := c.notifSender.SendToUser(ctx, userID, title, body, data)

	if err != nil {
		errStr := err.Error()
		logEntry.Status = models.NotificationLogStatusFailed
		logEntry.Error = &errStr
		c.db.Model(logEntry).Updates(map[string]interface{}{
			"status": logEntry.Status,
			"error":  logEntry.Error,
		})
		logger.L.Error("Failed to send notification",
			zap.String("user_id", userID.String()),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return false
	}

	sentAt := time.Now()
	logEntry.Status = models.NotificationLogStatusSent
	logEntry.SentAt = &sentAt
	c.db.Model(logEntry).Updates(map[string]interface{}{
		"status":  logEntry.Status,
		"sent_at": logEntry.SentAt,
	})
	return true
}

type reminderContent struct {
	Title string
	Body  string
}

func getReminderContent(reminderType models.ReminderType) reminderContent {
	switch reminderType {
	case models.ReminderTypeWater:
		return reminderContent{
			Title: "Uống nước",
			Body:  "Đến giờ uống một ngụm nước rồi.",
		}
	case models.ReminderTypeBreakTime:
		return reminderContent{
			Title: "Nghỉ mắt một chút",
			Body:  "Hãy rời mắt khỏi màn hình và nghỉ nhẹ vài phút.",
		}
	case models.ReminderTypeDailyReview:
		return reminderContent{
			Title: "Tổng kết ngày",
			Body:  "Dành vài phút nhìn lại ngày hôm nay.",
		}
	case models.ReminderTypeSleep:
		return reminderContent{
			Title: "Chuẩn bị đi ngủ",
			Body:  "Đến giờ thư giãn và chuẩn bị cho giấc ngủ.",
		}
	case models.ReminderTypeLearning:
		return reminderContent{
			Title: "Học tập",
			Body:  "Đến giờ dành một chút thời gian để học.",
		}
	case models.ReminderTypeMovement:
		return reminderContent{
			Title: "Vận động nhẹ",
			Body:  "Hãy đứng dậy và vận động nhẹ một chút.",
		}
	default:
		return reminderContent{
			Title: "Nhắc nhở",
			Body:  "Đến giờ thực hiện hoạt động của bạn.",
		}
	}
}

func parseHHMMToToday(timeStr string, today time.Time) (time.Time, error) {
	h, m, err := parseHHMM(timeStr)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(today.Year(), today.Month(), today.Day(), h, m, 0, 0, today.Location()), nil
}

func parseHHMM(timeStr string) (int, int, error) {
	timeStr = strings.TrimSpace(timeStr)
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return 0, 0, fmt.Errorf("invalid HH:MM format: %s", timeStr)
	}
	h, err1 := strconv.Atoi(timeStr[0:2])
	m, err2 := strconv.Atoi(timeStr[3:5])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time values: %s", timeStr)
	}
	return h, m, nil
}

func isTimeInRange(t time.Time, startStr, endStr string) bool {
	startH, startM, err1 := parseHHMM(startStr)
	endH, endM, err2 := parseHHMM(endStr)
	if err1 != nil || err2 != nil {
		return false
	}

	loc := t.Location()
	startTime := time.Date(t.Year(), t.Month(), t.Day(), startH, startM, 0, 0, loc)
	endTime := time.Date(t.Year(), t.Month(), t.Day(), endH, endM, 0, 0, loc)

	// Handle overnight quiet hours (e.g., 22:00 to 06:00)
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		// Overnight range: if current time is after start OR before end, it's in range
		return !t.Before(startTime) || t.Before(endTime)
	}

	return !t.Before(startTime) && t.Before(endTime)
}

func generateStableRandomOccurrences(userID uuid.UUID, reminderType, localDate, startTimeStr, endTimeStr string, maxCount int) []string {
	seedBytes := []byte(fmt.Sprintf("%s-%s-%s", userID.String(), reminderType, localDate))
	hash := md5.Sum(seedBytes)
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := rand.New(rand.NewSource(seed))

	startH, startM, err1 := parseHHMM(startTimeStr)
	endH, endM, err2 := parseHHMM(endTimeStr)
	if err1 != nil || err2 != nil {
		return nil
	}

	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM
	if endMinutes <= startMinutes {
		endMinutes += 24 * 60
	}

	var occurrences []string
	used := make(map[int]bool)

	for i := 0; i < maxCount; i++ {
		if len(occurrences) >= maxCount {
			break
		}

		rangeSize := endMinutes - startMinutes
		if rangeSize <= 0 {
			break
		}

		candidate := startMinutes + rng.Intn(rangeSize)
		// Try up to 20 times to find a non-duplicate slot
		for attempts := 0; attempts < 20 && used[candidate]; attempts++ {
			candidate = startMinutes + rng.Intn(rangeSize)
		}
		if used[candidate] {
			continue
		}

		used[candidate] = true
		h := candidate / 60
		m := candidate % 60
		occurrences = append(occurrences, fmt.Sprintf("%02d:%02d", h%24, m))
	}

	sort.Strings(occurrences)
	return occurrences
}

func generateJitteredIntervalOccurrences(
	userID uuid.UUID,
	reminderType string,
	localDate string,
	startTimeStr string,
	endTimeStr string,
	intervalMinutes int,
	maxPerDay *int,
	defaultJitterMinutes int,
) []string {
	seedBytes := []byte(fmt.Sprintf("%s-%s-%s", userID.String(), reminderType, localDate))
	hash := md5.Sum(seedBytes)
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := rand.New(rand.NewSource(seed))

	startH, startM, err1 := parseHHMM(startTimeStr)
	endH, endM, err2 := parseHHMM(endTimeStr)
	if err1 != nil || err2 != nil {
		return nil
	}

	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM
	if endMinutes <= startMinutes {
		endMinutes += 24 * 60
	}

	var accepted []int
	maxCount := 999999
	if maxPerDay != nil && *maxPerDay > 0 {
		maxCount = *maxPerDay
	}

	for i := 0; ; i++ {
		if len(accepted) >= maxCount {
			break
		}

		baseSlot := startMinutes + i*intervalMinutes
		if baseSlot > endMinutes {
			break
		}

		jitter := 0
		if defaultJitterMinutes > 0 {
			jitter = rng.Intn(2*defaultJitterMinutes+1) - defaultJitterMinutes
		}

		candidate := baseSlot + jitter

		// Keep occurrence within start/end window
		if candidate < startMinutes {
			candidate = startMinutes
		}
		if candidate > endMinutes {
			candidate = endMinutes
		}

		// Enforce mandatory minimum spacing
		if len(accepted) > 0 {
			prev := accepted[len(accepted)-1]
			if candidate < prev+intervalMinutes {
				candidate = prev + intervalMinutes
			}
		}

		// Check if occurrence can fit
		if candidate > endMinutes {
			break
		}

		accepted = append(accepted, candidate)
	}

	var occurrences []string
	seen := make(map[string]bool)
	for _, min := range accepted {
		h := min / 60
		m := min % 60
		formatted := fmt.Sprintf("%02d:%02d", h%24, m)
		if !seen[formatted] {
			seen[formatted] = true
			occurrences = append(occurrences, formatted)
		}
	}
	sort.Strings(occurrences)

	return occurrences
}

func buildQuestReminderPayload(userID uuid.UUID, questID uuid.UUID, questType string, localDate string) map[string]string {
	return map[string]string{
		"event":             "quest_reminder",
		"source":            "soloquest_backend",
		"user_id":           userID.String(),
		"local_date":        localDate,
		"quest_id":          questID.String(),
		"quest_type":        questType,
		"action":            "open_quest_detail",
		"display_mode":      "quest_detail",
		"countdown_enabled": "false",
		"countdown_minutes": "0",
	}
}

func buildQuestSnoozePayload(userID uuid.UUID, questID uuid.UUID, questType string, localDate string) map[string]string {
	return map[string]string{
		"event":             "quest_snooze",
		"source":            "soloquest_backend",
		"user_id":           userID.String(),
		"local_date":        localDate,
		"quest_id":          questID.String(),
		"quest_type":        questType,
		"action":            "open_quest_detail",
		"display_mode":      "quest_detail",
		"countdown_enabled": "false",
		"countdown_minutes": "0",
	}
}

func buildReminderSettingPayload(userID uuid.UUID, localDate string, setting *models.ReminderSetting, occurrenceTime string) map[string]string {
	intervalMinutesStr := "0"
	if setting.IntervalMinutes != nil {
		intervalMinutesStr = strconv.Itoa(*setting.IntervalMinutes)
	}

	var scheduleMode string
	switch setting.Frequency {
	case models.ReminderFrequencyFixed:
		scheduleMode = "fixed"
	case models.ReminderFrequencyInterval:
		scheduleMode = "interval"
	case models.ReminderFrequencyRandomInRange:
		if setting.IntervalMinutes != nil && *setting.IntervalMinutes > 0 {
			scheduleMode = "jittered_interval"
		} else {
			scheduleMode = "random_in_range"
		}
	default:
		scheduleMode = string(setting.Frequency)
	}

	var action, displayMode, countdownEnabled, countdownMinutes string
	switch setting.Type {
	case models.ReminderTypeWater:
		action = "water_reminder"
		displayMode = "water_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	case models.ReminderTypeBreakTime:
		action = "start_break_timer"
		displayMode = "break_timer_prompt"
		countdownEnabled = "true"
		countdownMinutes = "5"
	case models.ReminderTypeMovement:
		action = "movement_reminder"
		displayMode = "reminder_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	case models.ReminderTypeLearning:
		action = "learning_reminder"
		displayMode = "reminder_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	case models.ReminderTypeSleep:
		action = "sleep_reminder"
		displayMode = "reminder_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	case models.ReminderTypeDailyReview:
		action = "daily_review_reminder"
		displayMode = "daily_review_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	default:
		action = string(setting.Type) + "_reminder"
		displayMode = "reminder_prompt"
		countdownEnabled = "false"
		countdownMinutes = "0"
	}

	return map[string]string{
		"source":              "soloquest_backend",
		"event":              "reminder_setting",
		"user_id":            userID.String(),
		"local_date":         localDate,
		"reminder_type":      string(setting.Type),
		"reminder_setting_id": setting.ID.String(),
		"frequency":          string(setting.Frequency),
		"occurrence_time":    occurrenceTime,
		"interval_minutes":   intervalMinutesStr,
		"schedule_mode":      scheduleMode,
		"action":              action,
		"display_mode":       displayMode,
		"countdown_enabled":  countdownEnabled,
		"countdown_minutes":  countdownMinutes,
	}
}
