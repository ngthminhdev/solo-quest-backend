package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/pkg/validate"
)

type QuestSettingsService struct {
	db *gorm.DB
}

func NewQuestSettingsService(db *gorm.DB) *QuestSettingsService {
	return &QuestSettingsService{db: db}
}

func (s *QuestSettingsService) GetOrCreate(userID uuid.UUID) (*dto.QuestSettingsResponse, error) {
	var settings models.QuestSettings
	err := s.db.Where("user_id = ?", userID).First(&settings).Error
	if err == nil {
		return toQuestSettingsResponse(&settings)
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return s.createDefaults(userID)
}

func (s *QuestSettingsService) createDefaults(userID uuid.UUID) (*dto.QuestSettingsResponse, error) {
	settings := BuildDefaultQuestSettings(userID)
	if err := s.db.Create(settings).Error; err != nil {
		return nil, err
	}
	return toQuestSettingsResponse(settings)
}

func (s *QuestSettingsService) Update(userID uuid.UUID, req *dto.UpdateQuestSettingsRequest) (*dto.QuestSettingsResponse, error) {
	settings, err := s.getOrFetchDefaults(userID)
	if err != nil {
		return nil, err
	}

	// Sanitize request: filter out water/break_time/breakTime from req.EnabledCategories
	if req.EnabledCategories != nil {
		var sanitizedCats []string
		for _, cat := range req.EnabledCategories {
			if cat != "water" && cat != "breakTime" && cat != "break_time" {
				sanitizedCats = append(sanitizedCats, cat)
			}
		}
		req.EnabledCategories = sanitizedCats
	}

	// Sanitize request: filter out rule_water/rule_break_time/water/breakTime/break_time from req.Rules
	if req.Rules != nil {
		var sanitizedRules []dto.UpdateQuestRuleRequest
		for _, rule := range req.Rules {
			if rule.ID != "rule_water" && rule.ID != "rule_break_time" &&
				(rule.Type == nil || (*rule.Type != "water" && *rule.Type != "breakTime" && *rule.Type != "break_time")) {
				sanitizedRules = append(sanitizedRules, rule)
			}
		}
		req.Rules = sanitizedRules
	}

	if err := validateQuestSettingsRequest(req); err != nil {
		return nil, err
	}

	if req.DailyQuestCount != nil {
		settings.DailyQuestCount = *req.DailyQuestCount
	}
	if req.Difficulty != nil {
		settings.Difficulty = *req.Difficulty
	}
	if req.AutoAdjustEnabled != nil {
		settings.AutoAdjustEnabled = *req.AutoAdjustEnabled
	}
	if req.EnabledCategories != nil {
		catsJSON, _ := json.Marshal(req.EnabledCategories)
		settings.EnabledCategories = datatypes.JSON(catsJSON)
	} else {
		// Clean existing EnabledCategories in DB from legacy values
		var existingCats []string
		if len(settings.EnabledCategories) > 0 {
			_ = json.Unmarshal(settings.EnabledCategories, &existingCats)
		}
		var sanitizedCats []string
		for _, cat := range existingCats {
			if cat != "water" && cat != "breakTime" && cat != "break_time" {
				sanitizedCats = append(sanitizedCats, cat)
			}
		}
		catsJSON, _ := json.Marshal(sanitizedCats)
		settings.EnabledCategories = datatypes.JSON(catsJSON)
	}
	if req.PreferredDuration != nil {
		settings.PreferredDuration = *req.PreferredDuration
	}
	if req.RestDayEnabled != nil {
		settings.RestDayEnabled = *req.RestDayEnabled
	}

	// Clean existing Rules in DB from legacy values and merge
	var existingRules []dto.QuestRuleResponse
	if len(settings.Rules) > 0 {
		_ = json.Unmarshal(settings.Rules, &existingRules)
	}
	var sanitizedRules []dto.QuestRuleResponse
	for _, r := range existingRules {
		if r.ID != "rule_water" && r.ID != "rule_break_time" &&
			r.Type != "water" && r.Type != "breakTime" && r.Type != "break_time" {
			sanitizedRules = append(sanitizedRules, r)
		}
	}
	sanitizedRulesJSON, _ := json.Marshal(sanitizedRules)
	settings.Rules = datatypes.JSON(sanitizedRulesJSON)

	if req.Rules != nil && len(req.Rules) > 0 {
		if err := mergeRules(settings, req.Rules); err != nil {
			return nil, err
		}
	}

	settings.UpdatedAt = timeutil.NowUTC()

	if err := s.db.Save(settings).Error; err != nil {
		return nil, err
	}

	return toQuestSettingsResponse(settings)
}

func (s *QuestSettingsService) Reset(userID uuid.UUID) (*dto.QuestSettingsResponse, error) {
	defaults := BuildDefaultQuestSettings(userID)

	var existing models.QuestSettings
	err := s.db.Where("user_id = ?", userID).First(&existing).Error
	if err == nil {
		existing.DailyQuestCount = defaults.DailyQuestCount
		existing.Difficulty = defaults.Difficulty
		existing.AutoAdjustEnabled = defaults.AutoAdjustEnabled
		existing.EnabledCategories = defaults.EnabledCategories
		existing.PreferredDuration = defaults.PreferredDuration
		existing.RestDayEnabled = defaults.RestDayEnabled
		existing.Rules = defaults.Rules
		existing.UpdatedAt = timeutil.NowUTC()
		if err := s.db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return toQuestSettingsResponse(&existing)
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err := s.db.Create(defaults).Error; err != nil {
		return nil, err
	}
	return toQuestSettingsResponse(defaults)
}

func (s *QuestSettingsService) getOrFetchDefaults(userID uuid.UUID) (*models.QuestSettings, error) {
	var settings models.QuestSettings
	err := s.db.Where("user_id = ?", userID).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if err == gorm.ErrRecordNotFound {
		defaults := BuildDefaultQuestSettings(userID)
		if err := s.db.Create(defaults).Error; err != nil {
			return nil, err
		}
		return defaults, nil
	}
	return nil, err
}

func BuildDefaultQuestSettings(userID uuid.UUID) *models.QuestSettings {
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

func mergeRules(settings *models.QuestSettings, incoming []dto.UpdateQuestRuleRequest) error {
	var existingRules []dto.QuestRuleResponse
	if len(settings.Rules) > 0 {
		if err := json.Unmarshal(settings.Rules, &existingRules); err != nil {
			existingRules = []dto.QuestRuleResponse{}
		}
	}

	rulesByID := make(map[string]dto.QuestRuleResponse)
	for _, r := range existingRules {
		if r.ID != "" {
			rulesByID[r.ID] = r
		}
	}

	for _, in := range incoming {
		if in.ID == "" {
			return fmt.Errorf("rule id is required for update")
		}

		existing, found := rulesByID[in.ID]
		if !found {
			return fmt.Errorf("unknown rule id: %s", in.ID)
		}

		rulesByID[in.ID] = mergeRule(existing, in)
	}

	var merged []dto.QuestRuleResponse
	for _, r := range existingRules {
		if updated, ok := rulesByID[r.ID]; ok {
			merged = append(merged, updated)
		} else {
			merged = append(merged, r)
		}
	}

	rulesJSON, _ := json.Marshal(merged)
	settings.Rules = datatypes.JSON(rulesJSON)
	return nil
}

func mergeRule(existing dto.QuestRuleResponse, in dto.UpdateQuestRuleRequest) dto.QuestRuleResponse {
	if in.Enabled != nil {
		existing.Enabled = *in.Enabled
	}
	if in.Difficulty != nil && *in.Difficulty != "" {
		existing.Difficulty = *in.Difficulty
	}
	if in.Title != nil && *in.Title != "" {
		existing.Title = *in.Title
	}
	if in.Description != nil && *in.Description != "" {
		existing.Description = *in.Description
	}
	if in.Type != nil && *in.Type != "" {
		existing.Type = *in.Type
	}
	if in.MinIntervalMinutes != nil {
		existing.MinIntervalMinutes = in.MinIntervalMinutes
	}
	if in.MaxPerDay != nil {
		existing.MaxPerDay = in.MaxPerDay
	}
	if in.ActiveTimeRange != nil {
		existing.ActiveTimeRange = &dto.TimeRangeResponse{
			Start: in.ActiveTimeRange.Start,
			End:   in.ActiveTimeRange.End,
		}
	}
	if in.ActiveWeekdays != nil {
		existing.ActiveWeekdays = *in.ActiveWeekdays
	}
	if in.Priority != nil {
		existing.Priority = *in.Priority
	}
	if in.AdaptToEnergy != nil {
		existing.AdaptToEnergy = *in.AdaptToEnergy
	}
	if in.AdaptToStress != nil {
		existing.AdaptToStress = *in.AdaptToStress
	}
	if in.AdaptToSchedule != nil {
		existing.AdaptToSchedule = *in.AdaptToSchedule
	}

	return existing
}

func toQuestSettingsResponse(s *models.QuestSettings) (*dto.QuestSettingsResponse, error) {
	var cats []string
	if len(s.EnabledCategories) > 0 {
		if err := json.Unmarshal(s.EnabledCategories, &cats); err != nil {
			cats = []string{}
		}
	} else {
		cats = []string{}
	}

	// Filter legacy categories: remove water, break_time, breakTime
	var sanitizedCats []string
	for _, cat := range cats {
		if cat != "water" && cat != "breakTime" && cat != "break_time" {
			sanitizedCats = append(sanitizedCats, cat)
		}
	}
	cats = sanitizedCats

	var rules []dto.QuestRuleResponse
	if len(s.Rules) > 0 {
		if err := json.Unmarshal(s.Rules, &rules); err != nil {
			rules = []dto.QuestRuleResponse{}
		}
	} else {
		rules = []dto.QuestRuleResponse{}
	}

	// Filter legacy rules: remove rule_water, rule_break_time, breakTime, break_time, water
	var sanitizedRules []dto.QuestRuleResponse
	for _, r := range rules {
		if r.ID != "rule_water" && r.ID != "rule_break_time" &&
			r.Type != "water" && r.Type != "breakTime" && r.Type != "break_time" {
			sanitizedRules = append(sanitizedRules, r)
		}
	}
	rules = sanitizedRules

	return &dto.QuestSettingsResponse{
		DailyQuestCount:   s.DailyQuestCount,
		Difficulty:        s.Difficulty,
		AutoAdjustEnabled: s.AutoAdjustEnabled,
		EnabledCategories: cats,
		PreferredDuration: s.PreferredDuration,
		RestDayEnabled:    s.RestDayEnabled,
		Rules:             rules,
	}, nil
}

func validateQuestSettingsRequest(req *dto.UpdateQuestSettingsRequest) error {
	if req.DailyQuestCount != nil {
		if *req.DailyQuestCount < 1 || *req.DailyQuestCount > 20 {
			return fmt.Errorf("daily_quest_count must be between 1 and 20")
		}
	}
	if req.Difficulty != nil && !validate.IsValidGlobalDifficulty(*req.Difficulty) {
		return fmt.Errorf("invalid difficulty, must be easy, normal, or hard")
	}
	if req.PreferredDuration != nil && !validate.IsValidPreferredDuration(*req.PreferredDuration) {
		return fmt.Errorf("invalid preferred_duration, must be short, medium, or long")
	}
	if req.EnabledCategories != nil {
		for _, cat := range req.EnabledCategories {
			if !validate.IsValidFEQuestType(cat) {
				return fmt.Errorf("invalid enabled category: %s", cat)
			}
		}
	}
	if req.Rules != nil {
		for _, rule := range req.Rules {
			if err := validateRule(rule); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRule(rule dto.UpdateQuestRuleRequest) error {
	if rule.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	if rule.Type != nil && !validate.IsValidFEQuestType(*rule.Type) {
		return fmt.Errorf("invalid rule type: %s", *rule.Type)
	}
	if rule.Difficulty != nil && !validate.IsValidRuleDifficulty(*rule.Difficulty) {
		return fmt.Errorf("invalid rule difficulty, must be easy, medium, or hard")
	}
	if rule.ActiveWeekdays != nil && !validate.IsValidActiveWeekdays(*rule.ActiveWeekdays) {
		return fmt.Errorf("invalid active_weekdays, must be non-empty subset of 1-7")
	}
	if rule.Priority != nil && !validate.IsValidPriority(*rule.Priority) {
		return fmt.Errorf("priority must be between 1 and 5")
	}
	if rule.MinIntervalMinutes != nil && !validate.IsValidNullableMinIntervalMinutes(rule.MinIntervalMinutes) {
		return fmt.Errorf("min_interval_minutes must be null or >= 15")
	}
	if rule.MaxPerDay != nil && !validate.IsValidNullableMaxPerDay(rule.MaxPerDay) {
		return fmt.Errorf("max_per_day must be null or >= 0")
	}
	if rule.ActiveTimeRange != nil {
		if !validate.IsValidHHMM(rule.ActiveTimeRange.Start) {
			return fmt.Errorf("invalid active_time_range.start, expected HH:MM")
		}
		if !validate.IsValidHHMM(rule.ActiveTimeRange.End) {
			return fmt.Errorf("invalid active_time_range.end, expected HH:MM")
		}
		if rule.ActiveTimeRange.Start >= rule.ActiveTimeRange.End {
			return fmt.Errorf("active_time_range start must be before end")
		}
	}
	return nil
}

func buildDefaultRules() []dto.QuestRuleResponse {
	min90 := 90
	max3 := 3
	max2 := 2
	max1 := 1

	allWeekdays := []int{1, 2, 3, 4, 5, 6, 7}

	return []dto.QuestRuleResponse{
		{
			ID:                "rule_movement",
			Type:              "movement",
			Title:             "Vận động nhẹ",
			Description:       "Gợi ý vận động nhẹ trong ngày",
			Enabled:           true,
			Difficulty:        "medium",
			MinIntervalMinutes: &min90,
			MaxPerDay:         &max3,
			ActiveTimeRange:   &dto.TimeRangeResponse{Start: "10:00", End: "20:00"},
			ActiveWeekdays:    allWeekdays,
			Priority:          4,
			AdaptToEnergy:     true,
			AdaptToStress:     true,
			AdaptToSchedule:   true,
		},
		{
			ID:                "rule_learning",
			Type:              "learning",
			Title:             "Học tập",
			Description:       "Quest học tập theo mục tiêu của bạn",
			Enabled:           true,
			Difficulty:        "medium",
			MinIntervalMinutes: &min90,
			MaxPerDay:         &max2,
			ActiveTimeRange:   &dto.TimeRangeResponse{Start: "19:00", End: "22:00"},
			ActiveWeekdays:    allWeekdays,
			Priority:          4,
			AdaptToEnergy:     true,
			AdaptToStress:     true,
			AdaptToSchedule:   true,
		},
		{
			ID:                "rule_sleep",
			Type:              "sleep",
			Title:             "Giấc ngủ",
			Description:       "Nhắc bạn chuẩn bị ngủ đúng giờ",
			Enabled:           true,
			Difficulty:        "easy",
			MinIntervalMinutes: nil,
			MaxPerDay:         &max1,
			ActiveTimeRange:   &dto.TimeRangeResponse{Start: "22:00", End: "23:30"},
			ActiveWeekdays:    allWeekdays,
			Priority:          3,
			AdaptToEnergy:     true,
			AdaptToStress:     true,
			AdaptToSchedule:   true,
		},
		{
			ID:                "rule_review",
			Type:              "review",
			Title:             "Review cuối ngày",
			Description:       "Nhắc bạn nhìn lại ngày hôm nay",
			Enabled:           true,
			Difficulty:        "easy",
			MinIntervalMinutes: nil,
			MaxPerDay:         &max1,
			ActiveTimeRange:   &dto.TimeRangeResponse{Start: "21:00", End: "23:00"},
			ActiveWeekdays:    allWeekdays,
			Priority:          3,
			AdaptToEnergy:     true,
			AdaptToStress:     true,
			AdaptToSchedule:   true,
		},
	}
}
