package quest_generation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

type UserQuestContextBuilder struct {
	db *gorm.DB
}

func NewUserQuestContextBuilder(db *gorm.DB) *UserQuestContextBuilder {
	return &UserQuestContextBuilder{db: db}
}

func (b *UserQuestContextBuilder) Build(
	ctx context.Context,
	userID uuid.UUID,
	localDate time.Time,
) (*UserQuestContext, error) {
	// 1. Load User Profile
	var profile models.UserProfile
	err := b.db.WithContext(ctx).Where("id = ?", userID).First(&profile).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load user profile: %w", err)
	}

	// 2. Load latest onboarding answers if available
	var onboarding models.OnboardingAnswer
	var obReq struct {
		MainActivity            string   `json:"main_activity"`
		WorkScheduleType        string   `json:"work_schedule_type"`
		WorkStartTime           string   `json:"work_start_time"`
		WorkEndTime             string   `json:"work_end_time"`
		WakeUpTime              string   `json:"wake_up_time"`
		TargetSleepTime         string   `json:"target_sleep_time"`
		QuietAfterTime          string   `json:"quiet_after_time"`
		FreeTimeStart           string   `json:"free_time_start"`
		FreeTimeEnd             string   `json:"free_time_end"`
		LearningTimePreference  string   `json:"learning_time_preference"`
		LearningTimePreferences []string `json:"learning_time_preferences"`
		MovementTimePreference  string   `json:"movement_time_preference"`
		MovementTimePreferences []string `json:"movement_time_preferences"`
	}

	hasOnboarding := false
	err = b.db.WithContext(ctx).Where("user_id = ?", userID).First(&onboarding).Error
	if err == nil {
		if len(onboarding.Answers) > 0 {
			if jsonErr := json.Unmarshal(onboarding.Answers, &obReq); jsonErr == nil {
				hasOnboarding = true
			}
		}
	}

	// 3. Load or create Quest Settings
	var settings models.QuestSettings
	err = b.db.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		// Create defaults
		defaults := buildDefaultQuestSettings(userID)
		if err = b.db.WithContext(ctx).Create(defaults).Error; err != nil {
			return nil, fmt.Errorf("failed to create default quest settings: %w", err)
		}
		settings = *defaults
	} else if err != nil {
		return nil, fmt.Errorf("failed to load quest settings: %w", err)
	}

	// 4. Load existing quest titles for the same local date
	start, end := timeutil.DayRangeVN(localDate)
	var existingQuests []models.Quest
	err = b.db.WithContext(ctx).Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Find(&existingQuests).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load existing quests: %w", err)
	}

	existingTitles := make([]string, len(existingQuests))
	for i, q := range existingQuests {
		existingTitles[i] = q.Title
	}

	// 5. Parse goals & limitations
	var mainGoals []string
	if len(profile.MainGoals) > 0 {
		_ = json.Unmarshal(profile.MainGoals, &mainGoals)
	}
	var healthLimitations []string
	if len(profile.HealthLimitations) > 0 {
		_ = json.Unmarshal(profile.HealthLimitations, &healthLimitations)
	}

	// 6. Parse quest settings (categories, rules)
	var enabledCategories []string
	if len(settings.EnabledCategories) > 0 {
		_ = json.Unmarshal(settings.EnabledCategories, &enabledCategories)
	}

	var rules []dto.QuestRuleResponse
	if len(settings.Rules) > 0 {
		_ = json.Unmarshal(settings.Rules, &rules)
	}

	ruleCtxs := make([]QuestRuleContext, len(rules))
	for i, r := range rules {
		var trCtx *TimeRangeContext
		if r.ActiveTimeRange != nil {
			trCtx = &TimeRangeContext{
				Start: r.ActiveTimeRange.Start,
				End:   r.ActiveTimeRange.End,
			}
		}
		ruleCtxs[i] = QuestRuleContext{
			ID:                 r.ID,
			Type:               r.Type,
			Title:              r.Title,
			Description:        r.Description,
			Enabled:            r.Enabled,
			Difficulty:         r.Difficulty,
			Priority:           r.Priority,
			MinIntervalMinutes: r.MinIntervalMinutes,
			MaxPerDay:          r.MaxPerDay,
			ActiveTimeRange:    trCtx,
			Weekdays:           r.ActiveWeekdays,
			AdaptToEnergy:      r.AdaptToEnergy,
			AdaptToStress:      r.AdaptToStress,
			AdaptToSchedule:    r.AdaptToSchedule,
		}
	}

	// 7. Normalize time preferences from onboarding
	var mainActivity string
	if profile.MainActivity != nil {
		mainActivity = *profile.MainActivity
	} else if hasOnboarding {
		mainActivity = obReq.MainActivity
	}

	learningPref := obReq.LearningTimePreference
	learningPrefs := obReq.LearningTimePreferences
	if len(learningPrefs) == 0 && learningPref != "" {
		learningPrefs = []string{learningPref}
	}
	if learningPref == "" && len(learningPrefs) > 0 {
		learningPref = learningPrefs[0]
	}

	movementPref := obReq.MovementTimePreference
	movementPrefs := obReq.MovementTimePreferences
	if len(movementPrefs) == 0 && movementPref != "" {
		movementPrefs = []string{movementPref}
	}
	if movementPref == "" && len(movementPrefs) > 0 {
		movementPref = movementPrefs[0]
	}

	// 8. Construct UserQuestContext
	qCtx := &UserQuestContext{
		UserID:            userID,
		LocalDate:         timeutil.StartOfDayVN(localDate),
		Timezone:          "Asia/Ho_Chi_Minh",
		DisplayName:       profile.DisplayName,
		MainActivity:      mainActivity,
		MainGoals:         mainGoals,
		HealthLimitations: healthLimitations,
		WorkScheduleType:  obReq.WorkScheduleType,
		WorkStartTime:     obReq.WorkStartTime,
		WorkEndTime:       obReq.WorkEndTime,
		WakeUpTime:        obReq.WakeUpTime,
		TargetSleepTime:   obReq.TargetSleepTime,
		QuietAfterTime:    obReq.QuietAfterTime,
		FreeTimeStart:     obReq.FreeTimeStart,
		FreeTimeEnd:       obReq.FreeTimeEnd,
		LearningTimePreference:  learningPref,
		LearningTimePreferences: learningPrefs,
		MovementTimePreference:  movementPref,
		MovementTimePreferences: movementPrefs,
		DailyQuestCount:   settings.DailyQuestCount,
		Difficulty:        settings.Difficulty,
		PreferredDuration: settings.PreferredDuration,
		EnabledCategories: enabledCategories,
		Rules:             ruleCtxs,
		ExistingQuestTitles: existingTitles,
	}

	return qCtx, nil
}

func buildDefaultQuestSettings(userID uuid.UUID) *models.QuestSettings {
	catsJSON, _ := json.Marshal([]string{"water", "breakTime", "movement", "learning", "sleep", "review"})
	rulesJSON, _ := json.Marshal(buildDefaultRules())

	now := timeutil.NowUTC()
	return &models.QuestSettings{
		UserID:            userID,
		DailyQuestCount:   8,
		Difficulty:        "normal",
		AutoAdjustEnabled: true,
		EnabledCategories: datatypes.JSON(catsJSON),
		PreferredDuration: "medium",
		RestDayEnabled:    false,
		Rules:             datatypes.JSON(rulesJSON),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func buildDefaultRules() []dto.QuestRuleResponse {
	min90 := 90
	max8 := 8
	max6 := 6
	max3 := 3
	max2 := 2
	max1 := 1

	allWeekdays := []int{1, 2, 3, 4, 5, 6, 7}

	return []dto.QuestRuleResponse{
		{
			ID:                 "rule_water",
			Type:               "water",
			Title:              "Uống nước",
			Description:        "Nhắc bạn uống nước đều trong ngày",
			Enabled:            true,
			Difficulty:         "easy",
			MinIntervalMinutes: &min90,
			MaxPerDay:          &max8,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "08:00", End: "22:00"},
			ActiveWeekdays:     allWeekdays,
			Priority:           5,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
		{
			ID:                 "rule_break_time",
			Type:               "breakTime",
			Title:              "Nghỉ giải lao",
			Description:        "Nhắc bạn nghỉ ngắn để tránh quá tải",
			Enabled:            true,
			Difficulty:         "easy",
			MinIntervalMinutes: &min90,
			MaxPerDay:          &max6,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "09:00", End: "18:00"},
			ActiveWeekdays:     allWeekdays,
			Priority:           5,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
		{
			ID:                 "rule_movement",
			Type:               "movement",
			Title:              "Vận động nhẹ",
			Description:        "Gợi ý vận động nhẹ trong ngày",
			Enabled:            true,
			Difficulty:         "medium",
			MinIntervalMinutes: &min90,
			MaxPerDay:          &max3,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "10:00", End: "20:00"},
			ActiveWeekdays:     allWeekdays,
			Priority:           4,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
		{
			ID:                 "rule_learning",
			Type:               "learning",
			Title:              "Học tập",
			Description:        "Quest học tập theo mục tiêu của bạn",
			Enabled:            true,
			Difficulty:         "medium",
			MinIntervalMinutes: &min90,
			MaxPerDay:          &max2,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "19:00", End: "22:00"},
			ActiveWeekdays:     allWeekdays,
			Priority:           4,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
		{
			ID:                 "rule_sleep",
			Type:               "sleep",
			Title:              "Giấc ngủ",
			Description:        "Nhắc bạn chuẩn bị ngủ đúng giờ",
			Enabled:            true,
			Difficulty:         "easy",
			MinIntervalMinutes: nil,
			MaxPerDay:          &max1,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "22:00", End: "23:30"},
			ActiveWeekdays:     allWeekdays,
			Priority:           3,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
		{
			ID:                 "rule_review",
			Type:               "review",
			Title:              "Review cuối ngày",
			Description:        "Nhắc bạn nhìn lại ngày hôm nay",
			Enabled:            true,
			Difficulty:         "easy",
			MinIntervalMinutes: nil,
			MaxPerDay:          &max1,
			ActiveTimeRange:    &dto.TimeRangeResponse{Start: "21:00", End: "23:00"},
			ActiveWeekdays:     allWeekdays,
			Priority:           3,
			AdaptToEnergy:      true,
			AdaptToStress:      true,
			AdaptToSchedule:    true,
		},
	}
}
