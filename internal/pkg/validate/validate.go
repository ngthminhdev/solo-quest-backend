package validate

import (
	"regexp"

	"solo_quest_backend/internal/models"
)

var hhmmRegex = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func IsValidHHMM(s string) bool {
	return hhmmRegex.MatchString(s)
}

func IsValidFrequency(f string) bool {
	return models.IsValidReminderFrequency(f)
}

func IsValidQuestType(t string) bool {
	for _, v := range models.ValidQuestTypes() {
		if string(v) == t {
			return true
		}
	}
	return false
}

func IsValidReminderType(t string) bool {
	return models.IsValidReminderType(t)
}

func IsValidReminderStatus(s string) bool {
	return models.IsValidReminderStatus(s)
}

var validSettingsDifficulties = map[string]bool{
	"easy": true, "normal": true, "hard": true,
}

func IsValidGlobalDifficulty(d string) bool {
	return validSettingsDifficulties[d]
}

var validRuleDifficulties = map[string]bool{
	"easy": true, "medium": true, "hard": true,
}

func IsValidRuleDifficulty(d string) bool {
	return validRuleDifficulties[d]
}

var validPreferredDurations = map[string]bool{
	"short": true, "medium": true, "long": true,
}

func IsValidPreferredDuration(d string) bool {
	return validPreferredDurations[d]
}

var feQuestTypeValues = map[string]bool{
	"water": true, "breakTime": true, "movement": true,
	"learning": true, "sleep": true, "fitness": true,
	"mindfulness": true, "review": true, "custom": true,
}

func IsValidFEQuestType(t string) bool {
	return feQuestTypeValues[t]
}

func IsValidActiveWeekdays(weekdays []int) bool {
	if len(weekdays) == 0 {
		return false
	}
	seen := make(map[int]bool)
	for _, d := range weekdays {
		if d < 1 || d > 7 {
			return false
		}
		if seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}

func IsValidPriority(p int) bool {
	return p >= 1 && p <= 5
}

func IsValidNullableMinIntervalMinutes(v *int) bool {
	if v == nil {
		return true
	}
	return *v >= 15
}

func IsValidNullableMaxPerDay(v *int) bool {
	if v == nil {
		return true
	}
	return *v >= 0
}
