package quest_generation

import (
	"solo_quest_backend/internal/models"
)

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
