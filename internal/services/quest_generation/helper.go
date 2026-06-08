package quest_generation

import (
	"fmt"
	"strings"
	"time"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

// CalculateSleepTimes computes the actual sleep datetime and sleep reminder time.
// questDate is the target quest date (local date).
// targetSleepTime is a string in "HH:mm" format.
func CalculateSleepTimes(questDate time.Time, targetSleepTime string) (time.Time, time.Time, error) {
	var hour, minute int
	if targetSleepTime == "" {
		hour, minute = 23, 0 // Default target sleep time
	} else {
		_, err := fmt.Sscanf(targetSleepTime, "%d:%d", &hour, &minute)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("failed to parse target sleep time '%s': %w", targetSleepTime, err)
		}
	}

	today := timeutil.StartOfDayVN(questDate)

	var actualSleep time.Time
	if hour >= 0 && hour <= 4 {
		// Overnight sleep on the next day
		actualSleep = time.Date(today.Year(), today.Month(), today.Day()+1, hour, minute, 0, 0, timeutil.LocationVN)
	} else {
		// Same day sleep
		actualSleep = time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
	}

	// Sleep reminder is 30 minutes before actual sleep
	reminderTime := actualSleep.Add(-30 * time.Minute)
	return actualSleep, reminderTime, nil
}

// GetCutoffTimeForNormalQuests computes the latest allowed time for normal daily quests (e.g. learning, movement).
func GetCutoffTimeForNormalQuests(questDate time.Time, qctx *UserQuestContext) time.Time {
	today := timeutil.StartOfDayVN(questDate)
	if qctx.QuietAfterTime != "" {
		var h, m int
		if _, err := fmt.Sscanf(qctx.QuietAfterTime, "%d:%d", &h, &m); err == nil {
			return time.Date(today.Year(), today.Month(), today.Day(), h, m, 0, 0, timeutil.LocationVN)
		}
	}

	if qctx.TargetSleepTime != "" {
		_, sleepReminder, err := CalculateSleepTimes(questDate, qctx.TargetSleepTime)
		if err == nil {
			// Cutoff is 30 minutes before sleep reminder
			return sleepReminder.Add(-30 * time.Minute)
		}
	}

	// Default cutoff: 22:00
	return time.Date(today.Year(), today.Month(), today.Day(), 22, 0, 0, 0, timeutil.LocationVN)
}

// NormalizeTags converts tags to canonical lowercase codes.
func NormalizeTags(tags []string) []string {
	tagSet := make(map[string]bool)
	var normalized []string
	for _, t := range tags {
		tLower := strings.ToLower(strings.TrimSpace(t))
		if tLower == "" {
			continue
		}
		var canonical string
		switch tLower {
		case "sức khỏe", "suc khoe", "health":
			canonical = "health"
		case "vận động", "van dong", "movement":
			canonical = "movement"
		case "học tập", "hoc tap", "learning", "study":
			canonical = "learning"
		case "giấc ngủ", "giac ngu", "sleep", "rest":
			canonical = "sleep"
		case "review", "phản ánh", "phan anh", "daily review", "daily_review":
			canonical = "review"
		case "hydration", "nước", "nuoc", "water":
			canonical = "hydration"
		case "break", "breaktime", "nghỉ ngơi", "nghi ngoi", "tập trung", "tap trung":
			canonical = "break"
		case "đọc sách", "doc sach", "reading":
			canonical = "reading"
		case "ngoại ngữ", "ngu phap", "tu vung", "vocabulary", "language":
			canonical = "language"
		case "coding", "lap trinh", "lập trình":
			canonical = "coding"
		case "roadmap":
			canonical = "roadmap"
		default:
			canonical = tLower
		}

		if !tagSet[canonical] {
			tagSet[canonical] = true
			normalized = append(normalized, canonical)
		}
	}
	return normalized
}

// NormalizeCandidates adjusts reminder times, handles sleep, and normalizes tags for Quest Candidates.
func NormalizeCandidates(qctx *UserQuestContext, candidates []QuestCandidate, now time.Time) []QuestCandidate {
	if qctx == nil {
		return candidates
	}

	today := timeutil.StartOfDayVN(qctx.LocalDate)
	isToday := today.Equal(timeutil.StartOfDayVN(now))

	// Find cutoff for normal quests (learning, movement)
	cutoff := GetCutoffTimeForNormalQuests(qctx.LocalDate, qctx)

	var sleepCand *QuestCandidate
	var reviewCand *QuestCandidate
	var normalCands []QuestCandidate

	for i := range candidates {
		c := &candidates[i]
		c.Tags = NormalizeTags(c.Tags)
		normType := NormalizeType(c.Type)

		if normType == "sleep" {
			sleepCand = c
		} else if normType == "review" {
			reviewCand = c
		} else if normType != "water" && normType != "breakTime" {
			normalCands = append(normalCands, *c)
		}
	}

	// 1. Normalize Sleep Quest reminder
	var sleepReminder time.Time
	if sleepCand != nil {
		_, sr, err := CalculateSleepTimes(qctx.LocalDate, qctx.TargetSleepTime)
		if err == nil {
			sleepReminder = sr
		} else {
			sleepReminder = time.Date(today.Year(), today.Month(), today.Day(), 22, 30, 0, 0, timeutil.LocationVN)
		}

		if isToday && sleepReminder.Before(now) {
			sleepReminder = now.Add(5 * time.Minute)
		}
		sleepCand.ReminderTime = sleepReminder.Format("15:04")
	} else {
		_, sleepReminder, _ = CalculateSleepTimes(qctx.LocalDate, qctx.TargetSleepTime)
	}

	// 2. Normalize Review Quest reminder
	if reviewCand != nil {
		var reviewReminder time.Time
		if !sleepReminder.IsZero() {
			reviewReminder = sleepReminder.Add(-30 * time.Minute)
		} else {
			reviewReminder = time.Date(today.Year(), today.Month(), today.Day(), 21, 30, 0, 0, timeutil.LocationVN)
		}

		if isToday && reviewReminder.Before(now) {
			reviewReminder = now.Add(5 * time.Minute)
			if !sleepReminder.IsZero() && reviewReminder.After(sleepReminder) {
				reviewReminder = sleepReminder.Add(-1 * time.Minute)
			}
		}
		reviewCand.ReminderTime = reviewReminder.Format("15:04")
	}

	// 3. Normalize Normal Quests (movement, learning, etc.)
	var finalNormal []QuestCandidate
	if isToday {
		nextSlot := now.Add(15 * time.Minute)
		min := nextSlot.Minute()
		rem := min % 5
		if rem > 0 {
			nextSlot = nextSlot.Add(time.Duration(5-rem) * time.Minute)
		}
		nextSlot = nextSlot.Truncate(time.Minute)

		for _, c := range normalCands {
			var hour, minute int
			_, err := fmt.Sscanf(c.ReminderTime, "%d:%d", &hour, &minute)
			if err != nil {
				continue
			}

			candTime := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
			if candTime.After(now) {
				if candTime.Before(cutoff) {
					finalNormal = append(finalNormal, c)
				}
				continue
			}

			// In the past, attempt to shift
			if nextSlot.Before(cutoff) {
				c.ReminderTime = nextSlot.Format("15:04")
				finalNormal = append(finalNormal, c)
				nextSlot = nextSlot.Add(30 * time.Minute)
			}
		}
	} else {
		// Future date: simply filter out those past the cutoff just in case
		for _, c := range normalCands {
			var hour, minute int
			_, err := fmt.Sscanf(c.ReminderTime, "%d:%d", &hour, &minute)
			if err != nil {
				continue
			}
			candTime := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
			if candTime.Before(cutoff) {
				finalNormal = append(finalNormal, c)
			}
		}
	}

	var result []QuestCandidate
	for _, c := range finalNormal {
		result = append(result, c)
	}
	if reviewCand != nil {
		result = append(result, *reviewCand)
	}
	if sleepCand != nil {
		result = append(result, *sleepCand)
	}

	return result
}

// NormalizeQuests adjusts reminder times, handles sleep, and normalizes tags for models.Quest slice directly.
func NormalizeQuests(qctx *UserQuestContext, quests []models.Quest, now time.Time) []models.Quest {
	if qctx == nil {
		return quests
	}

	today := timeutil.StartOfDayVN(qctx.LocalDate)
	isToday := today.Equal(timeutil.StartOfDayVN(now))
	cutoff := GetCutoffTimeForNormalQuests(qctx.LocalDate, qctx)

	var sleepQuest *models.Quest
	var reviewQuest *models.Quest
	var normalQuests []models.Quest

	for i := range quests {
		q := &quests[i]
		normType := NormalizeType(string(q.Type))

		if normType == "sleep" {
			sleepQuest = q
		} else if normType == "review" {
			reviewQuest = q
		} else if normType != "water" && normType != "breakTime" {
			normalQuests = append(normalQuests, *q)
		}
	}

	// 1. Sleep Quest
	var sleepReminder time.Time
	if sleepQuest != nil {
		_, sr, err := CalculateSleepTimes(qctx.LocalDate, qctx.TargetSleepTime)
		if err == nil {
			sleepReminder = sr
		} else {
			sleepReminder = time.Date(today.Year(), today.Month(), today.Day(), 22, 30, 0, 0, timeutil.LocationVN)
		}

		if isToday && sleepReminder.Before(now) {
			sleepReminder = now.Add(5 * time.Minute)
		}
		sleepQuest.ReminderTime = &sleepReminder
	} else {
		_, sleepReminder, _ = CalculateSleepTimes(qctx.LocalDate, qctx.TargetSleepTime)
	}

	// 2. Review Quest
	if reviewQuest != nil {
		var reviewReminder time.Time
		if !sleepReminder.IsZero() {
			reviewReminder = sleepReminder.Add(-30 * time.Minute)
		} else {
			reviewReminder = time.Date(today.Year(), today.Month(), today.Day(), 21, 30, 0, 0, timeutil.LocationVN)
		}

		if isToday && reviewReminder.Before(now) {
			reviewReminder = now.Add(5 * time.Minute)
			if !sleepReminder.IsZero() && reviewReminder.After(sleepReminder) {
				reviewReminder = sleepReminder.Add(-1 * time.Minute)
			}
		}
		reviewQuest.ReminderTime = &reviewReminder
	}

	// 3. Normal Quests
	var finalNormal []models.Quest
	if isToday {
		nextSlot := now.Add(15 * time.Minute)
		min := nextSlot.Minute()
		rem := min % 5
		if rem > 0 {
			nextSlot = nextSlot.Add(time.Duration(5-rem) * time.Minute)
		}
		nextSlot = nextSlot.Truncate(time.Minute)

		for _, q := range normalQuests {
			if q.ReminderTime == nil {
				continue
			}
			candTime := *q.ReminderTime
			if candTime.After(now) {
				if candTime.Before(cutoff) {
					finalNormal = append(finalNormal, q)
				}
				continue
			}

			// In the past, shift
			if nextSlot.Before(cutoff) {
				shifted := nextSlot
				q.ReminderTime = &shifted
				finalNormal = append(finalNormal, q)
				nextSlot = nextSlot.Add(30 * time.Minute)
			}
		}
	} else {
		for _, q := range normalQuests {
			if q.ReminderTime == nil {
				continue
			}
			if q.ReminderTime.Before(cutoff) {
				finalNormal = append(finalNormal, q)
			}
		}
	}

	var result []models.Quest
	for _, q := range finalNormal {
		result = append(result, q)
	}
	if reviewQuest != nil {
		result = append(result, *reviewQuest)
	}
	if sleepQuest != nil {
		result = append(result, *sleepQuest)
	}

	return result
}

// IsPastReminderForToday checks if a reminder time is already in the past for today's quest date.
func IsPastReminderForToday(questDate time.Time, reminderTime string, now time.Time) bool {
	today := timeutil.StartOfDayVN(questDate)
	if !today.Equal(timeutil.StartOfDayVN(now)) {
		return false
	}

	var hour, minute int
	if _, err := fmt.Sscanf(reminderTime, "%d:%d", &hour, &minute); err != nil {
		return false
	}

	candTime := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, timeutil.LocationVN)
	if hour >= 0 && hour <= 4 {
		candTime = candTime.AddDate(0, 0, 1)
	}

	return candTime.Before(now.Add(-1 * time.Minute))
}

// NextSafeTimeSlot computes the next available reminder slot from now.
func NextSafeTimeSlot(startTime time.Time) time.Time {
	nextSlot := startTime.Add(15 * time.Minute)
	min := nextSlot.Minute()
	rem := min % 5
	if rem > 0 {
		nextSlot = nextSlot.Add(time.Duration(5-rem) * time.Minute)
	}
	return nextSlot.Truncate(time.Minute)
}

// HasReviewEnabled returns true if review or daily_review is in the enabled categories.
func HasReviewEnabled(enabledCategories []string) bool {
	for _, cat := range enabledCategories {
		if NormalizeType(cat) == "review" {
			return true
		}
	}
	return false
}

// HasCategoryEnabled returns true if the given category (after normalization) is in the enabled list.
func HasCategoryEnabled(enabledCategories []string, category string) bool {
	normTarget := NormalizeType(category)
	for _, cat := range enabledCategories {
		if NormalizeType(cat) == normTarget {
			return true
		}
	}
	return false
}
