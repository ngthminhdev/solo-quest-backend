package quest_generation

import (
	"fmt"
	"strings"
	"time"

	"solo_quest_backend/internal/pkg/timeutil"
)

type CandidateValidator struct {
	Now time.Time
}

func NewCandidateValidator() *CandidateValidator {
	return &CandidateValidator{}
}

func NewCandidateValidatorWithNow(now time.Time) *CandidateValidator {
	return &CandidateValidator{Now: now}
}

func (v *CandidateValidator) getNow() time.Time {
	if !v.Now.IsZero() {
		return v.Now
	}
	return timeutil.NowVN()
}

func (v *CandidateValidator) Validate(qctx *UserQuestContext, candidates []QuestCandidate) error {
	var errs []string

	if qctx == nil {
		return fmt.Errorf("UserQuestContext cannot be nil")
	}

	if len(candidates) == 0 {
		return fmt.Errorf("candidates list cannot be empty")
	}

	if len(candidates) > qctx.DailyQuestCount {
		errs = append(errs, fmt.Sprintf("candidate count (%d) exceeds daily quest count limit (%d)", len(candidates), qctx.DailyQuestCount))
	}

	usedTitles := make(map[string]bool)
	existingTitles := make(map[string]bool)
	for _, title := range qctx.ExistingQuestTitles {
		existingTitles[strings.ToLower(strings.TrimSpace(title))] = true
	}

	enabledCats := make(map[string]bool)
	for _, cat := range qctx.EnabledCategories {
		enabledCats[cat] = true
		enabledCats[NormalizeType(cat)] = true
	}

	rulesByType := make(map[string]QuestRuleContext)
	for _, rule := range qctx.Rules {
		rulesByType[rule.Type] = rule
		rulesByType[NormalizeType(rule.Type)] = rule
	}

	// Count candidates by type to enforce aggregate limits
	typeCounts := make(map[string]int)
	for _, c := range candidates {
		typeCounts[NormalizeType(c.Type)]++
	}

	if typeCounts["movement"] > 0 {
		allowedMax := 1
		if rule, exists := rulesByType["movement"]; exists && rule.MaxPerDay != nil {
			allowedMax = *rule.MaxPerDay
		}
		if typeCounts["movement"] > allowedMax {
			errs = append(errs, fmt.Sprintf("movement: cannot generate more than %d movement quests", allowedMax))
		}
	}

	if typeCounts["sleep"] > 1 {
		errs = append(errs, "sleep: cannot generate more than 1 sleep quest")
	}

	if typeCounts["review"] > 1 {
		errs = append(errs, "review: cannot generate more than 1 daily review quest")
	}

	// Detect review enabled but missing
	reviewEnabled := false
	for _, cat := range qctx.EnabledCategories {
		if NormalizeType(cat) == "review" {
			reviewEnabled = true
			break
		}
	}
	if reviewEnabled && typeCounts["review"] == 0 {
		errs = append(errs, "review: review is enabled but no review quest was generated")
	}

	learningCount := typeCounts["learning"]
	if qctx.ActiveLearningPath == nil {
		if learningCount > 1 {
			errs = append(errs, "learning: cannot generate more than 1 learning quest without active learning path")
		}
		// Check for invented learning topics
		for _, c := range candidates {
			if NormalizeType(c.Type) == "learning" {
				lowerContent := strings.ToLower(c.Title + " " + c.Description)
				specificKeywords := []string{
					"english", "tiếng anh", "vocabulary", "từ vựng", "coding", "lập trình",
					"grammar", "ngữ pháp", "book", "sách", "đọc sách", "toán", "math",
				}
				for _, kw := range specificKeywords {
					if strings.Contains(lowerContent, kw) {
						errs = append(errs, fmt.Sprintf("learning: generic learning quest cannot invent specific topic '%s' without active learning path", kw))
						break
					}
				}
			}
		}
	}

	for i, c := range candidates {
		prefix := fmt.Sprintf("candidate[%d] (%s)", i, c.Title)
		normType := NormalizeType(c.Type)

		// 1. Type validation
		if c.Type == "" {
			errs = append(errs, fmt.Sprintf("%s: type is empty", prefix))
		} else {
			if !allowedQuestTypes[normType] && !allowedQuestTypes[c.Type] {
				errs = append(errs, fmt.Sprintf("%s: invalid quest type '%s'", prefix, c.Type))
			}
			if IsReminderOnlyDailyType(c.Type) {
				errs = append(errs, fmt.Sprintf("%s: type '%s' is reminder-only and cannot be generated as a quest", prefix, c.Type))
			}
			if !enabledCats[normType] && !enabledCats[c.Type] {
				errs = append(errs, fmt.Sprintf("%s: type '%s' is not enabled in quest settings", prefix, c.Type))
			}
			if rule, exists := rulesByType[normType]; exists && !rule.Enabled {
				errs = append(errs, fmt.Sprintf("%s: rule for type '%s' is disabled", prefix, c.Type))
			}
		}

		// 2. Title validation
		trimmedTitle := strings.TrimSpace(c.Title)
		if trimmedTitle == "" {
			errs = append(errs, fmt.Sprintf("candidate[%d]: title is empty", i))
		} else {
			if len(trimmedTitle) > 120 {
				errs = append(errs, fmt.Sprintf("%s: title exceeds 120 characters", prefix))
			}
			lowerTitle := strings.ToLower(trimmedTitle)
			if usedTitles[lowerTitle] {
				errs = append(errs, fmt.Sprintf("%s: duplicate title in candidate list", prefix))
			}
			usedTitles[lowerTitle] = true

			if existingTitles[lowerTitle] {
				errs = append(errs, fmt.Sprintf("%s: title duplicates an existing quest for today", prefix))
			}

			if strings.EqualFold(trimmedTitle, "do something") || strings.EqualFold(trimmedTitle, "làm gì đó") {
				errs = append(errs, fmt.Sprintf("%s: title is too vague", prefix))
			}

			lowerTitleContent := strings.ToLower(c.Title + " " + c.Description)
			dangerWords := []string{"suicide", "tự tử", "chết", "nguy hiểm", "extreme", "hurt", "doctor", "bác sĩ", "medicine", "thuốc"}
			for _, word := range dangerWords {
				if strings.Contains(lowerTitleContent, word) {
					errs = append(errs, fmt.Sprintf("%s: content contains unsafe or medical keyword '%s'", prefix, word))
					break
				}
			}
		}

		// 3. Difficulty validation
		diff := strings.ToLower(c.Difficulty)
		if diff != "easy" && diff != "normal" && diff != "medium" && diff != "hard" {
			errs = append(errs, fmt.Sprintf("%s: invalid difficulty '%s'", prefix, c.Difficulty))
		}

		// 4. Estimated minutes validation
		if c.EstimatedMinutes < 1 || c.EstimatedMinutes > 45 {
			errs = append(errs, fmt.Sprintf("%s: estimated minutes (%d) must be between 1 and 45", prefix, c.EstimatedMinutes))
		}

		// 5. XP reward validation
		expectedXP := 10
		if diff == "easy" {
			expectedXP = 5
		} else if diff == "hard" {
			expectedXP = 20
		}
		if c.XPReward != expectedXP {
			errs = append(errs, fmt.Sprintf("%s: xp_reward (%d) does not match expected value (%d) for difficulty '%s'", prefix, c.XPReward, expectedXP, c.Difficulty))
		}

		// 6. Reminder time validation
		if c.ReminderTime == "" {
			errs = append(errs, fmt.Sprintf("%s: reminder_time is empty", prefix))
		} else {
			_, timeErr := time.Parse("15:04", c.ReminderTime)
			if timeErr != nil {
				errs = append(errs, fmt.Sprintf("%s: invalid reminder_time format '%s' (expected HH:mm)", prefix, c.ReminderTime))
			} else {
				if qctx.QuietAfterTime != "" && normType != "sleep" && normType != "review" {
					if c.ReminderTime > qctx.QuietAfterTime {
						errs = append(errs, fmt.Sprintf("%s: reminder_time '%s' is after quiet_after_time '%s'", prefix, c.ReminderTime, qctx.QuietAfterTime))
					}
				}
				if rule, exists := rulesByType[normType]; exists && rule.ActiveTimeRange != nil {
					start := rule.ActiveTimeRange.Start
					end := rule.ActiveTimeRange.End
					if start != "" && end != "" {
						isOutside := false
						if start <= end {
							isOutside = c.ReminderTime < start || c.ReminderTime > end
						} else {
							// Crossing midnight (e.g. 21:00 to 03:00). Outside if both > end AND < start.
							isOutside = c.ReminderTime > end && c.ReminderTime < start
						}
						if isOutside {
							errs = append(errs, fmt.Sprintf("%s: reminder_time '%s' is outside active_time_range [%s, %s] for rule", prefix, c.ReminderTime, start, end))
						}
					}
				}
			}
		}

		// 7. Tags validation
		for _, tag := range c.Tags {
			if !isCanonicalTag(tag) {
				errs = append(errs, fmt.Sprintf("%s: tag '%s' is not a canonical code", prefix, tag))
			}
		}

		// 8. Past reminder validation (strictly reject if today)
		today := timeutil.StartOfDayVN(qctx.LocalDate)
		now := v.getNow()
		isToday := today.Equal(timeutil.StartOfDayVN(now))
		if isToday && c.ReminderTime != "" {
			var hour, minute int
			if _, timeErr := fmt.Sscanf(c.ReminderTime, "%d:%d", &hour, &minute); timeErr == nil {
				candTime := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
				if normType == "sleep" && hour >= 0 && hour <= 4 {
					candTime = candTime.AddDate(0, 0, 1)
				}
				if candTime.Before(now.Add(-1 * time.Minute)) {
					errs = append(errs, fmt.Sprintf("%s: reminder_time '%s' is in the past", prefix, c.ReminderTime))
				}
			}
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}

	return nil
}

type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}

var allowedQuestTypes = map[string]bool{
	"breakTime": true,
	"movement":  true,
	"learning":  true,
	"sleep":     true,
	"review":    true,
}

var allowedCanonicalTags = map[string]bool{
	"health":    true,
	"hydration": true,
	"break":     true,
	"movement":  true,
	"learning":  true,
	"reading":   true,
	"language":  true,
	"coding":    true,
	"roadmap":   true,
	"review":    true,
	"sleep":     true,
}

func isCanonicalTag(tag string) bool {
	return allowedCanonicalTags[tag]
}
