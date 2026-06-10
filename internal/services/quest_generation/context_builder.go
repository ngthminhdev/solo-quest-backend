package quest_generation

import (
	"context"
	"encoding/json"
	"fmt"
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
		Age                     int      `json:"age"`
		HeightCm                float64  `json:"height_cm"`
		WeightKg                float64  `json:"weight_kg"`
		MainActivity            string   `json:"main_activity"`
		WorkScheduleType        string   `json:"work_schedule_type"`
		WorkWeekdays            []int    `json:"work_weekdays"`
		WorkStartTime           string   `json:"work_start_time"`
		WorkEndTime             string   `json:"work_end_time"`
		WakeUpTime              string   `json:"wake_up_time"`
		TargetSleepTime         string   `json:"target_sleep_time"`
		QuietAfterTime          string   `json:"quiet_after_time"`
		FreeTimeStart           string   `json:"free_time_start"`
		FreeTimeEnd             string   `json:"free_time_end"`
		PreferredFreeTimes      []string `json:"preferred_free_times"`
		FreeTimePreference      string   `json:"free_time_preference"`
		LearningTimePreference  string   `json:"learning_time_preference"`
		LearningTimePreferences []string `json:"learning_time_preferences"`
		MovementTimePreference  string   `json:"movement_time_preference"`
		MovementTimePreferences []string `json:"movement_time_preferences"`
		SleepTimePreference     string   `json:"sleep_time_preference"`
		NutritionTimePreference string   `json:"nutrition_time_preference"`
		ActivityLevel           string   `json:"activity_level"`
		LastWorkout             string   `json:"last_workout"`
		HealthLimitations       []string `json:"health_limitations"`
		MainGoals               []string `json:"main_goals"`
		LearningTopic           *string  `json:"learning_topic"`
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
	existingTypeCount := make(map[string]int)
	for i, q := range existingQuests {
		existingTitles[i] = q.Title
		existingTypeCount[string(q.Type)]++
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

	var sanitizedCats []string
	for _, cat := range enabledCategories {
		if cat != "water" && cat != "breakTime" && cat != "break_time" {
			sanitizedCats = append(sanitizedCats, cat)
		}
	}
	enabledCategories = sanitizedCats

	var rawRules []dto.QuestRuleResponse
	if len(settings.Rules) > 0 {
		_ = json.Unmarshal(settings.Rules, &rawRules)
	}

	var sanitizedRules []dto.QuestRuleResponse
	for _, r := range rawRules {
		if r.ID != "rule_water" && r.ID != "rule_break_time" &&
			r.Type != "water" && r.Type != "breakTime" && r.Type != "break_time" {
			sanitizedRules = append(sanitizedRules, r)
		}
	}
	rules := sanitizedRules

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

	// 7. Extract profile fields with onboarding fallbacks
	var mainActivity string
	if profile.MainActivity != nil && *profile.MainActivity != "" {
		mainActivity = *profile.MainActivity
	} else if hasOnboarding {
		mainActivity = obReq.MainActivity
	}

	age := 0
	if profile.Age != nil {
		age = *profile.Age
	} else if hasOnboarding {
		age = obReq.Age
	}

	height := 0.0
	if profile.HeightCm != nil {
		height = *profile.HeightCm
	} else if hasOnboarding {
		height = obReq.HeightCm
	}

	weight := 0.0
	if profile.WeightKg != nil {
		weight = *profile.WeightKg
	} else if hasOnboarding {
		weight = obReq.WeightKg
	}

	workScheduleType := ""
	if profile.WorkScheduleType != nil {
		workScheduleType = *profile.WorkScheduleType
	} else if hasOnboarding {
		workScheduleType = obReq.WorkScheduleType
	}

	var workWeekdays []int
	if len(profile.WorkWeekdays) > 0 {
		_ = json.Unmarshal(profile.WorkWeekdays, &workWeekdays)
	}
	if len(workWeekdays) == 0 && hasOnboarding {
		workWeekdays = obReq.WorkWeekdays
	}
	if len(workWeekdays) == 0 {
		switch workScheduleType {
		case "weekdays":
			workWeekdays = []int{1, 2, 3, 4, 5}
		case "monday_to_saturday":
			workWeekdays = []int{1, 2, 3, 4, 5, 6}
		case "full_week":
			workWeekdays = []int{1, 2, 3, 4, 5, 6, 7}
		case "night_shift":
			workWeekdays = []int{1, 2, 3, 4, 5}
		}
	}

	workStartTime := ""
	if profile.WorkStartTime != nil {
		workStartTime = *profile.WorkStartTime
	} else if hasOnboarding {
		workStartTime = obReq.WorkStartTime
	}

	workEndTime := ""
	if profile.WorkEndTime != nil {
		workEndTime = *profile.WorkEndTime
	} else if hasOnboarding {
		workEndTime = obReq.WorkEndTime
	}

	var preferredFreeTimes []string
	if len(profile.PreferredFreeTimes) > 0 {
		_ = json.Unmarshal(profile.PreferredFreeTimes, &preferredFreeTimes)
	}
	if len(preferredFreeTimes) == 0 && hasOnboarding {
		preferredFreeTimes = obReq.PreferredFreeTimes
	}

	learningPref := ""
	if profile.LearningTimePreference != nil {
		learningPref = *profile.LearningTimePreference
	} else if hasOnboarding {
		learningPref = obReq.LearningTimePreference
	}

	var learningPrefs []string
	if len(profile.LearningTimePreferences) > 0 {
		_ = json.Unmarshal(profile.LearningTimePreferences, &learningPrefs)
	}
	if len(learningPrefs) == 0 && hasOnboarding {
		learningPrefs = obReq.LearningTimePreferences
	}
	if len(learningPrefs) == 0 && learningPref != "" {
		learningPrefs = []string{learningPref}
	}
	if learningPref == "" && len(learningPrefs) > 0 {
		learningPref = learningPrefs[0]
	}

	movementPref := ""
	if profile.MovementTimePreference != nil {
		movementPref = *profile.MovementTimePreference
	} else if hasOnboarding {
		movementPref = obReq.MovementTimePreference
	}

	var movementPrefs []string
	if len(profile.MovementTimePreferences) > 0 {
		_ = json.Unmarshal(profile.MovementTimePreferences, &movementPrefs)
	}
	if len(movementPrefs) == 0 && hasOnboarding {
		movementPrefs = obReq.MovementTimePreferences
	}
	if len(movementPrefs) == 0 && movementPref != "" {
		movementPrefs = []string{movementPref}
	}
	if movementPref == "" && len(movementPrefs) > 0 {
		movementPref = movementPrefs[0]
	}

	sleepTimePref := ""
	if profile.SleepTimePreference != nil {
		sleepTimePref = *profile.SleepTimePreference
	} else if hasOnboarding {
		sleepTimePref = obReq.SleepTimePreference
	}

	nutritionTimePref := ""
	if profile.NutritionTimePreference != nil {
		nutritionTimePref = *profile.NutritionTimePreference
	} else if hasOnboarding {
		nutritionTimePref = obReq.NutritionTimePreference
	}

	activityLevel := ""
	if profile.ActivityLevel != nil {
		activityLevel = *profile.ActivityLevel
	} else if hasOnboarding {
		activityLevel = obReq.ActivityLevel
	}

	lastWorkout := ""
	if profile.LastWorkout != nil {
		lastWorkout = *profile.LastWorkout
	} else if hasOnboarding {
		lastWorkout = obReq.LastWorkout
	}

	wakeUpTime := ""
	if profile.WakeUpTime != nil {
		wakeUpTime = *profile.WakeUpTime
	} else if hasOnboarding {
		wakeUpTime = obReq.WakeUpTime
	}

	targetSleepTime := ""
	if profile.TargetSleepTime != nil {
		targetSleepTime = *profile.TargetSleepTime
	} else if hasOnboarding {
		targetSleepTime = obReq.TargetSleepTime
	}

	// 7b. Load or create Reminder Settings
	var reminderSettings []models.ReminderSetting
	err = b.db.WithContext(ctx).Where("user_id = ?", userID).Find(&reminderSettings).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load reminder settings: %w", err)
	}

	if len(reminderSettings) == 0 {
		defaults := buildDefaultReminderSettings(userID)
		for _, ds := range defaults {
			_ = b.db.WithContext(ctx).Create(&ds).Error
			reminderSettings = append(reminderSettings, ds)
		}
	}

	reminderCtxs := make([]ReminderSettingContext, len(reminderSettings))
	for i, rs := range reminderSettings {
		reminderCtxs[i] = ReminderSettingContext{
			Type:            string(rs.Type),
			Title:           rs.Title,
			Description:     rs.Description,
			Frequency:       string(rs.Frequency),
			Status:          string(rs.Status),
			StartTime:       rs.StartTime,
			EndTime:         rs.EndTime,
			IntervalMinutes: rs.IntervalMinutes,
			MaxPerDay:       rs.MaxPerDay,
		}
	}

	// 7c. Load schedule blocks from DB
	var blocks []models.ScheduleBlock
	_ = b.db.WithContext(ctx).Where("user_id = ? AND enabled = ?", userID, true).Find(&blocks)
	scheduleBlocks := make([]ScheduleBlockDetail, len(blocks))
	for i, block := range blocks {
		var days []int
		_ = json.Unmarshal(block.DaysOfWeek, &days)
		scheduleBlocks[i] = ScheduleBlockDetail{
			ID:         block.ID.String(),
			Title:      block.Title,
			Type:       string(block.Type),
			StartTime:  block.StartTime,
			EndTime:    block.EndTime,
			DaysOfWeek: days,
			IsBusy:     block.IsBusy,
		}
	}

	// 7d. Load Today Check-in if available
	var checkin models.DailyCheckin
	err = b.db.WithContext(ctx).Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).First(&checkin).Error
	var todayCheckIn *TodayCheckInDetail
	if err == nil {
		todayCheckIn = &TodayCheckInDetail{
			Mood:         checkin.Mood,
			EnergyLevel:  checkin.EnergyLevel,
			Availability: checkin.Availability,
			Priority:     checkin.Priority,
		}
	}

	// 7e. Load Yesterday Review if available
	yesterday := localDate.AddDate(0, 0, -1)
	yStart, yEnd := timeutil.DayRangeVN(yesterday)
	var review models.DailyReview
	err = b.db.WithContext(ctx).Where("user_id = ? AND date >= ? AND date < ?", userID, yStart, yEnd).First(&review).Error
	var prevReview *PreviousDailyReviewDetail
	if err == nil {
		prevReview = &PreviousDailyReviewDetail{
			CompletionRate:      review.CompletionRate,
			CompletedQuestCount: review.CompletedQuestCount,
			SkippedQuestCount:   review.SkippedQuestCount,
		}
	}

	// 7f. Load Active Learning Path if available
	var activePath *ActiveLearningPathDetail
	var trackingRoadmaps []models.UserLearningRoadmap
	err = b.db.WithContext(ctx).Preload("Roadmap").
		Where("user_id = ? AND status = ?", userID, "tracking").
		Order("started_at DESC, updated_at DESC").
		Find(&trackingRoadmaps).Error
	if err == nil && len(trackingRoadmaps) > 0 {
		if len(trackingRoadmaps) > 1 {
			logger.L.Warn("multiple tracking roadmaps found for user, using most recent",
				zap.String("user_id", userID.String()),
				zap.Int("count", len(trackingRoadmaps)),
			)
		}
		activeRoadmap := trackingRoadmaps[0]

		var steps []models.LearningRoadmapStep
		_ = b.db.WithContext(ctx).Where("roadmap_id = ? AND enabled = ?", activeRoadmap.RoadmapID, true).Order("order_index ASC").Find(&steps)

		var progress []models.UserLearningRoadmapStepProgress
		_ = b.db.WithContext(ctx).Where("user_id = ? AND roadmap_id = ? AND completed = ?", userID, activeRoadmap.RoadmapID, true).Find(&progress)
		completedSet := make(map[uuid.UUID]bool)
		for _, p := range progress {
			completedSet[p.StepID] = true
		}

		completedCount := 0
		enabledCount := 0
		for _, s := range steps {
			enabledCount++
			if completedSet[s.ID] {
				completedCount++
			}
		}

		var nextStep *models.LearningRoadmapStep
		for i := range steps {
			if !completedSet[steps[i].ID] {
				nextStep = &steps[i]
				break
			}
		}

		if nextStep == nil && enabledCount > 0 {
			activeRoadmap.Status = models.UserLearningRoadmapStatusCompleted
			now := timeutil.NowUTC()
			if activeRoadmap.CompletedAt == nil {
				activeRoadmap.CompletedAt = &now
			}
			activeRoadmap.UpdatedAt = now
			if saveErr := b.db.WithContext(ctx).Save(&activeRoadmap).Error; saveErr != nil {
				logger.L.Warn("failed to auto-complete roadmap after all steps done",
					zap.String("user_id", userID.String()),
					zap.String("roadmap_id", activeRoadmap.RoadmapID.String()),
					zap.Error(saveErr),
				)
			}
			activePath = nil
		} else if nextStep != nil {
			activePath = &ActiveLearningPathDetail{
				RoadmapID:            activeRoadmap.RoadmapID.String(),
				StepID:               nextStep.ID.String(),
				RoadmapTitle:         activeRoadmap.Roadmap.Title,
				CurrentStepTitle:     nextStep.Title,
				Description:          nextStep.Description,
				StepOrderIndex:       nextStep.OrderIndex,
				CompletedSteps:       completedCount,
				TotalSteps:           enabledCount,
				RoadmapCategory:      activeRoadmap.Roadmap.Category,
				StepEstimatedMinutes: nextStep.EstimatedMinutes,
			}
		}
	}

	// 8. Construct UserQuestContext
	qCtx := &UserQuestContext{
		UserID:            userID,
		LocalDate:         timeutil.StartOfDayVN(localDate),
		Timezone:          "Asia/Ho_Chi_Minh",
		DisplayName:       profile.DisplayName,
		Age:               age,
		Height:            height,
		Weight:            weight,
		MainActivity:      mainActivity,
		MainGoals:         mainGoals,
		WorkScheduleType:  workScheduleType,
		WorkWeekdays:      workWeekdays,
		WorkStartTime:     workStartTime,
		WorkEndTime:       workEndTime,
		ScheduleBlocks:    scheduleBlocks,
		WakeUpTime:        wakeUpTime,
		TargetSleepTime:   targetSleepTime,
		QuietAfterTime:    "", // set below
		FreeTimeStart:     obReq.FreeTimeStart,
		FreeTimeEnd:       obReq.FreeTimeEnd,
		PreferredFreeTimes:      preferredFreeTimes,
		LearningTimePreference:  learningPref,
		LearningTimePreferences: learningPrefs,
		MovementTimePreference:  movementPref,
		MovementTimePreferences: movementPrefs,
		SleepTimePreference:     sleepTimePref,
		NutritionTimePreference: nutritionTimePref,
		ActivityLevel:           activityLevel,
		LastWorkout:             lastWorkout,
		HealthLimitations:       healthLimitations,
		DailyQuestCount:   settings.DailyQuestCount,
		Difficulty:        settings.Difficulty,
		PreferredDuration: settings.PreferredDuration,
		EnabledCategories: enabledCategories,
		Rules:             ruleCtxs,
		ReminderSettings:  reminderCtxs,
		ExistingQuestTitles:    existingTitles,
		ExistingQuestTypeCount: existingTypeCount,
		TodayCheckIn:        todayCheckIn,
		PreviousDailyReview: prevReview,
		ActiveLearningPath:  activePath,
	}

	if profile.QuietAfterTime != nil {
		qCtx.QuietAfterTime = *profile.QuietAfterTime
	} else if hasOnboarding {
		qCtx.QuietAfterTime = obReq.QuietAfterTime
	}

	// 8b. Rest-day / weekend flags. Go's Weekday(): Sunday=0 ... Saturday=6.
	// rest_day_enabled is read from QuestSettings (previously loaded but unused).
	wd := timeutil.StartOfDayVN(localDate).Weekday()
	qCtx.RestDayEnabled = settings.RestDayEnabled
	qCtx.IsWeekend = wd == time.Saturday || wd == time.Sunday
	qCtx.IsRestDay = qCtx.RestDayEnabled && qCtx.IsWeekend

	// 9. Apply reminder policies to enforce aggregate quests and exclusions
	ApplyReminderPolicies(qCtx)

	return qCtx, nil
}

func buildDefaultReminderSettings(userID uuid.UUID) []models.ReminderSetting {
	min90 := 90
	max8 := 8
	max3 := 3
	start08 := "08:00"
	end22 := "22:00"
	start09 := "09:00"
	end18 := "18:00"
	start10 := "10:00"
	end17 := "17:00"
	start20 := "20:00"
	start22 := "22:30"
	start21 := "21:30"

	return []models.ReminderSetting{
		{
			UserID:          userID,
			Type:            models.ReminderTypeWater,
			Title:           "Uống nước",
			Description:     "Nhắc uống nước nhỏ giọt thay vì mục tiêu lớn.",
			Frequency:       models.ReminderFrequencyInterval,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start08,
			EndTime:         &end22,
			IntervalMinutes: &min90,
			MaxPerDay:       &max8,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeBreakTime,
			Title:           "Nghỉ mắt & nghỉ giải lao",
			Description:     "Nhắc nghỉ sau thời gian tập trung.",
			Frequency:       models.ReminderFrequencyInterval,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start09,
			EndTime:         &end18,
			IntervalMinutes: &min90,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeMovement,
			Title:           "Vận động nhẹ",
			Description:     "Nhắc đứng dậy và vận động nhẹ trong ngày.",
			Frequency:       models.ReminderFrequencyRandomInRange,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start10,
			EndTime:         &end17,
			MaxPerDay:       &max3,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeLearning,
			Title:           "Học tập",
			Description:     "Nhắc bạn dành thời gian học vào buổi tối.",
			Frequency:       models.ReminderFrequencyFixed,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start20,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeSleep,
			Title:           "Chuẩn bị ngủ",
			Description:     "Nhắc bạn chuẩn bị ngủ đúng giờ.",
			Frequency:       models.ReminderFrequencyFixed,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start22,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeDailyReview,
			Title:           "Tổng kết ngày",
			Description:     "Nhắc bạn nhìn lại ngày hôm nay.",
			Frequency:       models.ReminderFrequencyFixed,
			Status:          models.ReminderStatusEnabled,
			StartTime:       &start21,
		},
		{
			UserID:          userID,
			Type:            models.ReminderTypeCustom,
			Title:           "Tùy chỉnh",
			Description:     "Nhắc nhở cá nhân do bạn cấu hình.",
			Frequency:       models.ReminderFrequencyFixed,
			Status:          models.ReminderStatusDisabled,
		},
	}
}

func buildDefaultQuestSettings(userID uuid.UUID) *models.QuestSettings {
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "sleep", "review"})
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
	max3 := 3
	max2 := 2
	max1 := 1

	allWeekdays := []int{1, 2, 3, 4, 5, 6, 7}

	return []dto.QuestRuleResponse{
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
