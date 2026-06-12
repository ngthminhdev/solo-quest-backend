package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
)

var (
	ErrRoadmapNotFound      = errors.New("roadmap not found")
	ErrStepNotFound         = errors.New("step not found")
	ErrNotFollowingRoadmap  = errors.New("user is not following this roadmap")
	ErrStepNotInRoadmap     = errors.New("step does not belong to this roadmap")
	ErrSuggestionNotFound   = errors.New("AI suggestion not found")
	ErrInvalidCreateRequest = errors.New("invalid create roadmap request")
	ErrUnauthorizedAccess   = errors.New("unauthorized access to roadmap")
)

type LearningRoadmapService struct {
	db             *gorm.DB
	aiClient       ai.Client
	workerLauncher func(uuid.UUID)
}

func NewLearningRoadmapService(db *gorm.DB) *LearningRoadmapService {
	svc := &LearningRoadmapService{db: db}
	svc.workerLauncher = func(jobID uuid.UUID) {
		go svc.ProcessRoadmapGenerationJob(context.Background(), jobID)
	}
	return svc
}

func (s *LearningRoadmapService) SetAIClient(client ai.Client) {
	s.aiClient = client
}

func (s *LearningRoadmapService) SetRoadmapGenerationWorkerLauncher(launcher func(uuid.UUID)) {
	s.workerLauncher = launcher
}

func (s *LearningRoadmapService) ListRoadmaps(userID uuid.UUID) ([]dto.LearningRoadmapItem, error) {
	var roadmaps []models.LearningRoadmap
	if err := s.db.
		Where("enabled = ? AND (created_by_user_id IS NULL OR created_by_user_id = ?)", true, userID).
		Order("category ASC, title ASC").
		Find(&roadmaps).Error; err != nil {
		return nil, err
	}

	var userFollows []models.UserLearningRoadmap
	if err := s.db.Where("user_id = ?", userID).Find(&userFollows).Error; err != nil {
		return nil, err
	}
	followMap := make(map[uuid.UUID]models.UserLearningRoadmap)
	for _, f := range userFollows {
		followMap[f.RoadmapID] = f
	}

	var allSteps []models.LearningRoadmapStep
	if err := s.db.Where("enabled = ?", true).Order("order_index ASC").Find(&allSteps).Error; err != nil {
		return nil, err
	}
	stepsByRoadmap := make(map[uuid.UUID][]models.LearningRoadmapStep)
	for _, step := range allSteps {
		stepsByRoadmap[step.RoadmapID] = append(stepsByRoadmap[step.RoadmapID], step)
	}

	var allProgress []models.UserLearningRoadmapStepProgress
	if err := s.db.Where("user_id = ? AND completed = ?", userID, true).Find(&allProgress).Error; err != nil {
		return nil, err
	}
	progressMap := make(map[uuid.UUID]bool)
	for _, p := range allProgress {
		progressMap[p.StepID] = true
	}

	items := make([]dto.LearningRoadmapItem, 0, len(roadmaps))
	for _, rm := range roadmaps {
		follow, hasFollow := followMap[rm.ID]
		if hasFollow && follow.Status == models.UserLearningRoadmapStatusArchived {
			continue
		}

		steps := stepsByRoadmap[rm.ID]

		completedSteps := 0
		stepItems := make([]dto.LearningRoadmapStepItem, 0, len(steps))
		for _, step := range steps {
			completed := progressMap[step.ID]
			if completed {
				completedSteps++
			}
			var completedAt *time.Time
			if completed {
				for _, p := range allProgress {
					if p.StepID == step.ID {
						completedAt = p.CompletedAt
						break
					}
				}
			}
			stepItems = append(stepItems, dto.LearningRoadmapStepItem{
				ID:               step.ID,
				Title:            step.Title,
				Description:      step.Description,
				OrderIndex:       step.OrderIndex,
				EstimatedMinutes: step.EstimatedMinutes,
				Completed:        completed,
				CompletedAt:      completedAt,
			})
		}

		progressPercent := 0
		if rm.TotalSteps > 0 {
			progressPercent = (completedSteps * 100) / rm.TotalSteps
		}

		status := ""
		var startedAt *time.Time
		var completedAt *time.Time
		if hasFollow {
			status = string(follow.Status)
			startedAt = &follow.StartedAt
			completedAt = follow.CompletedAt
		}

		items = append(items, dto.LearningRoadmapItem{
			ID:               rm.ID,
			Title:            rm.Title,
			Description:      rm.Description,
			Category:         rm.Category,
			Difficulty:       rm.Difficulty,
			EstimatedMinutes: rm.EstimatedMinutes,
			TotalSteps:       rm.TotalSteps,
			CompletedSteps:   completedSteps,
			ProgressPercent:  progressPercent,
			Source:           string(rm.Source),
			Status:           status,
			Enabled:          rm.Enabled,
			StartedAt:        startedAt,
			CompletedAt:      completedAt,
			Steps:            stepItems,
		})
	}

	return items, nil
}

func (s *LearningRoadmapService) GetRoadmapDetail(userID uuid.UUID, roadmapID uuid.UUID) (*dto.LearningRoadmapItem, error) {
	var roadmap models.LearningRoadmap
	if err := s.db.Where("id = ? AND enabled = ? AND (created_by_user_id IS NULL OR created_by_user_id = ?)", roadmapID, true, userID).First(&roadmap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}

	var follow models.UserLearningRoadmap
	hasFollow := false
	if err := s.db.Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).First(&follow).Error; err == nil {
		hasFollow = true
		if follow.Status == models.UserLearningRoadmapStatusArchived {
			return nil, ErrRoadmapNotFound
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var steps []models.LearningRoadmapStep
	if err := s.db.Where("roadmap_id = ? AND enabled = ?", roadmapID, true).Order("order_index ASC").Find(&steps).Error; err != nil {
		return nil, err
	}

	var progressRecords []models.UserLearningRoadmapStepProgress
	if err := s.db.Where("user_id = ? AND roadmap_id = ? AND completed = ?", userID, roadmapID, true).Find(&progressRecords).Error; err != nil {
		return nil, err
	}
	progressMap := make(map[uuid.UUID]models.UserLearningRoadmapStepProgress)
	for _, p := range progressRecords {
		progressMap[p.StepID] = p
	}

	completedSteps := 0
	stepItems := make([]dto.LearningRoadmapStepItem, 0, len(steps))
	for _, step := range steps {
		completed := false
		var completedAt *time.Time
		if p, ok := progressMap[step.ID]; ok {
			completed = true
			completedAt = p.CompletedAt
		}
		if completed {
			completedSteps++
		}
		stepItems = append(stepItems, dto.LearningRoadmapStepItem{
			ID:               step.ID,
			Title:            step.Title,
			Description:      step.Description,
			OrderIndex:       step.OrderIndex,
			EstimatedMinutes: step.EstimatedMinutes,
			Completed:        completed,
			CompletedAt:      completedAt,
		})
	}

	progressPercent := 0
	if roadmap.TotalSteps > 0 {
		progressPercent = (completedSteps * 100) / roadmap.TotalSteps
	}

	status := ""
	var startedAt *time.Time
	var completedAt *time.Time
	if hasFollow {
		status = string(follow.Status)
		startedAt = &follow.StartedAt
		completedAt = follow.CompletedAt
	}

	item := &dto.LearningRoadmapItem{
		ID:               roadmap.ID,
		Title:            roadmap.Title,
		Description:      roadmap.Description,
		Category:         roadmap.Category,
		Difficulty:       roadmap.Difficulty,
		EstimatedMinutes: roadmap.EstimatedMinutes,
		TotalSteps:       roadmap.TotalSteps,
		CompletedSteps:   completedSteps,
		ProgressPercent:  progressPercent,
		Source:           string(roadmap.Source),
		Status:           status,
		Enabled:          roadmap.Enabled,
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		Steps:            stepItems,
	}

	return item, nil
}

func (s *LearningRoadmapService) DeleteRoadmap(userID uuid.UUID, roadmapID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var roadmap models.LearningRoadmap
		if err := tx.Where("id = ? AND enabled = ? AND created_by_user_id = ?", roadmapID, true, userID).First(&roadmap).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoadmapNotFound
			}
			return err
		}

		now := timeutil.NowUTC()
		if err := tx.Model(&models.LearningRoadmap{}).
			Where("id = ?", roadmapID).
			Updates(map[string]interface{}{
				"enabled":    false,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.LearningRoadmapStep{}).
			Where("roadmap_id = ?", roadmapID).
			Updates(map[string]interface{}{
				"enabled":    false,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		return tx.Model(&models.UserLearningRoadmap{}).
			Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).
			Updates(map[string]interface{}{
				"status":       models.UserLearningRoadmapStatusArchived,
				"completed_at": nil,
				"updated_at":   now,
			}).Error
	})
}

func (s *LearningRoadmapService) FollowRoadmap(userID uuid.UUID, roadmapID uuid.UUID) (*dto.FollowRoadmapResponse, error) {
	var roadmap models.LearningRoadmap
	if err := s.db.Where("id = ? AND enabled = ? AND (created_by_user_id IS NULL OR created_by_user_id = ?)", roadmapID, true, userID).First(&roadmap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}

	var existing models.UserLearningRoadmap
	if err := s.db.Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).First(&existing).Error; err == nil {
		if existing.Status == models.UserLearningRoadmapStatusArchived {
			now := timeutil.NowUTC()
			existing.Status = models.UserLearningRoadmapStatusTracking
			existing.CompletedAt = nil
			existing.UpdatedAt = now
			if existing.StartedAt.IsZero() {
				existing.StartedAt = now
			}
			if err := s.db.Save(&existing).Error; err != nil {
				return nil, err
			}

			logEntry := models.LogEntry{
				UserID:    userID,
				Type:      models.LogEntryTypeLearningRoadmapFollowed,
				Title:     "Bắt đầu theo dõi lộ trình",
				Content:   roadmap.Title,
				CreatedAt: now,
			}
			if err := s.db.Create(&logEntry).Error; err != nil {
				return nil, err
			}
		}

		return &dto.FollowRoadmapResponse{
			ID:          existing.ID,
			UserID:      existing.UserID,
			RoadmapID:   existing.RoadmapID,
			Status:      string(existing.Status),
			StartedAt:   existing.StartedAt,
			CompletedAt: existing.CompletedAt,
		}, nil
	}

	now := timeutil.NowUTC()
	follow := models.UserLearningRoadmap{
		UserID:    userID,
		RoadmapID: roadmapID,
		Status:    models.UserLearningRoadmapStatusTracking,
		StartedAt: now,
	}

	if err := s.db.Create(&follow).Error; err != nil {
		return nil, err
	}

	logEntry := models.LogEntry{
		UserID:    userID,
		Type:      models.LogEntryTypeLearningRoadmapFollowed,
		Title:     "Bắt đầu theo dõi lộ trình",
		Content:   roadmap.Title,
		CreatedAt: now,
	}
	if err := s.db.Create(&logEntry).Error; err != nil {
		return nil, err
	}

	return &dto.FollowRoadmapResponse{
		ID:          follow.ID,
		UserID:      follow.UserID,
		RoadmapID:   follow.RoadmapID,
		Status:      string(follow.Status),
		StartedAt:   follow.StartedAt,
		CompletedAt: follow.CompletedAt,
	}, nil
}

func (s *LearningRoadmapService) ToggleStep(userID uuid.UUID, roadmapID uuid.UUID, stepID uuid.UUID, completed bool) (*dto.ToggleRoadmapStepResponse, error) {
	var roadmap models.LearningRoadmap
	if err := s.db.Where("id = ? AND enabled = ? AND (created_by_user_id IS NULL OR created_by_user_id = ?)", roadmapID, true, userID).First(&roadmap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}

	var step models.LearningRoadmapStep
	if err := s.db.Where("id = ? AND roadmap_id = ? AND enabled = ?", stepID, roadmapID, true).First(&step).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStepNotFound
		}
		return nil, err
	}

	var follow models.UserLearningRoadmap
	if err := s.db.Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).First(&follow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFollowingRoadmap
		}
		return nil, err
	}
	if follow.Status == models.UserLearningRoadmapStatusArchived {
		return nil, ErrNotFollowingRoadmap
	}

	now := timeutil.NowUTC()

	var progress models.UserLearningRoadmapStepProgress
	err := s.db.Where("user_id = ? AND step_id = ?", userID, stepID).First(&progress).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		progress = models.UserLearningRoadmapStepProgress{
			UserID:    userID,
			RoadmapID: roadmapID,
			StepID:    stepID,
			Completed: completed,
		}
		if completed {
			progress.CompletedAt = &now
		}
		if err := s.db.Create(&progress).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		progress.Completed = completed
		progress.UpdatedAt = now
		if completed {
			progress.CompletedAt = &now
		} else {
			progress.CompletedAt = nil
		}
		if err := s.db.Save(&progress).Error; err != nil {
			return nil, err
		}
	}

	var completedSteps int64
	s.db.Model(&models.UserLearningRoadmapStepProgress{}).
		Where("user_id = ? AND roadmap_id = ? AND completed = ?", userID, roadmapID, true).
		Count(&completedSteps)

	totalSteps := roadmap.TotalSteps
	progressPercent := 0
	if totalSteps > 0 {
		progressPercent = (int(completedSteps) * 100) / totalSteps
	}

	roadmapWasCompleted := follow.Status == models.UserLearningRoadmapStatusCompleted

	if int(completedSteps) == totalSteps {
		follow.Status = models.UserLearningRoadmapStatusCompleted
		if follow.CompletedAt == nil {
			follow.CompletedAt = &now
		}
	} else {
		follow.Status = models.UserLearningRoadmapStatusTracking
		follow.CompletedAt = nil
	}
	follow.UpdatedAt = now
	s.db.Save(&follow)

	// Create log entries for step toggle
	if completed {
		stepLog := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapStepCompleted,
			Title:     "Hoàn thành bước học",
			Content:   step.Title,
			CreatedAt: now,
		}
		if err := s.db.Create(&stepLog).Error; err != nil {
			return nil, err
		}
	} else {
		stepLog := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapStepUncompleted,
			Title:     "Bỏ hoàn thành bước học",
			Content:   step.Title,
			CreatedAt: now,
		}
		if err := s.db.Create(&stepLog).Error; err != nil {
			return nil, err
		}
	}

	// Log roadmap completed only on first transition to completed
	if int(completedSteps) == totalSteps && !roadmapWasCompleted {
		roadmapLog := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCompleted,
			Title:     "Hoàn thành lộ trình học",
			Content:   roadmap.Title,
			CreatedAt: now,
		}
		if err := s.db.Create(&roadmapLog).Error; err != nil {
			return nil, err
		}
	}

	return &dto.ToggleRoadmapStepResponse{
		RoadmapID:       roadmapID,
		StepID:          stepID,
		Completed:       completed,
		CompletedAt:     progress.CompletedAt,
		CompletedSteps:  int(completedSteps),
		TotalSteps:      totalSteps,
		ProgressPercent: progressPercent,
		RoadmapStatus:   string(follow.Status),
	}, nil
}

func (s *LearningRoadmapService) CompleteStepForQuest(userID uuid.UUID, roadmapID uuid.UUID, stepID uuid.UUID) error {
	var roadmap models.LearningRoadmap
	if err := s.db.Where("id = ? AND enabled = ? AND (created_by_user_id IS NULL OR created_by_user_id = ?)", roadmapID, true, userID).First(&roadmap).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoadmapNotFound
		}
		return err
	}

	var step models.LearningRoadmapStep
	if err := s.db.Where("id = ? AND roadmap_id = ? AND enabled = ?", stepID, roadmapID, true).First(&step).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStepNotFound
		}
		return err
	}

	var follow models.UserLearningRoadmap
	if err := s.db.Where("user_id = ? AND roadmap_id = ?", userID, roadmapID).First(&follow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFollowingRoadmap
		}
		return err
	}

	now := timeutil.NowUTC()

	var progress models.UserLearningRoadmapStepProgress
	err := s.db.Where("user_id = ? AND step_id = ?", userID, stepID).First(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		progress = models.UserLearningRoadmapStepProgress{
			UserID:      userID,
			RoadmapID:   roadmapID,
			StepID:      stepID,
			Completed:   true,
			CompletedAt: &now,
		}
		if err := s.db.Create(&progress).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !progress.Completed {
		progress.Completed = true
		progress.CompletedAt = &now
		progress.UpdatedAt = now
		if err := s.db.Save(&progress).Error; err != nil {
			return err
		}
	}

	var completedSteps int64
	s.db.Model(&models.UserLearningRoadmapStepProgress{}).
		Where("user_id = ? AND roadmap_id = ? AND completed = ?", userID, roadmapID, true).
		Count(&completedSteps)

	totalSteps := roadmap.TotalSteps

	if int(completedSteps) == totalSteps && follow.Status != models.UserLearningRoadmapStatusCompleted {
		follow.Status = models.UserLearningRoadmapStatusCompleted
		if follow.CompletedAt == nil {
			follow.CompletedAt = &now
		}
		follow.UpdatedAt = now
		s.db.Save(&follow)

		roadmapLog := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCompleted,
			Title:     "Hoàn thành lộ trình học",
			Content:   roadmap.Title,
			CreatedAt: now,
		}
		_ = s.db.Create(&roadmapLog)
	}

	stepLog := models.LogEntry{
		UserID:    userID,
		Type:      models.LogEntryTypeLearningRoadmapStepCompleted,
		Title:     "Hoàn thành bước học",
		Content:   step.Title,
		CreatedAt: now,
	}
	_ = s.db.Create(&stepLog)

	return nil
}

// Curated suggestion templates - AI adapter placeholder
type suggestionTemplate struct {
	ID               string
	Title            string
	Description      string
	Category         string
	Difficulty       string
	EstimatedMinutes int
	TotalSteps       int
	Reason           string
	Steps            []suggestionTemplateStep
}

type suggestionTemplateStep struct {
	Title            string
	Description      string
	OrderIndex       int
	EstimatedMinutes int
}

var curatedSuggestions = []suggestionTemplate{
	{
		ID:               "ai_flutter_performance_optimization",
		Title:            "Flutter Performance Optimization",
		Description:      "Tối ưu hiệu suất ứng dụng Flutter cho production",
		Category:         "Flutter",
		Difficulty:       "advanced",
		EstimatedMinutes: 240,
		TotalSteps:       8,
		Reason:           "Phù hợp nếu bạn đã có nền tảng Flutter và muốn tối ưu hiệu suất.",
		Steps: []suggestionTemplateStep{
			{Title: "Hiểu widget lifecycle", Description: "Nắm vững lifecycle của StatefulWidget và StatelessWidget", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Tối ưu rebuild với const và keys", Description: "Sử dụng const constructor và widget keys đúng cách", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Phân tích performance với Flutter DevTools", Description: "Sử dụng DevTools để phát hiện jank và rebuild không cần thiết", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Tối ưu list/grid lớn", Description: "ListView.builder, GridView.builder và kỹ thuật lazy loading", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Quản lý memory và image cache", Description: "Tối ưu cache hình ảnh và quản lý memory hiệu quả", OrderIndex: 4, EstimatedMinutes: 30},
			{Title: "Tối ưu animation", Description: "Sử dụng AnimatedBuilder, RepaintBoundary đúng cách", OrderIndex: 5, EstimatedMinutes: 30},
			{Title: "Theo dõi jank và frame time", Description: "Monitor performance metrics trong production", OrderIndex: 6, EstimatedMinutes: 30},
			{Title: "Checklist tối ưu trước khi release", Description: "Review toàn bộ app với performance checklist", OrderIndex: 7, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "ai_riverpod_state_management",
		Title:            "State Management với Riverpod",
		Description:      "Quản lý state hiệu quả cho ứng dụng Flutter với Riverpod",
		Category:         "Flutter",
		Difficulty:       "intermediate",
		EstimatedMinutes: 180,
		TotalSteps:       6,
		Reason:           "Riverpod phù hợp để quản lý state rõ ràng cho app Flutter thực tế.",
		Steps: []suggestionTemplateStep{
			{Title: "Ôn lại Provider và StateProvider", Description: "Hiểu các loại provider cơ bản trong Riverpod", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Tìm hiểu FutureProvider và StreamProvider", Description: "Xử lý async data với Riverpod", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Tách state theo feature", Description: "Tổ chức state logic theo từng feature module", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Dependency injection với Riverpod", Description: "Quản lý dependencies và services", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Error/loading state chuẩn", Description: "Xử lý loading, error, và empty state", OrderIndex: 4, EstimatedMinutes: 30},
			{Title: "Refactor một màn hình thật", Description: "Áp dụng Riverpod vào một feature thực tế", OrderIndex: 5, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "ai_flutter_testing",
		Title:            "Testing Flutter Apps",
		Description:      "Viết test để tự tin refactor và phát triển tính năng mới",
		Category:         "Testing",
		Difficulty:       "intermediate",
		EstimatedMinutes: 200,
		TotalSteps:       7,
		Reason:           "Giúp bạn tự tin hơn khi sửa UI/API mà không làm vỡ flow cũ.",
		Steps: []suggestionTemplateStep{
			{Title: "Unit test cho model/dto", Description: "Test business logic và data models", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Service test với fake API", Description: "Mock API calls và test service layer", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Widget test cơ bản", Description: "Test UI components và interactions", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Test form validation", Description: "Test form input và validation logic", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Test navigation", Description: "Test navigation flow giữa các màn hình", OrderIndex: 4, EstimatedMinutes: 20},
			{Title: "Golden/screenshot test nếu cần", Description: "Snapshot testing cho UI regression", OrderIndex: 5, EstimatedMinutes: 30},
			{Title: "Viết checklist regression", Description: "Tạo test checklist cho các tính năng quan trọng", OrderIndex: 6, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "ai_flutter_animation",
		Title:            "Flutter Animation Deep Dive",
		Description:      "Làm chủ animation để tạo UX mượt mà và ấn tượng",
		Category:         "Flutter",
		Difficulty:       "intermediate",
		EstimatedMinutes: 150,
		TotalSteps:       5,
		Reason:           "Phù hợp để polish UI và tăng cảm giác mượt cho app.",
		Steps: []suggestionTemplateStep{
			{Title: "AnimationController cơ bản", Description: "Hiểu AnimationController, Tween, và Curve", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Implicit animations", Description: "Sử dụng AnimatedContainer, AnimatedOpacity, etc", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "AnimatedSwitcher và Hero", Description: "Transition giữa widgets và màn hình", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Bottom sheet/page transition", Description: "Custom transitions cho navigation", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Polish một component thật", Description: "Áp dụng animation vào một tính năng thực tế", OrderIndex: 4, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "ai_dart_async",
		Title:            "Dart Async Programming",
		Description:      "Nắm vững async/await và xử lý bất đồng bộ trong Dart",
		Category:         "Dart",
		Difficulty:       "beginner",
		EstimatedMinutes: 120,
		TotalSteps:       6,
		Reason:           "Nền tảng async tốt giúp xử lý API, loading và error state chính xác hơn.",
		Steps: []suggestionTemplateStep{
			{Title: "Future và async/await", Description: "Hiểu cơ bản về asynchronous programming", OrderIndex: 0, EstimatedMinutes: 20},
			{Title: "Try/catch trong async", Description: "Xử lý errors trong async functions", OrderIndex: 1, EstimatedMinutes: 20},
			{Title: "Future.wait", Description: "Chạy nhiều async tasks song song", OrderIndex: 2, EstimatedMinutes: 20},
			{Title: "Stream cơ bản", Description: "Làm việc với data streams", OrderIndex: 3, EstimatedMinutes: 20},
			{Title: "Cancel/retry pattern", Description: "Hủy request và retry logic", OrderIndex: 4, EstimatedMinutes: 20},
			{Title: "Áp dụng vào API service", Description: "Tích hợp async vào real API layer", OrderIndex: 5, EstimatedMinutes: 20},
		},
	},
	{
		ID:               "ai_uiux_fundamentals",
		Title:            "UI/UX Design Fundamentals",
		Description:      "Nguyên tắc thiết kế cơ bản để tạo giao diện dễ dùng",
		Category:         "Design",
		Difficulty:       "beginner",
		EstimatedMinutes: 180,
		TotalSteps:       7,
		Reason:           "Giúp bạn review flow sản phẩm và tránh UI gây hiểu nhầm.",
		Steps: []suggestionTemplateStep{
			{Title: "Visual hierarchy", Description: "Tạo thứ bậc thông tin rõ ràng", OrderIndex: 0, EstimatedMinutes: 25},
			{Title: "Spacing và layout", Description: "Sử dụng khoảng cách và grid system", OrderIndex: 1, EstimatedMinutes: 25},
			{Title: "Empty/loading/error state", Description: "Thiết kế các trạng thái đặc biệt", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "CTA rõ ràng", Description: "Thiết kế call-to-action buttons hiệu quả", OrderIndex: 3, EstimatedMinutes: 25},
			{Title: "Accessibility cơ bản", Description: "Đảm bảo app dễ tiếp cận cho mọi người", OrderIndex: 4, EstimatedMinutes: 25},
			{Title: "UX writing", Description: "Viết text hướng dẫn người dùng", OrderIndex: 5, EstimatedMinutes: 25},
			{Title: "Review một flow thật", Description: "Áp dụng nguyên tắc UX vào một feature", OrderIndex: 6, EstimatedMinutes: 25},
		},
	},
}

func (s *LearningRoadmapService) GetAISuggestions(userID uuid.UUID, req dto.AiRoadmapSuggestRequest) (*dto.AiRoadmapSuggestResponse, error) {
	// Validate and set defaults
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 5 {
		limit = 5
	}

	// Filter suggestions based on preferences
	filtered := []suggestionTemplate{}

	for _, suggestion := range curatedSuggestions {
		matches := true

		// Filter by category (single)
		if req.Preferences.Category != "" {
			if !matchesCategory(suggestion.Category, req.Preferences.Category) {
				matches = false
			}
		}

		// Filter by categories (array)
		if len(req.Preferences.Categories) > 0 {
			categoryMatch := false
			for _, cat := range req.Preferences.Categories {
				if matchesCategory(suggestion.Category, cat) {
					categoryMatch = true
					break
				}
			}
			if !categoryMatch {
				matches = false
			}
		}

		// Filter by difficulty
		if req.Preferences.Difficulty != "" && req.Preferences.Difficulty != "any" {
			if !matchesDifficulty(suggestion.Difficulty, req.Preferences.Difficulty) {
				matches = false
			}
		}

		// Filter by max duration
		if req.Preferences.MaxDuration > 0 {
			if suggestion.EstimatedMinutes > req.Preferences.MaxDuration {
				matches = false
			}
		}

		if matches {
			filtered = append(filtered, suggestion)
		}
	}

	// If no matches, return top featured suggestions
	if len(filtered) == 0 {
		filtered = []suggestionTemplate{
			curatedSuggestions[1], // Riverpod
			curatedSuggestions[4], // Dart Async
			curatedSuggestions[5], // UI/UX
		}
	}

	// Rank by learning goal relevance if provided
	if req.Preferences.LearningGoal != "" {
		rankSuggestionsByGoal(filtered, req.Preferences.LearningGoal)
	}

	// Limit results
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	// Convert to response DTOs
	suggestions := make([]dto.AiRoadmapSuggestionItem, 0, len(filtered))
	for _, s := range filtered {
		suggestions = append(suggestions, dto.AiRoadmapSuggestionItem{
			ID:               s.ID,
			Title:            s.Title,
			Description:      s.Description,
			Category:         s.Category,
			Difficulty:       s.Difficulty,
			EstimatedMinutes: s.EstimatedMinutes,
			TotalSteps:       s.TotalSteps,
			Reason:           s.Reason,
		})
	}

	return &dto.AiRoadmapSuggestResponse{
		Suggestions: suggestions,
		GeneratedAt: timeutil.NowUTC(),
	}, nil
}

func matchesCategory(suggestionCat, filterCat string) bool {
	return strings.EqualFold(strings.TrimSpace(suggestionCat), strings.TrimSpace(filterCat))
}

func matchesDifficulty(suggestionDiff, filterDiff string) bool {
	// Normalize "normal" to "intermediate"
	if strings.EqualFold(filterDiff, "normal") {
		filterDiff = "intermediate"
	}
	if strings.EqualFold(suggestionDiff, "normal") {
		suggestionDiff = "intermediate"
	}
	return strings.EqualFold(suggestionDiff, filterDiff)
}

func rankSuggestionsByGoal(suggestions []suggestionTemplate, goal string) {
	// Simple keyword-based ranking
	// In a real AI system, this would be semantic matching
	goalLower := strings.ToLower(goal)

	// Score each suggestion
	type scoredSuggestion struct {
		suggestion suggestionTemplate
		score      int
	}
	scoredList := make([]scoredSuggestion, len(suggestions))

	for i, s := range suggestions {
		score := 0
		titleLower := strings.ToLower(s.Title)
		descLower := strings.ToLower(s.Description)
		catLower := strings.ToLower(s.Category)

		// Check for keyword matches
		keywords := strings.Fields(goalLower)
		for _, kw := range keywords {
			if len(kw) < 3 {
				continue
			}
			if strings.Contains(titleLower, kw) {
				score += 3
			}
			if strings.Contains(descLower, kw) {
				score += 2
			}
			if strings.Contains(catLower, kw) {
				score += 1
			}
		}

		scoredList[i] = scoredSuggestion{suggestion: s, score: score}
	}

	// Sort by score descending
	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[j].score > scoredList[i].score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}

	// Update original slice
	for i, s := range scoredList {
		suggestions[i] = s.suggestion
	}
}

func (s *LearningRoadmapService) CreateRoadmap(userID uuid.UUID, req dto.CreateLearningRoadmapRequest) (*dto.LearningRoadmapItem, error) {
	// Validate source
	if req.Source != "ai" && req.Source != "user" {
		return nil, ErrInvalidCreateRequest
	}

	// Two creation paths: from suggestion or custom
	if req.SuggestionID != nil && *req.SuggestionID != "" {
		return s.createFromSuggestion(userID, *req.SuggestionID, req.Source, req.Customize)
	}

	return s.createCustomRoadmap(userID, req)
}

func (s *LearningRoadmapService) createFromSuggestion(userID uuid.UUID, suggestionID string, source string, customize *dto.CreateLearningRoadmapCustomize) (*dto.LearningRoadmapItem, error) {
	// Find the suggestion template
	var template *suggestionTemplate
	for i := range curatedSuggestions {
		if curatedSuggestions[i].ID == suggestionID {
			template = &curatedSuggestions[i]
			break
		}
	}

	if template == nil {
		return nil, ErrSuggestionNotFound
	}

	// Apply customization or use template values
	title := template.Title
	description := template.Description
	category := template.Category
	difficulty := template.Difficulty

	if customize != nil {
		if customize.Title != nil && *customize.Title != "" {
			title = *customize.Title
		}
		if customize.Description != nil {
			description = *customize.Description
		}
		if customize.Category != nil && *customize.Category != "" {
			category = *customize.Category
		}
		if customize.Difficulty != nil && *customize.Difficulty != "" {
			difficulty = normalizeDifficulty(*customize.Difficulty)
		}
	}

	// Create roadmap in transaction
	var roadmap models.LearningRoadmap
	var steps []models.LearningRoadmapStep
	var userRoadmap models.UserLearningRoadmap

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create roadmap
		roadmap = models.LearningRoadmap{
			Title:            title,
			Description:      description,
			Category:         category,
			Difficulty:       difficulty,
			EstimatedMinutes: template.EstimatedMinutes,
			TotalSteps:       template.TotalSteps,
			Source:           models.LearningRoadmapSource(source),
			CreatedByUserID:  &userID,
			Enabled:          true,
		}

		if err := tx.Create(&roadmap).Error; err != nil {
			return err
		}

		// Clone steps from template
		steps = make([]models.LearningRoadmapStep, 0, len(template.Steps))
		for _, ts := range template.Steps {
			step := models.LearningRoadmapStep{
				RoadmapID:        roadmap.ID,
				Title:            ts.Title,
				Description:      ts.Description,
				OrderIndex:       ts.OrderIndex,
				EstimatedMinutes: ts.EstimatedMinutes,
				Enabled:          true,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
			steps = append(steps, step)
		}

		// Create user tracking record
		now := timeutil.NowUTC()
		userRoadmap = models.UserLearningRoadmap{
			UserID:    userID,
			RoadmapID: roadmap.ID,
			Status:    models.UserLearningRoadmapStatusTracking,
			StartedAt: now,
		}

		if err := tx.Create(&userRoadmap).Error; err != nil {
			return err
		}

		// Create log entry
		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCreated,
			Title:     "Tạo lộ trình học",
			Content:   title,
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build response
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
		Status:           string(userRoadmap.Status),
		Enabled:          roadmap.Enabled,
		StartedAt:        &userRoadmap.StartedAt,
		CompletedAt:      nil,
		Steps:            stepItems,
	}, nil
}

func (s *LearningRoadmapService) createCustomRoadmap(userID uuid.UUID, req dto.CreateLearningRoadmapRequest) (*dto.LearningRoadmapItem, error) {
	// Validate required fields for custom roadmap
	if req.Title == nil || *req.Title == "" {
		return nil, ErrInvalidCreateRequest
	}
	if req.Category == nil || *req.Category == "" {
		return nil, ErrInvalidCreateRequest
	}
	if req.Difficulty == nil || *req.Difficulty == "" {
		return nil, ErrInvalidCreateRequest
	}
	if len(req.Steps) == 0 {
		return nil, ErrInvalidCreateRequest
	}

	// Validate steps
	for _, step := range req.Steps {
		if step.Title == "" {
			return nil, ErrInvalidCreateRequest
		}
		if step.EstimatedMinutes < 0 {
			return nil, ErrInvalidCreateRequest
		}
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	difficulty := normalizeDifficulty(*req.Difficulty)

	// Calculate totals
	totalSteps := len(req.Steps)
	estimatedMinutes := 0
	for _, step := range req.Steps {
		estimatedMinutes += step.EstimatedMinutes
	}

	// Create roadmap in transaction
	var roadmap models.LearningRoadmap
	var steps []models.LearningRoadmapStep
	var userRoadmap models.UserLearningRoadmap

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create roadmap
		roadmap = models.LearningRoadmap{
			Title:            *req.Title,
			Description:      description,
			Category:         *req.Category,
			Difficulty:       difficulty,
			EstimatedMinutes: estimatedMinutes,
			TotalSteps:       totalSteps,
			Source:           models.LearningRoadmapSource(req.Source),
			CreatedByUserID:  &userID,
			Enabled:          true,
		}

		if err := tx.Create(&roadmap).Error; err != nil {
			return err
		}

		// Create steps
		steps = make([]models.LearningRoadmapStep, 0, len(req.Steps))
		for _, stepReq := range req.Steps {
			step := models.LearningRoadmapStep{
				RoadmapID:        roadmap.ID,
				Title:            stepReq.Title,
				Description:      stepReq.Description,
				OrderIndex:       stepReq.OrderIndex,
				EstimatedMinutes: stepReq.EstimatedMinutes,
				Enabled:          true,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
			steps = append(steps, step)
		}

		// Create user tracking record
		now := timeutil.NowUTC()
		userRoadmap = models.UserLearningRoadmap{
			UserID:    userID,
			RoadmapID: roadmap.ID,
			Status:    models.UserLearningRoadmapStatusTracking,
			StartedAt: now,
		}

		if err := tx.Create(&userRoadmap).Error; err != nil {
			return err
		}

		// Create log entry
		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCreated,
			Title:     "Tạo lộ trình học",
			Content:   *req.Title,
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build response
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
		Status:           string(userRoadmap.Status),
		Enabled:          roadmap.Enabled,
		StartedAt:        &userRoadmap.StartedAt,
		CompletedAt:      nil,
		Steps:            stepItems,
	}, nil
}

func normalizeDifficulty(difficulty string) string {
	lower := strings.ToLower(strings.TrimSpace(difficulty))
	if lower == "normal" {
		return "intermediate"
	}
	if lower == "beginner" || lower == "intermediate" || lower == "advanced" {
		return lower
	}
	return "intermediate"
}

// Template-based suggestion system (replaces AI adapter)
var templateRoadmaps = []suggestionTemplate{
	{
		ID:               "template_flutter_performance",
		Title:            "Flutter Performance Optimization",
		Description:      "Tối ưu hiệu suất ứng dụng Flutter cho production",
		Category:         "Flutter",
		Difficulty:       "advanced",
		EstimatedMinutes: 240,
		TotalSteps:       8,
		Reason:           "Phù hợp nếu bạn đã có nền tảng Flutter và muốn tối ưu hiệu suất.",
		Steps: []suggestionTemplateStep{
			{Title: "Hiểu widget lifecycle", Description: "Nắm vững lifecycle của StatefulWidget và StatelessWidget", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Tối ưu rebuild với const và keys", Description: "Sử dụng const constructor và widget keys đúng cách", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Phân tích performance với Flutter DevTools", Description: "Sử dụng DevTools để phát hiện jank và rebuild không cần thiết", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Tối ưu list/grid lớn", Description: "ListView.builder, GridView.builder và kỹ thuật lazy loading", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Quản lý memory và image cache", Description: "Tối ưu cache hình ảnh và quản lý memory hiệu quả", OrderIndex: 4, EstimatedMinutes: 30},
			{Title: "Tối ưu animation", Description: "Sử dụng AnimatedBuilder, RepaintBoundary đúng cách", OrderIndex: 5, EstimatedMinutes: 30},
			{Title: "Theo dõi jank và frame time", Description: "Monitor performance metrics trong production", OrderIndex: 6, EstimatedMinutes: 30},
			{Title: "Checklist tối ưu trước khi release", Description: "Review toàn bộ app với performance checklist", OrderIndex: 7, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "template_riverpod",
		Title:            "State Management với Riverpod",
		Description:      "Quản lý state hiệu quả cho ứng dụng Flutter với Riverpod",
		Category:         "Flutter",
		Difficulty:       "intermediate",
		EstimatedMinutes: 180,
		TotalSteps:       6,
		Reason:           "Riverpod phù hợp để quản lý state rõ ràng cho app Flutter thực tế.",
		Steps: []suggestionTemplateStep{
			{Title: "Ôn lại Provider và StateProvider", Description: "Hiểu các loại provider cơ bản trong Riverpod", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Tìm hiểu FutureProvider và StreamProvider", Description: "Xử lý async data với Riverpod", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Tách state theo feature", Description: "Tổ chức state logic theo từng feature module", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Dependency injection với Riverpod", Description: "Quản lý dependencies và services", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Error/loading state chuẩn", Description: "Xử lý loading, error, và empty state", OrderIndex: 4, EstimatedMinutes: 30},
			{Title: "Refactor một màn hình thật", Description: "Áp dụng Riverpod vào một feature thực tế", OrderIndex: 5, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "template_testing",
		Title:            "Testing Flutter Apps",
		Description:      "Viết test để tự tin refactor và phát triển tính năng mới",
		Category:         "Testing",
		Difficulty:       "intermediate",
		EstimatedMinutes: 200,
		TotalSteps:       7,
		Reason:           "Giúp bạn tự tin hơn khi sửa UI/API mà không làm vỡ flow cũ.",
		Steps: []suggestionTemplateStep{
			{Title: "Unit test cho model/dto", Description: "Test business logic và data models", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Service test với fake API", Description: "Mock API calls và test service layer", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "Widget test cơ bản", Description: "Test UI components và interactions", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Test form validation", Description: "Test form input và validation logic", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Test navigation", Description: "Test navigation flow giữa các màn hình", OrderIndex: 4, EstimatedMinutes: 20},
			{Title: "Golden/screenshot test nếu cần", Description: "Snapshot testing cho UI regression", OrderIndex: 5, EstimatedMinutes: 30},
			{Title: "Viết checklist regression", Description: "Tạo test checklist cho các tính năng quan trọng", OrderIndex: 6, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "template_animation",
		Title:            "Flutter Animation Deep Dive",
		Description:      "Làm chủ animation để tạo UX mượt mà và ấn tượng",
		Category:         "Flutter",
		Difficulty:       "intermediate",
		EstimatedMinutes: 150,
		TotalSteps:       5,
		Reason:           "Phù hợp để polish UI và tăng cảm giác mượt cho app.",
		Steps: []suggestionTemplateStep{
			{Title: "AnimationController cơ bản", Description: "Hiểu AnimationController, Tween, và Curve", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Implicit animations", Description: "Sử dụng AnimatedContainer, AnimatedOpacity, etc", OrderIndex: 1, EstimatedMinutes: 30},
			{Title: "AnimatedSwitcher và Hero", Description: "Transition giữa widgets và màn hình", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "Bottom sheet/page transition", Description: "Custom transitions cho navigation", OrderIndex: 3, EstimatedMinutes: 30},
			{Title: "Polish một component thật", Description: "Áp dụng animation vào một tính năng thực tế", OrderIndex: 4, EstimatedMinutes: 30},
		},
	},
	{
		ID:               "template_dart_async",
		Title:            "Dart Async Programming",
		Description:      "Nắm vững async/await và xử lý bất đồng bộ trong Dart",
		Category:         "Dart",
		Difficulty:       "beginner",
		EstimatedMinutes: 120,
		TotalSteps:       6,
		Reason:           "Nền tảng async tốt giúp xử lý API, loading và error state chính xác hơn.",
		Steps: []suggestionTemplateStep{
			{Title: "Future và async/await", Description: "Hiểu cơ bản về asynchronous programming", OrderIndex: 0, EstimatedMinutes: 20},
			{Title: "Try/catch trong async", Description: "Xử lý errors trong async functions", OrderIndex: 1, EstimatedMinutes: 20},
			{Title: "Future.wait", Description: "Chạy nhiều async tasks song song", OrderIndex: 2, EstimatedMinutes: 20},
			{Title: "Stream cơ bản", Description: "Làm việc với data streams", OrderIndex: 3, EstimatedMinutes: 20},
			{Title: "Cancel/retry pattern", Description: "Hủy request và retry logic", OrderIndex: 4, EstimatedMinutes: 20},
			{Title: "Áp dụng vào API service", Description: "Tích hợp async vào real API layer", OrderIndex: 5, EstimatedMinutes: 20},
		},
	},
	{
		ID:               "template_uiux",
		Title:            "UI/UX Design Fundamentals",
		Description:      "Nguyên tắc thiết kế cơ bản để tạo giao diện dễ dùng",
		Category:         "Design",
		Difficulty:       "beginner",
		EstimatedMinutes: 180,
		TotalSteps:       7,
		Reason:           "Giúp bạn review flow sản phẩm và tránh UI gây hiểu nhầm.",
		Steps: []suggestionTemplateStep{
			{Title: "Visual hierarchy", Description: "Tạo thứ bậc thông tin rõ ràng", OrderIndex: 0, EstimatedMinutes: 25},
			{Title: "Spacing và layout", Description: "Sử dụng khoảng cách và grid system", OrderIndex: 1, EstimatedMinutes: 25},
			{Title: "Empty/loading/error state", Description: "Thiết kế các trạng thái đặc biệt", OrderIndex: 2, EstimatedMinutes: 30},
			{Title: "CTA rõ ràng", Description: "Thiết kế call-to-action buttons hiệu quả", OrderIndex: 3, EstimatedMinutes: 25},
			{Title: "Accessibility cơ bản", Description: "Đảm bảo app dễ tiếp cận cho mọi người", OrderIndex: 4, EstimatedMinutes: 25},
			{Title: "UX writing", Description: "Viết text hướng dẫn người dùng", OrderIndex: 5, EstimatedMinutes: 25},
			{Title: "Review một flow thật", Description: "Áp dụng nguyên tắc UX vào một feature", OrderIndex: 6, EstimatedMinutes: 25},
		},
	},
}

func (s *LearningRoadmapService) GetTemplateSuggestions(userID uuid.UUID, req dto.TemplateSuggestRequest) (*dto.TemplateSuggestResponse, error) {
	// Filter templates based on preferences
	filtered := []suggestionTemplate{}

	for _, tmpl := range templateRoadmaps {
		// Filter by category if provided
		if req.Preferences.Category != "" {
			if !strings.EqualFold(tmpl.Category, req.Preferences.Category) {
				continue
			}
		}

		// Filter by difficulty if provided
		if req.Preferences.Difficulty != "" {
			if !strings.EqualFold(tmpl.Difficulty, req.Preferences.Difficulty) {
				continue
			}
		}

		// Filter by max_duration if provided
		if req.Preferences.MaxDuration > 0 {
			if tmpl.EstimatedMinutes > req.Preferences.MaxDuration {
				continue
			}
		}

		// Soft match by learning_goal keywords in title/description/category
		if req.Preferences.LearningGoal != "" {
			goalLower := strings.ToLower(req.Preferences.LearningGoal)
			titleLower := strings.ToLower(tmpl.Title)
			descLower := strings.ToLower(tmpl.Description)
			catLower := strings.ToLower(tmpl.Category)

			if !strings.Contains(titleLower, goalLower) &&
				!strings.Contains(descLower, goalLower) &&
				!strings.Contains(catLower, goalLower) {
				// Check for partial keyword matches
				keywords := strings.Fields(goalLower)
				matched := false
				for _, kw := range keywords {
					if len(kw) > 3 { // Only check keywords longer than 3 chars
						if strings.Contains(titleLower, kw) ||
							strings.Contains(descLower, kw) ||
							strings.Contains(catLower, kw) {
							matched = true
							break
						}
					}
				}
				if !matched && len(keywords) > 0 {
					continue
				}
			}
		}

		filtered = append(filtered, tmpl)
	}

	// If no exact matches, return reasonable fallback templates
	if len(filtered) == 0 {
		// Return beginner-friendly templates as fallback
		for _, tmpl := range templateRoadmaps {
			if tmpl.Difficulty == "beginner" || tmpl.Difficulty == "intermediate" {
				filtered = append(filtered, tmpl)
			}
		}
	}

	// Convert to response format
	suggestions := make([]dto.TemplateSuggestionItem, 0, len(filtered))
	for _, tmpl := range filtered {
		suggestions = append(suggestions, dto.TemplateSuggestionItem{
			ID:               tmpl.ID,
			Title:            tmpl.Title,
			Description:      tmpl.Description,
			Category:         tmpl.Category,
			Difficulty:       tmpl.Difficulty,
			EstimatedMinutes: tmpl.EstimatedMinutes,
			TotalSteps:       tmpl.TotalSteps,
			Source:           "template",
		})
	}

	return &dto.TemplateSuggestResponse{
		Suggestions: suggestions,
	}, nil
}

func (s *LearningRoadmapService) CreateFromTemplate(userID uuid.UUID, req dto.CreateFromTemplateRequest) (*dto.LearningRoadmapItem, error) {
	// Validate source
	if req.Source != "template" {
		return nil, errors.New("invalid source for template creation")
	}

	// Find template
	var template *suggestionTemplate
	for _, tmpl := range templateRoadmaps {
		if tmpl.ID == req.TemplateID {
			template = &tmpl
			break
		}
	}

	if template == nil {
		return nil, errors.New("template not found")
	}

	// Check if user already created from this template
	var existing models.LearningRoadmap
	err := s.db.Where("created_by_user_id = ? AND source = ?", userID, models.LearningRoadmapSourceTemplate).
		Joins("JOIN learning_roadmap_steps ON learning_roadmap_steps.roadmap_id = learning_roadmaps.id").
		Where("learning_roadmaps.title = ?", template.Title).
		First(&existing).Error

	if err == nil {
		// User already created this template, check if they're tracking it
		var userRoadmap models.UserLearningRoadmap
		err := s.db.Where("user_id = ? AND roadmap_id = ?", userID, existing.ID).First(&userRoadmap).Error
		if err == nil {
			// Return existing roadmap
			return s.GetRoadmapDetail(userID, existing.ID)
		}
	}

	// Create roadmap in transaction
	var roadmap models.LearningRoadmap
	var steps []models.LearningRoadmapStep
	var userRoadmap models.UserLearningRoadmap

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Create roadmap
		roadmap = models.LearningRoadmap{
			Title:            template.Title,
			Description:      template.Description,
			Category:         template.Category,
			Difficulty:       template.Difficulty,
			EstimatedMinutes: template.EstimatedMinutes,
			TotalSteps:       template.TotalSteps,
			Source:           models.LearningRoadmapSourceTemplate,
			CreatedByUserID:  &userID,
			Enabled:          true,
		}

		if err := tx.Create(&roadmap).Error; err != nil {
			return err
		}

		// Clone steps from template
		steps = make([]models.LearningRoadmapStep, 0, len(template.Steps))
		for _, stepTmpl := range template.Steps {
			step := models.LearningRoadmapStep{
				RoadmapID:        roadmap.ID,
				Title:            stepTmpl.Title,
				Description:      stepTmpl.Description,
				OrderIndex:       stepTmpl.OrderIndex,
				EstimatedMinutes: stepTmpl.EstimatedMinutes,
				Enabled:          true,
			}
			if err := tx.Create(&step).Error; err != nil {
				return err
			}
			steps = append(steps, step)
		}

		// Create user tracking record
		now := timeutil.NowUTC()
		userRoadmap = models.UserLearningRoadmap{
			UserID:    userID,
			RoadmapID: roadmap.ID,
			Status:    models.UserLearningRoadmapStatusTracking,
			StartedAt: now,
		}

		if err := tx.Create(&userRoadmap).Error; err != nil {
			return err
		}

		// Create log entry
		logEntry := models.LogEntry{
			UserID:    userID,
			Type:      models.LogEntryTypeLearningRoadmapCreated,
			Title:     "Tạo lộ trình học",
			Content:   template.Title,
			CreatedAt: now,
		}
		if err := tx.Create(&logEntry).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build response
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
		Status:           string(userRoadmap.Status),
		Enabled:          roadmap.Enabled,
		StartedAt:        &userRoadmap.StartedAt,
		CompletedAt:      nil,
		Steps:            stepItems,
	}, nil
}
