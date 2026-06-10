package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

const (
	LearningRoadmapWorkerTimeout      = 120 * time.Second
	LearningRoadmapStaleJobThreshold  = 15 * time.Minute
	LearningRoadmapPollAfterSeconds   = 3
	maxRoadmapJobSafeErrorMessageSize = 300
)

type StartLearningRoadmapGenerationResult struct {
	JobID            string
	Status           string
	Source           string
	PollAfterSeconds int
	Item             *dto.LearningRoadmapItem
	GeneratedSteps   int
}

type LearningRoadmapGenerationError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type LearningRoadmapGenerationStatus struct {
	JobID              string                          `json:"job_id"`
	Status             string                          `json:"status"`
	StartedAt          *time.Time                      `json:"started_at"`
	CompletedAt        *time.Time                      `json:"completed_at"`
	Error              *LearningRoadmapGenerationError `json:"error"`
	Item               *dto.LearningRoadmapItem        `json:"item"`
	Source             string                          `json:"source,omitempty"`
	GeneratedStepCount int                             `json:"generated_step_count,omitempty"`
}

type normalizedRoadmapGenerationPreferences struct {
	LearningGoal string `json:"learning_goal"`
	Category     string `json:"category"`
	Difficulty   string `json:"difficulty"`
	MaxDuration  int    `json:"max_duration"`
}

func (s *LearningRoadmapService) StartRoadmapGeneration(
	ctx context.Context,
	userID uuid.UUID,
	req dto.GenerateLearningRoadmapRequest,
) (*StartLearningRoadmapGenerationResult, error) {
	if _, _, err := validateGenerateRoadmapRequest(req); err != nil {
		return nil, err
	}

	hash, prefsJSON, err := BuildLearningRoadmapRequestHash(req)
	if err != nil {
		return nil, err
	}

	job, created, err := s.getOrCreateRoadmapGenerationJob(ctx, userID, hash, prefsJSON)
	if err != nil {
		return nil, err
	}

	if created {
		logger.L.Info("learning roadmap generation job created",
			zap.String("job_id", job.ID.String()),
			zap.String("user_id", userID.String()),
			zap.String("request_hash", hash),
			zap.String("goal_preview", goalPreview(req.Preferences.LearningGoal)),
		)
	} else {
		logger.L.Info("learning roadmap generation job reused",
			zap.String("job_id", job.ID.String()),
			zap.String("user_id", userID.String()),
			zap.String("status", job.Status),
		)
	}

	if job.Status == models.LearningRoadmapGenJobStatusCompleted && job.RoadmapID != nil {
		item, _ := s.getGeneratedRoadmapItem(ctx, userID, *job.RoadmapID)
		return &StartLearningRoadmapGenerationResult{
			JobID:            job.ID.String(),
			Status:           models.LearningRoadmapGenJobStatusCompleted,
			Source:           string(models.LearningRoadmapSourceAI),
			PollAfterSeconds: LearningRoadmapPollAfterSeconds,
			Item:             item,
			GeneratedSteps:   job.GeneratedStepCount,
		}, nil
	}

	claimed, err := s.claimRoadmapGenerationJob(ctx, job.ID)
	if err != nil {
		return nil, err
	}
	if claimed {
		s.launchRoadmapGenerationWorker(job.ID)
	}

	return &StartLearningRoadmapGenerationResult{
		JobID:            job.ID.String(),
		Status:           models.LearningRoadmapGenJobStatusGenerating,
		Source:           string(models.LearningRoadmapSourceAI),
		PollAfterSeconds: LearningRoadmapPollAfterSeconds,
	}, nil
}

func (s *LearningRoadmapService) getOrCreateRoadmapGenerationJob(
	ctx context.Context,
	userID uuid.UUID,
	requestHash string,
	preferences datatypes.JSON,
) (*models.LearningRoadmapGenerationJob, bool, error) {
	var job models.LearningRoadmapGenerationJob
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND request_hash = ?", userID, requestHash).
		First(&job).Error
	if err == nil {
		return &job, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("failed to load learning roadmap generation job: %w", err)
	}

	job = models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: requestHash,
		Preferences: preferences,
		Status:      models.LearningRoadmapGenJobStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(&job).Error; err != nil {
		if err2 := s.db.WithContext(ctx).
			Where("user_id = ? AND request_hash = ?", userID, requestHash).
			First(&job).Error; err2 != nil {
			return nil, false, fmt.Errorf("failed to create learning roadmap generation job: %w", err)
		}
		return &job, false, nil
	}
	return &job, true, nil
}

func (s *LearningRoadmapService) claimRoadmapGenerationJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	now := timeutil.NowVN()
	staleBefore := now.Add(-LearningRoadmapStaleJobThreshold)

	res := s.db.WithContext(ctx).
		Model(&models.LearningRoadmapGenerationJob{}).
		Where(
			"id = ? AND (status IN ? OR (status = ? AND (started_at IS NULL OR started_at < ?)))",
			jobID,
			[]string{
				models.LearningRoadmapGenJobStatusPending,
				models.LearningRoadmapGenJobStatusFailed,
			},
			models.LearningRoadmapGenJobStatusGenerating,
			staleBefore,
		).
		Updates(map[string]interface{}{
			"status":               models.LearningRoadmapGenJobStatusGenerating,
			"started_at":           now,
			"completed_at":         nil,
			"error_type":           nil,
			"error_message":        nil,
			"generated_step_count": 0,
		})
	if res.Error != nil {
		return false, fmt.Errorf("failed to claim learning roadmap generation job: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		logger.L.Info("learning roadmap generation job claimed", zap.String("job_id", jobID.String()))
	}
	return res.RowsAffected == 1, nil
}

func (s *LearningRoadmapService) launchRoadmapGenerationWorker(jobID uuid.UUID) {
	if s.workerLauncher != nil {
		s.workerLauncher(jobID)
	}
}

func (s *LearningRoadmapService) ProcessRoadmapGenerationJob(ctx context.Context, jobID uuid.UUID) {
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("learning roadmap generation worker panicked",
				zap.String("job_id", jobID.String()),
				zap.Any("panic", r),
			)
			s.markRoadmapGenerationJobFailed(jobID, models.LearningRoadmapGenErrorInternal, "Learning roadmap generation failed. Please try again.")
		}
	}()

	var job models.LearningRoadmapGenerationJob
	if err := s.db.First(&job, "id = ?", jobID).Error; err != nil {
		logger.L.Error("learning roadmap generation worker: job not found",
			zap.String("job_id", jobID.String()), zap.Error(err))
		return
	}

	if job.Status == models.LearningRoadmapGenJobStatusCompleted && job.RoadmapID != nil {
		return
	}

	now := timeutil.NowVN()
	s.db.Model(&models.LearningRoadmapGenerationJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     models.LearningRoadmapGenJobStatusGenerating,
			"started_at": now,
		})

	req, err := decodeRoadmapGenerationRequest(job.Preferences)
	if err != nil {
		logger.L.Warn("learning roadmap generation validation failed",
			zap.String("job_id", jobID.String()),
			zap.String("user_id", job.UserID.String()),
			zap.Error(err),
		)
		s.markRoadmapGenerationJobFailed(jobID, models.LearningRoadmapGenErrorValidation, "Learning roadmap request is invalid.")
		return
	}

	workerCtx, cancel := context.WithTimeout(context.Background(), LearningRoadmapWorkerTimeout)
	defer cancel()

	logger.L.Info("learning roadmap AI generation started",
		zap.String("job_id", jobID.String()),
		zap.String("user_id", job.UserID.String()),
		zap.String("goal_preview", goalPreview(req.Preferences.LearningGoal)),
	)
	startedAt := time.Now()
	result, err := s.generateRoadmapSync(workerCtx, job.UserID, req)
	duration := time.Since(startedAt)
	if err != nil {
		errorType, safeMessage := mapRoadmapGenerationError(err)
		logger.L.Error("learning roadmap generation job failed",
			zap.String("job_id", jobID.String()),
			zap.String("user_id", job.UserID.String()),
			zap.String("error_type", errorType),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		s.markRoadmapGenerationJobFailed(jobID, errorType, safeMessage)
		return
	}

	logger.L.Info("learning roadmap AI generation completed",
		zap.String("job_id", jobID.String()),
		zap.String("user_id", job.UserID.String()),
		zap.String("roadmap_id", result.Item.ID.String()),
		zap.Int("generated_step_count", result.GeneratedStepCount),
		zap.Duration("duration", duration),
	)
	s.markRoadmapGenerationJobCompleted(jobID, result)

	_ = ctx
}

func (s *LearningRoadmapService) markRoadmapGenerationJobCompleted(jobID uuid.UUID, result *dto.GenerateLearningRoadmapResponse) {
	updates := map[string]interface{}{
		"status":               models.LearningRoadmapGenJobStatusCompleted,
		"roadmap_id":           result.Item.ID,
		"generated_step_count": result.GeneratedStepCount,
		"completed_at":         timeutil.NowVN(),
		"error_type":           nil,
		"error_message":        nil,
	}
	if err := s.db.Model(&models.LearningRoadmapGenerationJob{}).
		Where("id = ?", jobID).Updates(updates).Error; err != nil {
		logger.L.Error("failed to mark learning roadmap generation job completed",
			zap.String("job_id", jobID.String()), zap.Error(err))
	}
}

func (s *LearningRoadmapService) markRoadmapGenerationJobFailed(jobID uuid.UUID, errorType, safeMessage string) {
	if len(safeMessage) > maxRoadmapJobSafeErrorMessageSize {
		safeMessage = safeMessage[:maxRoadmapJobSafeErrorMessageSize]
	}
	updates := map[string]interface{}{
		"status":        models.LearningRoadmapGenJobStatusFailed,
		"completed_at":  timeutil.NowVN(),
		"error_type":    errorType,
		"error_message": safeMessage,
	}
	if err := s.db.Model(&models.LearningRoadmapGenerationJob{}).
		Where("id = ?", jobID).Updates(updates).Error; err != nil {
		logger.L.Error("failed to mark learning roadmap generation job failed",
			zap.String("job_id", jobID.String()), zap.Error(err))
	}
}

func (s *LearningRoadmapService) GetRoadmapGenerationStatus(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) (*LearningRoadmapGenerationStatus, error) {
	var job models.LearningRoadmapGenerationJob
	err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", jobID, userID).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoadmapNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load learning roadmap generation job: %w", err)
	}

	status := job.Status
	if status == models.LearningRoadmapGenJobStatusGenerating &&
		job.StartedAt != nil &&
		timeutil.NowVN().Sub(*job.StartedAt) > LearningRoadmapStaleJobThreshold {
		status = "stale"
	}

	out := &LearningRoadmapGenerationStatus{
		JobID:       job.ID.String(),
		Status:      status,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
	}

	switch status {
	case models.LearningRoadmapGenJobStatusCompleted:
		out.Source = string(models.LearningRoadmapSourceAI)
		out.GeneratedStepCount = job.GeneratedStepCount
		if job.RoadmapID != nil {
			item, err := s.getGeneratedRoadmapItem(ctx, userID, *job.RoadmapID)
			if err != nil {
				return nil, err
			}
			out.Item = item
		}
	case models.LearningRoadmapGenJobStatusFailed:
		out.Error = &LearningRoadmapGenerationError{
			Type:    valueOrDefault(job.ErrorType, models.LearningRoadmapGenErrorInternal),
			Message: valueOrDefault(job.ErrorMessage, "Learning roadmap generation failed. Please try again."),
		}
	case "stale":
		out.Error = &LearningRoadmapGenerationError{
			Type:    "timeout",
			Message: "Learning roadmap generation is taking too long. Please retry.",
		}
	}

	return out, nil
}

func BuildLearningRoadmapRequestHash(req dto.GenerateLearningRoadmapRequest) (string, datatypes.JSON, error) {
	storedPrefs := normalizedRoadmapGenerationPreferences{
		LearningGoal: strings.TrimSpace(req.Preferences.LearningGoal),
		Category:     strings.TrimSpace(req.Preferences.Category),
		Difficulty:   strings.ToLower(strings.TrimSpace(req.Preferences.Difficulty)),
		MaxDuration:  req.Preferences.MaxDuration,
	}
	if storedPrefs.Difficulty == "" {
		storedPrefs.Difficulty = "any"
	}
	if storedPrefs.MaxDuration == 0 {
		storedPrefs.MaxDuration = 300
	}

	hashPrefs := normalizedRoadmapGenerationPreferences{
		LearningGoal: strings.ToLower(strings.TrimSpace(req.Preferences.LearningGoal)),
		Category:     strings.ToLower(strings.TrimSpace(req.Preferences.Category)),
		Difficulty:   strings.ToLower(strings.TrimSpace(req.Preferences.Difficulty)),
		MaxDuration:  req.Preferences.MaxDuration,
	}
	if hashPrefs.Difficulty == "" {
		hashPrefs.Difficulty = "any"
	}
	if hashPrefs.MaxDuration == 0 {
		hashPrefs.MaxDuration = 300
	}

	hashBytes, err := json.Marshal(hashPrefs)
	if err != nil {
		return "", nil, err
	}
	storedBytes, err := json.Marshal(storedPrefs)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(hashBytes)
	return hex.EncodeToString(sum[:]), datatypes.JSON(storedBytes), nil
}

func decodeRoadmapGenerationRequest(raw datatypes.JSON) (dto.GenerateLearningRoadmapRequest, error) {
	var prefs normalizedRoadmapGenerationPreferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return dto.GenerateLearningRoadmapRequest{}, err
	}
	return dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: prefs.LearningGoal,
			Category:     prefs.Category,
			Difficulty:   prefs.Difficulty,
			MaxDuration:  prefs.MaxDuration,
		},
	}, nil
}

func mapRoadmapGenerationError(err error) (string, string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return models.LearningRoadmapGenErrorTimeout, "Learning roadmap generation timed out. Please try again."
	case errors.Is(err, ErrAIDisabled), errors.Is(err, ErrAIProviderFailed):
		return models.LearningRoadmapGenErrorAI, "AI generation is temporarily unavailable. Please try again later."
	case errors.Is(err, ErrAIInvalidOutput), errors.Is(err, ErrAITooFewValidSteps):
		return models.LearningRoadmapGenErrorInvalid, "AI could not generate a valid learning roadmap. Please adjust the goal and try again."
	case errors.Is(err, ErrEmptyLearningGoal), errors.Is(err, ErrInvalidMaxDuration):
		return models.LearningRoadmapGenErrorValidation, "Learning roadmap request is invalid."
	default:
		return models.LearningRoadmapGenErrorInternal, "Learning roadmap generation failed. Please try again."
	}
}

func valueOrDefault(v *string, fallback string) string {
	if v == nil || *v == "" {
		return fallback
	}
	return *v
}

func goalPreview(goal string) string {
	goal = strings.TrimSpace(goal)
	if len([]rune(goal)) <= 80 {
		return goal
	}
	runes := []rune(goal)
	return string(runes[:80])
}

func (s *LearningRoadmapService) getGeneratedRoadmapItem(
	ctx context.Context,
	userID uuid.UUID,
	roadmapID uuid.UUID,
) (*dto.LearningRoadmapItem, error) {
	var roadmap models.LearningRoadmap
	if err := s.db.WithContext(ctx).
		Where("id = ? AND enabled = ? AND created_by_user_id = ?", roadmapID, true, userID).
		First(&roadmap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}

	var steps []models.LearningRoadmapStep
	if err := s.db.WithContext(ctx).
		Where("roadmap_id = ? AND enabled = ?", roadmapID, true).
		Order("order_index ASC").
		Find(&steps).Error; err != nil {
		return nil, err
	}

	stepItems := make([]dto.LearningRoadmapStepItem, 0, len(steps))
	for _, step := range steps {
		stepItems = append(stepItems, dto.LearningRoadmapStepItem{
			ID:               step.ID,
			Title:            step.Title,
			Description:      step.Description,
			OrderIndex:       step.OrderIndex,
			EstimatedMinutes: step.EstimatedMinutes,
			Completed:        false,
			CompletedAt:      nil,
		})
	}

	return &dto.LearningRoadmapItem{
		ID:               roadmap.ID,
		Title:            roadmap.Title,
		Description:      roadmap.Description,
		Category:         roadmap.Category,
		Difficulty:       roadmap.Difficulty,
		EstimatedMinutes: roadmap.EstimatedMinutes,
		TotalSteps:       roadmap.TotalSteps,
		CompletedSteps:   0,
		ProgressPercent:  0,
		Source:           string(roadmap.Source),
		Status:           "",
		Enabled:          roadmap.Enabled,
		StartedAt:        nil,
		CompletedAt:      nil,
		Steps:            stepItems,
	}, nil
}
