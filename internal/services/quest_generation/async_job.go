package quest_generation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

// StaleJobThreshold is how long a job may stay in "generating" before it is
// considered stuck and eligible for a retry/reset. It must comfortably exceed
// WorkerTimeout so a legitimately slow AI run is never marked stale.
const StaleJobThreshold = 900 * time.Second // 15 minutes

// EstimatedGenerationSeconds is a hint returned to the frontend for sizing its
// poll loop.
const EstimatedGenerationSeconds = 300

// maxJobErrorMessageLen bounds how much of an error string is persisted.
const maxJobErrorMessageLen = 500

// WorkerTimeout bounds the whole background generation (build context + AI call
// + rule-based fallback/top-up + DB save + job update). It must be larger than
// AICallTimeout so that, after the AI call exhausts its own deadline, there is
// still time left on the worker context for the fallback and DB writes.
const WorkerTimeout = 700 * time.Second

// AICallTimeout bounds a single AI provider call. Derived from the worker
// context so the AI call cannot consume the entire worker budget; the
// remaining time is reserved for fallback + DB persistence.
const AICallTimeout = 600 * time.Second

// JobInfo describes an asynchronous generation job in flight (HTTP 202).
type JobInfo struct {
	Date             string
	Status           string
	JobID            string
	EstimatedSeconds int
}

// StartResult is the outcome of StartTodayGeneration. Exactly one of Existing
// or Job is non-nil.
type StartResult struct {
	// Existing is set when today's quests already exist and were returned
	// synchronously (HTTP 200); no background job was started.
	Existing *GenerateTodayResult
	// Job is set when a background generation job is running (HTTP 202).
	Job *JobInfo
}

// JobStatus is the payload returned by the status endpoint.
type JobStatus struct {
	Date         string
	Status       string // not_started | generating | completed | failed | stale
	JobID        *string
	QuestCount   int
	Source       *string
	FallbackUsed bool
	ErrorMessage *string
}

func resolveLocalDate(date *string) (time.Time, string, error) {
	var localDate time.Time
	if date == nil || *date == "" {
		localDate = timeutil.TodayVN()
	} else {
		parsed, err := timeutil.ParseDateVN(*date)
		if err != nil {
			return time.Time{}, "", fmt.Errorf("invalid date format: %w", err)
		}
		localDate = parsed
	}
	return localDate, timeutil.FormatDateVN(localDate), nil
}

// StartTodayGeneration kicks off (or reuses) an asynchronous generation job for
// the user/date. It never blocks on the AI call: it returns either the existing
// quests (when force=false and quests already exist) or a job descriptor whose
// progress can be polled via GetJobStatus.
func (s *GenerationService) StartTodayGeneration(
	ctx context.Context,
	userID uuid.UUID,
	req GenerateTodayRequest,
) (*StartResult, error) {
	localDate, dateStr, err := resolveLocalDate(req.Date)
	if err != nil {
		return nil, err
	}
	day := timeutil.StartOfDayVN(localDate)

	force := false
	if req.Force != nil {
		force = *req.Force
	}
	preferAI := true
	if req.PreferAI != nil {
		preferAI = *req.PreferAI
	}

	// 1. If not forcing and today's quests already exist, return them
	//    synchronously without creating a job.
	if !force {
		start, end := timeutil.DayRangeVN(localDate)
		var existing []models.Quest
		if err := s.db.WithContext(ctx).
			Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
			Find(&existing).Error; err != nil {
			return nil, fmt.Errorf("failed to load existing quests: %w", err)
		}
		if len(existing) > 0 {
			models.SortQuests(existing)
			return &StartResult{Existing: &GenerateTodayResult{
				Date:             dateStr,
				Inserted:         false,
				ExistingReturned: true,
				Source:           "existing",
				PreservedCount:   len(existing),
				Quests:           existing,
			}}, nil
		}
	}

	// 2. Get or create the job row for this user/date.
	job, err := s.getOrCreateJob(ctx, userID, day, preferAI, force)
	if err != nil {
		return nil, err
	}

	// 3. Atomically claim the job for a (re)run unless it is already
	//    actively generating and not stale. This prevents duplicate workers
	//    for concurrent requests.
	claimed, err := s.claimJob(ctx, job.ID, preferAI, force)
	if err != nil {
		return nil, err
	}
	if claimed {
		s.launchWorker(job.ID)
	}

	return &StartResult{Job: &JobInfo{
		Date:             dateStr,
		Status:           models.QuestGenJobStatusGenerating,
		JobID:            job.ID.String(),
		EstimatedSeconds: EstimatedGenerationSeconds,
	}}, nil
}

func (s *GenerationService) getOrCreateJob(
	ctx context.Context,
	userID uuid.UUID,
	day time.Time,
	preferAI, force bool,
) (*models.DailyQuestGenerationJob, error) {
	var job models.DailyQuestGenerationJob
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND date = ?", userID, day).
		First(&job).Error
	if err == nil {
		return &job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to load generation job: %w", err)
	}

	job = models.DailyQuestGenerationJob{
		UserID:             userID,
		Date:               day,
		Status:             models.QuestGenJobStatusPending,
		PreferAI:           preferAI,
		Force:              force,
		ReplacePendingOnly: true,
	}
	if err := s.db.WithContext(ctx).Create(&job).Error; err != nil {
		// A concurrent request may have created the row first (unique
		// constraint on user_id+date). Re-fetch in that case.
		if err2 := s.db.WithContext(ctx).
			Where("user_id = ? AND date = ?", userID, day).
			First(&job).Error; err2 != nil {
			return nil, fmt.Errorf("failed to create generation job: %w", err)
		}
	}
	return &job, nil
}

// claimJob atomically transitions a job into "generating" if it is not already
// actively generating (or is stale). RowsAffected==1 means this caller won the
// claim and must launch the worker.
func (s *GenerationService) claimJob(
	ctx context.Context,
	jobID uuid.UUID,
	preferAI, force bool,
) (bool, error) {
	now := timeutil.NowVN()
	staleBefore := now.Add(-StaleJobThreshold)

	res := s.db.WithContext(ctx).
		Model(&models.DailyQuestGenerationJob{}).
		Where(
			"id = ? AND (status IN ? OR (status = ? AND (started_at IS NULL OR started_at < ?)))",
			jobID,
			[]string{
				models.QuestGenJobStatusPending,
				models.QuestGenJobStatusFailed,
				models.QuestGenJobStatusCompleted,
			},
			models.QuestGenJobStatusGenerating,
			staleBefore,
		).
		Updates(map[string]interface{}{
			"status":                 models.QuestGenJobStatusGenerating,
			"started_at":             now,
			"completed_at":           nil,
			"source":                 nil,
			"fallback_used":          false,
			"ai_error_type":          nil,
			"error_message":          nil,
			"generated_count":        0,
			"preserved_count":        0,
			"replaced_pending_count": 0,
			"prefer_ai":              preferAI,
			"force":                  force,
			"replace_pending_only":   true,
		})
	if res.Error != nil {
		return false, fmt.Errorf("failed to claim generation job: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

// ProcessJob runs the full background generation for a job: it delegates to
// GenerateToday (which handles AI, rule-based fallback and top-up) and records
// the outcome on the job row. It is safe to run in a goroutine — panics are
// recovered and marked as a failed job.
func (s *GenerationService) ProcessJob(ctx context.Context, jobID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("quest generation worker panicked",
				zap.String("job_id", jobID.String()),
				zap.Any("panic", r),
			)
			s.markJobFailed(jobID, "unknown", fmt.Sprintf("worker panic: %v", r), true)
		}
	}()

	var job models.DailyQuestGenerationJob
	if err := s.db.First(&job, "id = ?", jobID).Error; err != nil {
		logger.L.Error("quest generation worker: job not found",
			zap.String("job_id", jobID.String()), zap.Error(err))
		return
	}

	// Ensure the row reflects in-progress state (covers direct invocations
	// that did not go through claimJob, e.g. tests).
	s.db.Model(&models.DailyQuestGenerationJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     models.QuestGenJobStatusGenerating,
			"started_at": timeutil.NowVN(),
		})

	dateStr := timeutil.FormatDateVN(job.Date)
	preferAI := job.PreferAI
	force := job.Force
	replace := true

	// Bound the whole generation (jobCtx) so a hung provider cannot keep the
	// worker alive indefinitely. Use a fresh context (the originating HTTP
	// request is long gone). The AI call gets its own shorter aiCtx derived
	// from this inside GenerateToday, so when the AI deadline is exhausted
	// the fallback + DB save still run on a live context.
	genCtx, cancel := context.WithTimeout(context.Background(), WorkerTimeout)
	defer cancel()

	req := GenerateTodayRequest{
		Date:               &dateStr,
		PreferAI:           &preferAI,
		Force:              &force,
		ReplacePendingOnly: &replace,
	}

	startedAt := timeutil.NowVN()
	result, err := s.GenerateToday(genCtx, job.UserID, req)
	duration := time.Since(startedAt)

	if err != nil {
		logger.L.Error("quest generation worker failed",
			zap.String("job_id", jobID.String()),
			zap.String("user_id", job.UserID.String()),
			zap.String("date", dateStr),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		s.markJobFailed(jobID, mapAIErrorType(err), err.Error(), true)
		return
	}

	logger.L.Info("quest generation worker completed",
		zap.String("job_id", jobID.String()),
		zap.String("user_id", job.UserID.String()),
		zap.String("date", dateStr),
		zap.String("source", result.Source),
		zap.Bool("fallback_used", result.FallbackUsed),
		zap.Int("generated_count", result.GeneratedCount),
		zap.Duration("duration", duration),
	)
	s.markJobCompleted(jobID, result)
}

func (s *GenerationService) markJobCompleted(jobID uuid.UUID, result *GenerateTodayResult) {
	updates := map[string]interface{}{
		"status":                 models.QuestGenJobStatusCompleted,
		"completed_at":           timeutil.NowVN(),
		"generated_count":        result.GeneratedCount,
		"preserved_count":        result.PreservedCount,
		"replaced_pending_count": result.ReplacedPendingCount,
		"fallback_used":          result.FallbackUsed,
		"error_message":          nil,
		"source":                 normalizeJobSource(result.Source),
	}
	if result.AIErrorType != "" {
		updates["ai_error_type"] = result.AIErrorType
	} else {
		updates["ai_error_type"] = nil
	}
	if err := s.db.Model(&models.DailyQuestGenerationJob{}).
		Where("id = ?", jobID).Updates(updates).Error; err != nil {
		logger.L.Error("failed to mark job completed",
			zap.String("job_id", jobID.String()), zap.Error(err))
	}
}

func (s *GenerationService) markJobFailed(jobID uuid.UUID, aiErrorType, errMsg string, fallbackUsed bool) {
	safeMsg := sanitizeErrorMessage(errMsg)
	updates := map[string]interface{}{
		"status":        models.QuestGenJobStatusFailed,
		"completed_at":  timeutil.NowVN(),
		"fallback_used": fallbackUsed,
		"error_message": safeMsg,
	}
	if aiErrorType != "" {
		updates["ai_error_type"] = aiErrorType
	}
	if err := s.db.Model(&models.DailyQuestGenerationJob{}).
		Where("id = ?", jobID).Updates(updates).Error; err != nil {
		logger.L.Error("failed to mark job failed",
			zap.String("job_id", jobID.String()), zap.Error(err))
	}
}

// GetJobStatus reports the current state of a user/date generation job for the
// frontend to poll.
func (s *GenerationService) GetJobStatus(
	ctx context.Context,
	userID uuid.UUID,
	date *string,
) (*JobStatus, error) {
	localDate, dateStr, err := resolveLocalDate(date)
	if err != nil {
		return nil, err
	}
	day := timeutil.StartOfDayVN(localDate)

	var job models.DailyQuestGenerationJob
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND date = ?", userID, day).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No job recorded. Quests may still exist (e.g. created via the
		// synchronous rule-based path), so report their count.
		return &JobStatus{
			Date:       dateStr,
			Status:     "not_started",
			QuestCount: s.countQuests(ctx, userID, localDate),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load generation job: %w", err)
	}

	status := job.Status
	if status == models.QuestGenJobStatusGenerating &&
		job.StartedAt != nil &&
		timeutil.NowVN().Sub(*job.StartedAt) > StaleJobThreshold {
		status = "stale"
	}

	out := &JobStatus{
		Date:   dateStr,
		Status: status,
		JobID:  strPtr(job.ID.String()),
	}

	switch status {
	case models.QuestGenJobStatusCompleted:
		out.QuestCount = s.countQuests(ctx, userID, localDate)
		out.Source = job.Source
		out.FallbackUsed = job.FallbackUsed
	case models.QuestGenJobStatusFailed:
		out.FallbackUsed = job.FallbackUsed
		out.ErrorMessage = job.ErrorMessage
	default:
		// generating / stale: nothing generated yet.
	}

	return out, nil
}

func (s *GenerationService) countQuests(ctx context.Context, userID uuid.UUID, localDate time.Time) int {
	start, end := timeutil.DayRangeVN(localDate)
	var count int64
	s.db.WithContext(ctx).Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Count(&count)
	return int(count)
}

// normalizeJobSource maps a generation source to the job's allowed values,
// returning nil for anything that is not a real generation source (e.g.
// "existing").
func normalizeJobSource(source string) interface{} {
	switch source {
	case models.QuestGenJobSourceAI:
		return models.QuestGenJobSourceAI
	case models.QuestGenJobSourceRuleBased:
		return models.QuestGenJobSourceRuleBased
	default:
		return nil
	}
}

func sanitizeErrorMessage(msg string) string {
	if len(msg) > maxJobErrorMessageLen {
		return msg[:maxJobErrorMessageLen]
	}
	return msg
}

func strPtr(s string) *string {
	return &s
}
