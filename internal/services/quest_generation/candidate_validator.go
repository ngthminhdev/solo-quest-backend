package quest_generation

import (
	"fmt"
	"strings"
	"time"
)

type CandidateValidator struct{}

func NewCandidateValidator() *CandidateValidator {
	return &CandidateValidator{}
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
	}

	rulesByType := make(map[string]QuestRuleContext)
	for _, rule := range qctx.Rules {
		rulesByType[rule.Type] = rule
	}

	for i, c := range candidates {
		prefix := fmt.Sprintf("candidate[%d] (%s)", i, c.Title)

		// 1. Type validation
		if c.Type == "" {
			errs = append(errs, fmt.Sprintf("%s: type is empty", prefix))
		} else {
			if !allowedQuestTypes[c.Type] {
				errs = append(errs, fmt.Sprintf("%s: invalid quest type '%s'", prefix, c.Type))
			}
			if !enabledCats[c.Type] {
				errs = append(errs, fmt.Sprintf("%s: type '%s' is not enabled in quest settings", prefix, c.Type))
			}
			if rule, exists := rulesByType[c.Type]; exists && !rule.Enabled {
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
				if qctx.QuietAfterTime != "" {
					if c.ReminderTime > qctx.QuietAfterTime {
						errs = append(errs, fmt.Sprintf("%s: reminder_time '%s' is after quiet_after_time '%s'", prefix, c.ReminderTime, qctx.QuietAfterTime))
					}
				}
				if rule, exists := rulesByType[c.Type]; exists && rule.ActiveTimeRange != nil {
					start := rule.ActiveTimeRange.Start
					end := rule.ActiveTimeRange.End
					if start != "" && end != "" {
						if c.ReminderTime < start || c.ReminderTime > end {
							errs = append(errs, fmt.Sprintf("%s: reminder_time '%s' is outside active_time_range [%s, %s] for rule", prefix, c.ReminderTime, start, end))
						}
					}
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
	"water":     true,
	"breakTime": true,
	"movement":  true,
	"learning":  true,
	"sleep":     true,
	"review":    true,
}
