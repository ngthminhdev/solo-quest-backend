package services

import (
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

var devUserUUID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type BootstrapService struct {
	db                     *gorm.DB
	devGenerator           *DevQuestGenerator
	reminderSettingService *ReminderSettingService
	questSettingsService   *QuestSettingsService
}

func NewBootstrapService(db *gorm.DB) *BootstrapService {
	return &BootstrapService{
		db:                     db,
		devGenerator:           NewDevQuestGenerator(db),
		reminderSettingService: NewReminderSettingService(db),
		questSettingsService:   NewQuestSettingsService(db),
	}
}

func (s *BootstrapService) GetDevUserID() uuid.UUID {
	return devUserUUID
}

func (s *BootstrapService) BootstrapDefaultDevUser(devUserEmail string) error {
	logger.L.Info("bootstrapping default dev user")

	user, err := s.ensureDevUser()
	if err != nil {
		return err
	}

	if err := s.ensureDevAuthAccount(user.ID, devUserEmail); err != nil {
		return err
	}

	if err := s.ensureAppSettings(user.ID); err != nil {
		return err
	}

	if err := s.ensureQuestSettings(user.ID); err != nil {
		return err
	}

	if err := s.ensureRewards(user.ID); err != nil {
		return err
	}

	if err := s.ensureDevDailyQuests(user.ID); err != nil {
		return err
	}

	if err := s.ensureReminderSettings(user.ID); err != nil {
		return err
	}

	if err := s.ensureLearningRoadmaps(); err != nil {
		return err
	}

	if err := s.ensureStartupLogs(user.ID); err != nil {
		return err
	}

	logger.L.Info("default dev user bootstrap completed")
	return nil
}

func (s *BootstrapService) ensureDevUser() (*models.UserProfile, error) {
	var user models.UserProfile
	err := s.db.Where("id = ?", devUserUUID).First(&user).Error
	if err == nil {
		logger.L.Info("dev user already exists")
		return &user, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	quietTime := "22:00"
	user = models.UserProfile{
		ID:                     devUserUUID,
		DisplayName:            "Minh Thanh",
		Level:                  1,
		CurrentLevelExp:        0,
		NextLevelExp:           100,
		TotalExp:               0,
		RewardPoints:           100,
		StreakDays:             0,
		BestStreak:             0,
		StreakShields:          2,
		TotalCompletedQuests:   0,
		TotalSkippedQuests:     0,
		HasCompletedOnboarding: false,
		QuietAfterTime:         &quietTime,
	}

	if err := s.db.Create(&user).Error; err != nil {
		logger.L.Error("failed to create dev user", zap.Error(err))
		return nil, err
	}

	logger.L.Info("dev user created")
	return &user, nil
}

func (s *BootstrapService) ensureDevAuthAccount(userID uuid.UUID, email string) error {
	var count int64
	s.db.Model(&models.AuthAccount{}).
		Where("provider = ? AND provider_uid = ?", models.AuthProviderDev, "dev_minhthanh").
		Count(&count)

	if count > 0 {
		logger.L.Info("dev auth account already exists")
		return nil
	}

	authAccount := models.AuthAccount{
		UserID:      userID,
		Provider:    models.AuthProviderDev,
		ProviderUID: "dev_minhthanh",
		Email:       email,
	}

	if err := s.db.Create(&authAccount).Error; err != nil {
		logger.L.Error("failed to create dev auth account", zap.Error(err))
		return err
	}

	logger.L.Info("dev auth account created")
	return nil
}

func (s *BootstrapService) ensureAppSettings(userID uuid.UUID) error {
	var count int64
	s.db.Model(&models.AppSettings{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		logger.L.Info("app settings already exist")
		return nil
	}

	settings := models.AppSettings{
		UserID:               userID,
		Locale:               "vi",
		Theme:                "dark",
		DailyQuestLimit:      10,
		NotificationsEnabled: true,
		QuietAfterTime:       "22:00",
		Timezone:             "Asia/Ho_Chi_Minh",
	}

	if err := s.db.Create(&settings).Error; err != nil {
		logger.L.Error("failed to create app settings", zap.Error(err))
		return err
	}

	logger.L.Info("app settings created")
	return nil
}

func (s *BootstrapService) ensureRewards(userID uuid.UUID) error {
	var count int64
	s.db.Model(&models.Reward{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		logger.L.Info("rewards already exist")
		return nil
	}

	rewardData := []struct {
		Title      string
		Type       models.RewardType
		CostPoints int
		IconText   string
	}{
		{Title: "Nghỉ ngơi 30 phút", Type: models.RewardTypeRest, CostPoints: 30, IconText: "🛋️"},
		{Title: "Xem phim 1 tập", Type: models.RewardTypeEntertainment, CostPoints: 50, IconText: "🎬"},
		{Title: "Chơi game 45 phút", Type: models.RewardTypeEntertainment, CostPoints: 60, IconText: "🎮"},
		{Title: "Ngủ nướng thêm 1 giờ", Type: models.RewardTypeRest, CostPoints: 20, IconText: "😴"},
		{Title: "Ăn món yêu thích", Type: models.RewardTypeFood, CostPoints: 40, IconText: "🍜"},
		{Title: "Mạng xã hội 20 phút", Type: models.RewardTypeSocial, CostPoints: 25, IconText: "📱"},
		{Title: "Tự thưởng nhỏ", Type: models.RewardTypeCustom, CostPoints: 0, IconText: "🎁"},
	}

	for _, rd := range rewardData {
		reward := models.Reward{
			UserID:     userID,
			Title:      rd.Title,
			Type:       rd.Type,
			CostPoints: rd.CostPoints,
			IconText:   rd.IconText,
			Status:     models.RewardStatusAvailable,
		}

		if err := s.db.Create(&reward).Error; err != nil {
			logger.L.Error("failed to create reward", zap.String("title", rd.Title), zap.Error(err))
			return err
		}
	}

	logger.L.Info("rewards created", zap.Int("count", len(rewardData)))
	return nil
}

func (s *BootstrapService) ensureDevDailyQuests(userID uuid.UUID) error {
	today := timeutil.TodayVN()

	// Idempotent: skip if quests already exist for today.
	count, err := s.devGenerator.CountQuestsForDate(userID, today)
	if err != nil {
		return err
	}
	if count > 0 {
		logger.L.Info("dev daily quests already exist for today")
		return nil
	}

	_, err = s.devGenerator.GenerateDevDailyQuests(userID, today)
	if err != nil {
		return err
	}

	logger.L.Info("dev daily quests generated for today")
	return nil
}

func (s *BootstrapService) ensureReminderSettings(userID uuid.UUID) error {
	return s.reminderSettingService.EnsureDefaultReminderSettingsForUser(userID)
}

func (s *BootstrapService) ensureQuestSettings(userID uuid.UUID) error {
	_, err := s.questSettingsService.GetOrCreate(userID)
	if err != nil {
		logger.L.Error("failed to ensure quest settings", zap.Error(err))
		return err
	}
	logger.L.Info("quest settings ensured")
	return nil
}

func (s *BootstrapService) ensureLearningRoadmaps() error {
	type roadmapSeed struct {
		Title            string
		Description      string
		Category         string
		Difficulty       string
		EstimatedMinutes int
		Steps            []struct {
			Title            string
			Description      string
			OrderIndex       int
			EstimatedMinutes int
		}
	}

	seeds := []roadmapSeed{
		{
			Title:            "Flutter App Architecture",
			Description:      "Học kiến trúc ứng dụng Flutter từ cơ bản đến nâng cao",
			Category:         "flutter",
			Difficulty:       "normal",
			EstimatedMinutes: 180,
			Steps: []struct {
				Title            string
				Description      string
				OrderIndex       int
				EstimatedMinutes int
			}{
				{Title: "Tìm hiểu State Management", Description: "Học các pattern quản lý state phổ biến: Provider, Riverpod, Bloc", OrderIndex: 1, EstimatedMinutes: 30},
				{Title: "Dependency Injection", Description: "Học cách sử dụng GetIt, Riverpod, hoặc Provider để quản lý dependency", OrderIndex: 2, EstimatedMinutes: 25},
				{Title: "Clean Architecture", Description: "Áp dụng Clean Architecture với data/domain/presentation layers", OrderIndex: 3, EstimatedMinutes: 45},
				{Title: "Navigation & Routing", Description: "Học GoRouter hoặc auto_route để điều hướng trong ứng dụng", OrderIndex: 4, EstimatedMinutes: 30},
				{Title: "Testing Strategies", Description: "Unit test, widget test, integration test cho Flutter", OrderIndex: 5, EstimatedMinutes: 50},
			},
		},
		{
			Title:            "Dart Async Mastery",
			Description:      "Thành thạo lập trình bất đồng bộ trong Dart",
			Category:         "dart",
			Difficulty:       "normal",
			EstimatedMinutes: 120,
			Steps: []struct {
				Title            string
				Description      string
				OrderIndex       int
				EstimatedMinutes int
			}{
				{Title: "Future và async/await", Description: "Tìm hiểu Future, async/await và cách xử lý bất đồng bộ cơ bản", OrderIndex: 1, EstimatedMinutes: 30},
				{Title: "Stream cơ bản", Description: "Học Stream, StreamController và cách lắng nghe dữ liệu bất đồng bộ", OrderIndex: 2, EstimatedMinutes: 30},
				{Title: "Error handling trong async", Description: "Xử lý lỗi trong Future và Stream với try-catch và catchError", OrderIndex: 3, EstimatedMinutes: 30},
				{Title: "Thực hành async API call", Description: "Gọi REST API và xử lý response bất đồng bộ trong Flutter", OrderIndex: 4, EstimatedMinutes: 30},
			},
		},
		{
			Title:            "SoloQuest MVP",
			Description:      "Hoàn thành và kiểm tra SoloQuest MVP trước khi release",
			Category:         "product",
			Difficulty:       "normal",
			EstimatedMinutes: 150,
			Steps: []struct {
				Title            string
				Description      string
				OrderIndex       int
				EstimatedMinutes int
			}{
				{Title: "Review luồng daily quest", Description: "Kiểm tra toàn bộ luồng tạo, bắt đầu, hoàn thành, bỏ qua quest", OrderIndex: 1, EstimatedMinutes: 30},
				{Title: "Kiểm tra schedule và reminder", Description: "Verify schedule blocks và reminder settings hoạt động đúng", OrderIndex: 2, EstimatedMinutes: 30},
				{Title: "Polish UI lộ trình", Description: "Hoàn thiện giao diện Learning Roadmap trên Flutter", OrderIndex: 3, EstimatedMinutes: 30},
				{Title: "Test flow chính trên emulator", Description: "Chạy thử toàn bộ flow chính trên emulator/device thật", OrderIndex: 4, EstimatedMinutes: 30},
				{Title: "Ghi lại vấn đề cần sửa", Description: "Tạo danh sách bug và improvement cần xử lý trước release", OrderIndex: 5, EstimatedMinutes: 30},
			},
		},
	}

	for _, seed := range seeds {
		var count int64
		s.db.Model(&models.LearningRoadmap{}).Where("title = ? AND source = ?", seed.Title, "system").Count(&count)
		if count > 0 {
			continue
		}

		roadmap := models.LearningRoadmap{
			Title:            seed.Title,
			Description:      seed.Description,
			Category:         seed.Category,
			Difficulty:       seed.Difficulty,
			EstimatedMinutes: seed.EstimatedMinutes,
			TotalSteps:       len(seed.Steps),
			Source:           models.LearningRoadmapSourceSystem,
			Enabled:          true,
		}

		if err := s.db.Create(&roadmap).Error; err != nil {
			logger.L.Error("failed to create learning roadmap", zap.String("title", seed.Title), zap.Error(err))
			return err
		}

		for _, stepSeed := range seed.Steps {
			step := models.LearningRoadmapStep{
				RoadmapID:        roadmap.ID,
				Title:            stepSeed.Title,
				Description:      stepSeed.Description,
				OrderIndex:       stepSeed.OrderIndex,
				EstimatedMinutes: stepSeed.EstimatedMinutes,
				Enabled:          true,
			}
			if err := s.db.Create(&step).Error; err != nil {
				logger.L.Error("failed to create roadmap step", zap.String("title", stepSeed.Title), zap.Error(err))
				return err
			}
		}

		logger.L.Info("learning roadmap seeded", zap.String("title", seed.Title), zap.Int("steps", len(seed.Steps)))
	}

	return nil
}

func (s *BootstrapService) ensureStartupLogs(userID uuid.UUID) error {
	var count int64
	s.db.Model(&models.LogEntry{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		logger.L.Info("startup logs already exist")
		return nil
	}

	logData := []struct {
		Title   string
		Content string
	}{
		{Title: "Profile created", Content: "Dev user profile initialized for local development"},
		{Title: "Seed quest plan created", Content: "10 random dev daily quests generated for today"},
		{Title: "Rewards initialized", Content: "5 sample rewards created for dev user"},
	}

	for _, ld := range logData {
		logEntry := models.LogEntry{
			UserID:  userID,
			Type:    models.LogEntryTypeSystem,
			Title:   ld.Title,
			Content: ld.Content,
		}

		if err := s.db.Create(&logEntry).Error; err != nil {
			logger.L.Error("failed to create log entry", zap.String("title", ld.Title), zap.Error(err))
			return err
		}
	}

	logger.L.Info("startup logs created", zap.Int("count", len(logData)))
	return nil
}
