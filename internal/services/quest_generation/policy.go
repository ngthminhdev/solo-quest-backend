package quest_generation

import (
	"strings"

	"solo_quest_backend/internal/models"
)

// Estimated-minutes bounds for a single daily quest. Values outside this range
// are clamped (not rejected) by the quality gate.
const (
	MinEstimatedMinutes = 5
	MaxEstimatedMinutes = 45
)

// XPForDifficulty returns the backend-authoritative XP reward for a difficulty.
// AI-provided xp_reward is never trusted; the backend always recomputes it from
// the (repaired) difficulty so a wrong/missing xp_reward can't fail a candidate.
func XPForDifficulty(difficulty string) int {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return 5
	case "hard":
		return 20
	default:
		// normal / medium / unknown -> normal reward
		return 10
	}
}

// NormalizeDifficulty maps a free-form difficulty to one of easy/normal/hard,
// defaulting to "normal" for anything unrecognized.
func NormalizeDifficulty(difficulty string) string {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return "easy"
	case "hard":
		return "hard"
	case "normal", "medium":
		return "normal"
	default:
		return "normal"
	}
}

// DefaultEstimatedMinutes picks a sensible duration from the user's preferred
// duration when a candidate has no usable estimated_minutes.
func DefaultEstimatedMinutes(preferredDuration string) int {
	switch strings.ToLower(strings.TrimSpace(preferredDuration)) {
	case "short":
		return 10
	case "long":
		return 35
	default:
		// medium / unknown
		return 20
	}
}

// ClampEstimatedMinutes repairs an estimated_minutes value instead of rejecting
// it: missing/zero/negative falls back to the preferred-duration default, and
// out-of-range values are clamped into [MinEstimatedMinutes, MaxEstimatedMinutes].
func ClampEstimatedMinutes(minutes int, preferredDuration string) int {
	if minutes <= 0 {
		return DefaultEstimatedMinutes(preferredDuration)
	}
	if minutes > MaxEstimatedMinutes {
		return MaxEstimatedMinutes
	}
	if minutes < MinEstimatedMinutes {
		return MinEstimatedMinutes
	}
	return minutes
}

// FilterCanonicalTags drops unknown tags (after synonym normalization) instead
// of rejecting the candidate. When nothing canonical survives, a safe
// type-based tag is added so the quest is never left untagged.
func FilterCanonicalTags(tags []string, questType string) []string {
	normalized := NormalizeTags(tags)
	out := make([]string, 0, len(normalized))
	for _, t := range normalized {
		if isCanonicalTag(t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		switch NormalizeType(questType) {
		case "movement":
			out = append(out, "movement")
		case "learning":
			out = append(out, "learning")
		case "sleep":
			out = append(out, "sleep")
		case "review":
			out = append(out, "review")
		}
	}
	return out
}

// AllowedDailyQuestTypes are the canonical categories the daily quest
// generator (AI + rule-based fallback) is allowed to produce.
//
// NOTE on taxonomy: "water" and "breakTime"/"eyeBreak" are reminder habits /
// micro reminders handled by the reminder module, NOT daily quests. They must
// never be generated as daily quests, even if a user still has them enabled as
// categories/rules (the context builder sanitizes them out, and the validator
// rejects them as a defence-in-depth). Reminder-only types live in
// IsReminderOnlyDailyType below.
func AllowedDailyQuestTypes() []string {
	return []string{"movement", "learning", "sleep", "review"}
}

// IsReminderOnlyDailyType reports whether a category is a reminder habit that
// must never be generated as a daily quest (water, breakTime/eyeBreak).
func IsReminderOnlyDailyType(questType string) bool {
	switch NormalizeType(questType) {
	case "water", "breakTime", "eyeBreak", "eye_break":
		return true
	default:
		return false
	}
}

// NormalizeType maps reminder setting types to daily quest types.
func NormalizeType(t string) string {
	switch t {
	case "break_time":
		return "breakTime"
	case "daily_review":
		return "review"
	default:
		return t
	}
}

// IsReminderOnlyType returns true if the quest type and frequency indicate it's purely a reminder
// and shouldn't be blindly expanded into daily quests.
func IsReminderOnlyType(questType string, frequency string) bool {
	normType := NormalizeType(questType)
	if normType == "breakTime" && frequency == "interval" {
		return true
	}
	return false
}

// ShouldCreateDailyQuestFromReminder returns true if a daily quest should be created for the setting.
func ShouldCreateDailyQuestFromReminder(setting models.ReminderSetting) bool {
	if setting.Status != models.ReminderStatusEnabled {
		return false
	}
	if IsReminderOnlyType(string(setting.Type), string(setting.Frequency)) {
		return false
	}
	return true
}

// ApplyReminderPolicies updates the rules context with reminder settings policies.
func ApplyReminderPolicies(qctx *UserQuestContext) {
	if qctx == nil {
		return
	}

	// Create a fast lookup for reminder settings by normalized type
	reminders := make(map[string]ReminderSettingContext)
	for _, r := range qctx.ReminderSettings {
		reminders[NormalizeType(r.Type)] = r
	}

	for i := range qctx.Rules {
		rule := &qctx.Rules[i]
		normType := NormalizeType(rule.Type)
		reminder, hasReminder := reminders[normType]

		// If corresponding reminder setting is disabled, disable the rule by default
		if hasReminder && reminder.Status == string(models.ReminderStatusDisabled) {
			rule.Enabled = false
			continue
		}

		switch normType {
		case "water":
			// water: reminder-only, generate 0 daily quests
			rule.Enabled = false
		case "breakTime":
			// break_time: reminder-only, generate 0 daily quests
			rule.Enabled = false
		case "movement":
			// movement: generate at most ONE movement quest per day unless intentionally allowed more
			if rule.MaxPerDay == nil || *rule.MaxPerDay > 1 {
				// check if reminder setting is random_in_range
				if hasReminder && reminder.Frequency == string(models.ReminderFrequencyRandomInRange) {
					maxVal := 1
					rule.MaxPerDay = &maxVal
					rule.MinIntervalMinutes = nil
				} else if rule.MaxPerDay == nil {
					maxVal := 1
					rule.MaxPerDay = &maxVal
				}
			}
		case "review":
			// daily_review: generate at most ONE per day
			maxVal := 1
			rule.MaxPerDay = &maxVal
			rule.MinIntervalMinutes = nil
		case "custom":
			// custom: only generate if status enabled (handled above by status check)
			if !hasReminder || reminder.Status != string(models.ReminderStatusEnabled) {
				rule.Enabled = false
			}
		}
	}
}

// GetAggregateQuestDetails returns the default title and description for aggregate quest types
func GetAggregateQuestDetails(questType string) (title, description string, isAggregate bool) {
	switch NormalizeType(questType) {
	case "water":
		return "Uống nước đều hôm nay", "Uống nước nhỏ giọt trong ngày, không cần hoàn thành một lần.", true
	case "movement":
		return "Vận động nhẹ 10 phút", "Đứng dậy đi lại hoặc giãn cơ nhẹ.", true
	default:
		return "", "", false
	}
}
