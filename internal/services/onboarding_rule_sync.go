package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/pkg/logger"
)

var goalToCategory = map[string]string{
	"water":        "water",
	"movement":     "movement",
	"learning":     "learning",
	"sleep":        "sleep",
	"focus":        "breakTime",
	"discipline":   "review",
	"mindfulness":  "breakTime",
	"health":       "water",
	"productivity": "review",
	"weight_loss":  "movement",

	// Legacy Vietnamese labels mapping
	"Uống nước": "water",
	"Vận động":  "movement",
	"Học tập":   "learning",
	"Giấc ngủ":  "sleep",
	"Tập trung": "breakTime",
	"Kỷ luật":   "review",
	"Thiền":     "breakTime",
	"Tự ngẫm":   "review",
}

var slotRanges = map[string]struct{ start, end string }{
	"early_morning": {start: "05:30", end: "08:00"},
	"morning":       {start: "08:00", end: "11:30"},
	"lunch":         {start: "11:30", end: "13:30"},
	"afternoon":     {start: "13:30", end: "17:30"},
	"after_work":    {start: "17:30", end: "19:30"},
	"evening":       {start: "19:30", end: "22:00"},
	"night":         {start: "22:00", end: "23:30"},
}

func getCoveringRange(slots []string) (string, string) {
	minStart := "23:59"
	maxEnd := "00:00"
	hasValid := false

	for _, slot := range slots {
		if r, exists := slotRanges[slot]; exists {
			hasValid = true
			if r.start < minStart {
				minStart = r.start
			}
			if r.end > maxEnd {
				maxEnd = r.end
			}
		}
	}

	if !hasValid {
		return "", ""
	}
	return minStart, maxEnd
}

func subtractMinutes(hhmm string, mins int) (string, error) {
	var h, m int
	_, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	if err != nil {
		return "", err
	}

	totalMinutes := h*60 + m - mins
	if totalMinutes < 0 {
		totalMinutes += 24 * 60
	}

	newH := (totalMinutes / 60) % 24
	newM := totalMinutes % 60
	return fmt.Sprintf("%02d:%02d", newH, newM), nil
}

func addMinutes(hhmm string, mins int) (string, error) {
	var h, m int
	_, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	if err != nil {
		return "", err
	}

	totalMinutes := h*60 + m + mins
	newH := (totalMinutes / 60) % 24
	newM := totalMinutes % 60
	return fmt.Sprintf("%02d:%02d", newH, newM), nil
}

func hasHealthLimitations(limitations []string) bool {
	for _, lim := range limitations {
		limLower := strings.ToLower(lim)
		if strings.Contains(limLower, "back") ||
			strings.Contains(limLower, "knee") ||
			strings.Contains(limLower, "low_energy") ||
			strings.Contains(limLower, "đau lưng") ||
			strings.Contains(limLower, "đau khớp") ||
			strings.Contains(limLower, "mệt mỏi") ||
			strings.Contains(limLower, "đau mỏi") ||
			strings.Contains(limLower, "cổ vai gáy") {
			return true
		}
	}
	return false
}

func findRule(rules []dto.QuestRuleResponse, ruleType string) *dto.QuestRuleResponse {
	for i := range rules {
		if rules[i].Type == ruleType {
			return &rules[i]
		}
	}
	return nil
}

func SyncQuestSettingsFromOnboarding(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
	req *OnboardingRequest,
) error {
	// 1. Get or create quest settings for the user
	var settings models.QuestSettings
	err := tx.Where("user_id = ?", userID).First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		var cats []string
		catsJSON, _ := json.Marshal(cats)
		defaultRules := buildDefaultRules()
		rulesJSON, _ := json.Marshal(defaultRules)

		settings = models.QuestSettings{
			UserID:            userID,
			DailyQuestCount:   8,
			Difficulty:        "normal",
			EnabledCategories: datatypes.JSON(catsJSON),
			PreferredDuration: "medium",
			Rules:             datatypes.JSON(rulesJSON),
		}
		if err := tx.Create(&settings).Error; err != nil {
			return fmt.Errorf("failed to create default quest settings: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to load quest settings: %w", err)
	}

	// 2. Unmarshal existing rules
	var rules []dto.QuestRuleResponse
	if len(settings.Rules) > 0 {
		if err := json.Unmarshal(settings.Rules, &rules); err != nil {
			return fmt.Errorf("failed to unmarshal rules: %w", err)
		}
	} else {
		rules = buildDefaultRules()
	}

	// 3. Sync enabled categories based on onboarding main_goals (ignoring legacy water/breakTime)
	enabledCatsMap := make(map[string]bool)
	for _, goal := range req.MainGoals {
		if cat, exists := goalToCategory[goal]; exists {
			if cat != "water" && cat != "breakTime" && cat != "break_time" {
				enabledCatsMap[cat] = true
			}
		}
	}

	var enabledCats []string
	for cat := range enabledCatsMap {
		enabledCats = append(enabledCats, cat)
	}
	enabledCatsJSON, _ := json.Marshal(enabledCats)
	settings.EnabledCategories = datatypes.JSON(enabledCatsJSON)

	// Sync rule enabled status
	for i := range rules {
		rules[i].Enabled = enabledCatsMap[rules[i].Type]
	}

	// 5. Sync learning rule
	if rule := findRule(rules, "learning"); rule != nil {
		oldRange := ""
		if rule.ActiveTimeRange != nil {
			oldRange = fmt.Sprintf("%s-%s", rule.ActiveTimeRange.Start, rule.ActiveTimeRange.End)
		}

		var start, end string
		hasEveningOrFlexible := false
		for _, p := range req.LearningTimePreferences {
			if p == "evening" || p == "flexible" {
				hasEveningOrFlexible = true
				break
			}
		}

		if hasEveningOrFlexible && req.FreeTimeStart != "" && req.FreeTimeEnd != "" {
			start = req.FreeTimeStart
			end = req.FreeTimeEnd
		} else {
			start, end = getCoveringRange(req.LearningTimePreferences)
		}

		if req.QuietAfterTime != "" && end != "" && end > req.QuietAfterTime {
			end = req.QuietAfterTime
		}

		if start != "" && end != "" && start < end {
			rule.ActiveTimeRange = &dto.TimeRangeResponse{Start: start, End: end}
			logger.L.Info("Synced learning rule from time preferences",
				zap.String("user_id", userID.String()),
				zap.String("old_range", oldRange),
				zap.String("new_range", fmt.Sprintf("%s-%s", start, end)),
			)
		} else {
			logger.L.Warn("Skipped learning rule sync: derived time range is invalid or empty",
				zap.String("user_id", userID.String()),
				zap.String("derived_start", start),
				zap.String("derived_end", end),
			)
		}
	}

	// 6. Sync movement rule
	if rule := findRule(rules, "movement"); rule != nil {
		oldRange := ""
		if rule.ActiveTimeRange != nil {
			oldRange = fmt.Sprintf("%s-%s", rule.ActiveTimeRange.Start, rule.ActiveTimeRange.End)
		}

		start, end := getCoveringRange(req.MovementTimePreferences)
		if req.QuietAfterTime != "" && end != "" && end > req.QuietAfterTime {
			end = req.QuietAfterTime
		}

		if start != "" && end != "" && start < end {
			rule.ActiveTimeRange = &dto.TimeRangeResponse{Start: start, End: end}
			logger.L.Info("Synced movement rule from time preferences",
				zap.String("user_id", userID.String()),
				zap.String("old_range", oldRange),
				zap.String("new_range", fmt.Sprintf("%s-%s", start, end)),
			)
		} else {
			logger.L.Warn("Skipped movement rule sync: derived time range is invalid or empty",
				zap.String("user_id", userID.String()),
				zap.String("derived_start", start),
				zap.String("derived_end", end),
			)
		}

		// Cap difficulty if health limitations exist
		if hasHealthLimitations(req.HealthLimitations) {
			oldDiff := rule.Difficulty
			if strings.ToLower(rule.Difficulty) == "hard" || strings.ToLower(rule.Difficulty) == "medium" {
				rule.Difficulty = "easy" // Cap to easy for health limitations
			}
			logger.L.Info("Capped movement rule difficulty due to health limitations",
				zap.String("user_id", userID.String()),
				zap.String("old_difficulty", oldDiff),
				zap.String("new_difficulty", rule.Difficulty),
			)
		}
	}

	// 7. Sync sleep rule
	if rule := findRule(rules, "sleep"); rule != nil {
		oldRange := ""
		if rule.ActiveTimeRange != nil {
			oldRange = fmt.Sprintf("%s-%s", rule.ActiveTimeRange.Start, rule.ActiveTimeRange.End)
		}

		if req.TargetSleepTime != "" {
			endVal := req.TargetSleepTime
			if req.QuietAfterTime != "" && req.QuietAfterTime < endVal {
				endVal = req.QuietAfterTime
			}

			startVal, err := subtractMinutes(endVal, 60)
			if err == nil && startVal >= "18:00" && startVal < endVal {
				rule.ActiveTimeRange = &dto.TimeRangeResponse{Start: startVal, End: endVal}
				logger.L.Info("Synced sleep rule from sleep preferences",
					zap.String("user_id", userID.String()),
					zap.String("old_range", oldRange),
					zap.String("new_range", fmt.Sprintf("%s-%s", startVal, endVal)),
				)
			} else {
				logger.L.Warn("Skipped sleep rule sync: derived start time is before 18:00 or after end time",
					zap.String("user_id", userID.String()),
					zap.String("derived_start", startVal),
					zap.String("derived_end", endVal),
				)
			}
		}
	}

	// 8. Sync review rule
	if rule := findRule(rules, "review"); rule != nil {
		oldRange := ""
		if rule.ActiveTimeRange != nil {
			oldRange = fmt.Sprintf("%s-%s", rule.ActiveTimeRange.Start, rule.ActiveTimeRange.End)
		}

		var start, end string
		var err error

		if req.PreferredReviewTime != "" {
			start = req.PreferredReviewTime
			end, err = addMinutes(start, 30)
			if err == nil && req.QuietAfterTime != "" && end > req.QuietAfterTime {
				end = req.QuietAfterTime
			}
		} else if req.TargetSleepTime != "" {
			start, err = subtractMinutes(req.TargetSleepTime, 60)
			end = req.TargetSleepTime
		}

		if err == nil && start != "" && end != "" && start < end {
			rule.ActiveTimeRange = &dto.TimeRangeResponse{Start: start, End: end}
			logger.L.Info("Synced review rule from preferred review time / sleep preferences",
				zap.String("user_id", userID.String()),
				zap.String("old_range", oldRange),
				zap.String("new_range", fmt.Sprintf("%s-%s", start, end)),
			)
		} else {
			logger.L.Warn("Skipped review rule sync: derived time range is invalid or empty",
				zap.String("user_id", userID.String()),
				zap.String("derived_start", start),
				zap.String("derived_end", end),
			)
		}
	}

	// 9. Sync water rule (no-op as water rule is legacy and reminder-only)

	// 10. Update GORM quest settings rules field
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal updated rules: %w", err)
	}

	settings.Rules = datatypes.JSON(rulesJSON)
	if err := tx.Save(&settings).Error; err != nil {
		return fmt.Errorf("failed to save updated quest settings: %w", err)
	}

	return nil
}
