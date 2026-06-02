package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

var devUserUUID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type BootstrapService struct {
	db *gorm.DB
}

func NewBootstrapService(db *gorm.DB) *BootstrapService {
	return &BootstrapService{db: db}
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

	if err := s.ensureRewards(user.ID); err != nil {
		return err
	}

	if err := s.ensureSampleQuests(user.ID); err != nil {
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
		{Title: "Xem một tập phim", Type: models.RewardTypeEntertainment, CostPoints: 50, IconText: "🎬"},
		{Title: "Cà phê yêu thích", Type: models.RewardTypeFood, CostPoints: 40, IconText: "☕"},
		{Title: "Chơi game 30 phút", Type: models.RewardTypeEntertainment, CostPoints: 60, IconText: "🎮"},
		{Title: "Tự thưởng nhỏ", Type: models.RewardTypeCustom, CostPoints: 80, IconText: "🎁"},
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

func (s *BootstrapService) ensureSampleQuests(userID uuid.UUID) error {
	today := timeutil.TodayUTC()
	start, end := timeutil.DayRangeUTC(today)

	var count int64
	s.db.Model(&models.Quest{}).Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).Count(&count)
	if count > 0 {
		logger.L.Info("quests for today already exist")
		return nil
	}

	questData := []struct {
		Title            string
		Description      string
		Type             models.QuestType
		Difficulty       models.QuestDifficulty
		Source           models.QuestSource
		XPReward         int
		EstimatedMinutes int
		Reason           string
		Instruction      string
		Tags             []string
		DueHour          int
		DueMinute        int
	}{
		{
			Title:            "Uống nước",
			Description:      "Uống một cốc nước đầy",
			Type:             models.QuestTypeWater,
			Difficulty:       models.QuestDifficultyEasy,
			Source:           models.QuestSourceDailyPlan,
			XPReward:         5,
			EstimatedMinutes: 2,
			Reason:           "Giữ cơ thể đủ nước",
			Instruction:      "Uống ít nhất 250ml nước",
			Tags:             []string{"health", "hydration"},
			DueHour:          9,
			DueMinute:        30,
		},
		{
			Title:            "Nghỉ mắt 5 phút",
			Description:      "Rời mắt khỏi màn hình và thư giãn",
			Type:             models.QuestTypeBreak,
			Difficulty:       models.QuestDifficultyEasy,
			Source:           models.QuestSourceDailyPlan,
			XPReward:         5,
			EstimatedMinutes: 5,
			Reason:           "Giảm mỏi mắt khi làm việc lâu",
			Instruction:      "Nhìn xa hoặc nhắm mắt thư giãn 5 phút",
			Tags:             []string{"health", "focus"},
			DueHour:          10,
			DueMinute:        30,
		},
		{
			Title:            "Đi bộ nhẹ",
			Description:      "Đi bộ hoặc vận động nhẹ trong vài phút",
			Type:             models.QuestTypeMovement,
			Difficulty:       models.QuestDifficultyEasy,
			Source:           models.QuestSourceDailyPlan,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Giúp cơ thể bớt ì sau thời gian ngồi lâu",
			Instruction:      "Đi bộ quanh phòng hoặc ngoài trời",
			Tags:             []string{"movement", "health"},
			DueHour:          14,
			DueMinute:        0,
		},
		{
			Title:            "Học tập 25 phút",
			Description:      "Tập trung học một chủ đề quan trọng",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyMedium,
			Source:           models.QuestSourceDailyPlan,
			XPReward:         15,
			EstimatedMinutes: 25,
			Reason:           "Duy trì tiến độ học tập mỗi ngày",
			Instruction:      "Chọn một nội dung nhỏ và học tập trung 25 phút",
			Tags:             []string{"learning", "focus"},
			DueHour:          16,
			DueMinute:        0,
		},
		{
			Title:            "Daily review",
			Description:      "Nhìn lại ngày hôm nay",
			Type:             models.QuestTypeReview,
			Difficulty:       models.QuestDifficultyEasy,
			Source:           models.QuestSourceDailyPlan,
			XPReward:         10,
			EstimatedMinutes: 5,
			Reason:           "Giúp cải thiện ngày mai",
			Instruction:      "Ghi lại điều tốt, điều khó và một điều muốn cải thiện",
			Tags:             []string{"review", "reflection"},
			DueHour:          21,
			DueMinute:        0,
		},
	}

	for _, qd := range questData {
		tagsJSON, _ := json.Marshal(qd.Tags)

		dueDate := today
		reminderTime := time.Date(today.Year(), today.Month(), today.Day(), qd.DueHour, qd.DueMinute, 0, 0, time.UTC)

		quest := models.Quest{
			UserID:           userID,
			Title:            qd.Title,
			Description:      qd.Description,
			Type:             qd.Type,
			Status:           models.QuestStatusPending,
			Difficulty:       qd.Difficulty,
			Source:           qd.Source,
			XPReward:         qd.XPReward,
			EstimatedMinutes: qd.EstimatedMinutes,
			Reason:           qd.Reason,
			Instruction:      qd.Instruction,
			Tags:             datatypes.JSON(tagsJSON),
			Date:             today,
			DueDate:          &dueDate,
			ReminderTime:     &reminderTime,
		}

		if err := s.db.Create(&quest).Error; err != nil {
			logger.L.Error("failed to create quest", zap.String("title", qd.Title), zap.Error(err))
			return err
		}
	}

	logger.L.Info("sample quests created", zap.Int("count", len(questData)))
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
		{Title: "Seed quest plan created", Content: "5 sample quests seeded for today"},
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
