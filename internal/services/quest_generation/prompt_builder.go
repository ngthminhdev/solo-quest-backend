package quest_generation

import (
	"encoding/json"
	"fmt"
	"strings"

	"solo_quest_backend/internal/pkg/timeutil"
)

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func (b *PromptBuilder) BuildDailyQuestPrompt(qctx *UserQuestContext) (systemPrompt string, userPrompt string, err error) {
	if qctx == nil {
		return "", "", fmt.Errorf("UserQuestContext cannot be nil")
	}

	if len(qctx.EnabledCategories) == 0 {
		return "", "", fmt.Errorf("enabled_categories cannot be empty")
	}

	if qctx.DailyQuestCount <= 0 {
		return "", "", fmt.Errorf("daily_quest_count must be greater than 0")
	}

	systemPrompt = b.buildSystemPrompt()
	userPrompt = b.buildUserPrompt(qctx)

	return systemPrompt, userPrompt, nil
}

func (b *PromptBuilder) buildSystemPrompt() string {
	return `You are a quest generator for SoloQuest, a daily self-improvement app.

OUTPUT FORMAT:
- Return raw JSON only.
- No Markdown. No code fences. No explanations.
- The final answer must be in message.content.
- Do NOT put JSON in reasoning_content.

SCHEMA:
{
  "quests": [
    {
      "type": "learning|movement|sleep|review",
      "title": "string",
      "description": "string",
      "difficulty": "easy|normal|hard",
      "estimated_minutes": 10,
      "xp_reward": 10,
      "tags": ["string"],
      "reason": "string",
      "instruction": "string",
      "reminder_time": "HH:mm"
    }
  ]
}

RULES:
- User configuration is authoritative. Quest rules are hard constraints.
- Constraint priority:
  1. enabled_categories and enabled rules
  2. usable_reminder_window / active_time_range / quiet_after_time
  3. target/max daily quest count
  4. difficulty and XP mapping
  5. user time preferences
  6. user goals and creativity
- daily_quest_count is a TARGET/MAXIMUM count, NOT a mandatory/hard requirement. You may generate FEWER quests than daily_quest_count if the context does not support enough high-quality tasks (e.g. in late evening, we prefer review and sleep and fewer/no learning tasks).
- Do NOT generate duplicate generic learning quests just to reach the daily quest count.
- Do not schedule outside the usable_reminder_window.
- Every reminder_time must be inside the exact active_time_range (or usable_reminder_window) of the selected quest type.
- Do not place reminders before active_time_range.start or after active_time_range.end.
- For water / hydration: Generate ZERO water daily quests (water/hydration is reminder-only, do not generate any water/hydration daily quests).
- For breakTime / focus: Generate ZERO breakTime daily quests (break_time is reminder-only, do not generate any breakTime daily quests).
- For health: The "health" category in main_goals represents general wellness/health context. Do not generate water quests or breakTime quests from it.
- For movement: Generate at most ONE movement quest per day unless explicitly configured otherwise.
  * Keep movement tasks gentle, safe, and appropriate based on age, height, weight, activity level, last workout, and health limitations.
  * Wording: Use "trong mức thoải mái" and "dừng lại nếu thấy khó chịu".
- For learning:
  * If an Active Learning Path (roadmap and step) is available, use its current step title and description to construct a roadmap-based learning quest.
  * If no Active Learning Path is available, generate at most ONE generic learning quest (e.g. "Học tập 20 phút", description "Dành một khoảng thời gian ngắn để học hoặc ôn lại nội dung quan trọng.").
  * If no Active Learning Path is available, do NOT invent specific learning topics (like coding, vocabulary, grammar, reading specific books, math, etc.). Only generate a generic learning quest.
  * Do NOT use the onboarding learning_topic as the main source of learning details.
- For sleep: Generate at most ONE sleep quest per day.
  * Sleep times (target_sleep_time) between 00:00 and 04:00 are overnight/end-of-day sleep times, not start-of-day. The sleep reminder should be target sleep time minus 30 minutes (e.g. if target sleep is 00:00, reminder is 23:30; if target sleep is 01:00, reminder is 00:30 of the next day).
- For review (daily review): Generate at most ONE daily review quest per day (type must be "review"). If review/daily_review is enabled in categories, you MUST include at least one review quest.
- Today's date and past reminders:
  * Do NOT create reminder_time values in the past. If generating for today's date, you must ensure all reminder times are strictly after the user's current local time (provided in the prompt context).
  * If generating in the late evening (current local time >= 20:00), you MUST:
    - Prefer review and sleep over learning quests.
    - Reduce the overall quest count (3-4 quests total is reasonable).
    - At most ONE short learning quest; prefer to drop learning entirely.
    - Include one review quest if review is enabled.
    - Include one sleep quest if sleep is enabled.
    - Optional: one gentle movement quest if it still fits comfortably.
- Technical fields (type, difficulty, tags) must be canonical codes in English.
  * Valid types: "learning", "movement", "sleep", "review".
  * Valid tags: "health", "hydration", "break", "movement", "learning", "reading", "language", "coding", "roadmap", "review", "sleep".
- Workday & Schedule Blocks:
  * If today is NOT a working day, do NOT generate work-related quests.
- Today Check-In & Previous Review Adjustments:
  * Today Check-In: If energy_level is low, generate easier quests. If availability is busy, generate fewer/shorter quests. If priority is health/movement/sleep/learning, prioritize that category.
  * Previous Review: If yesterday's completion rate was low, reduce today's load.
- difficulty: "easy", "normal" (for medium), or "hard"
- xp_reward: easy=5, normal=10, hard=20
- estimated_minutes: 1-45
- Respect active_time_range when present
- reminder_time must not be after quiet_after_time
- Avoid duplicate titles and existing_quest_titles
- No medical, dangerous, or extreme tasks. Do not claim a quest treats, reduces, or improves symptoms. Do not provide medical, rehab, or injury-specific exercise instructions.
- If health limitations are specified: Keep movement tasks gentle, generic, and optional. Use wording like "trong mức thoải mái" and "dừng lại nếu thấy khó chịu". Prefer walking or light mobility over body-part-specific exercises.
- Health limitation example:
  * Bad: "Bài tập lưng giúp giảm đau lưng"
  * Good: "Vận động nhẹ 10 phút trong mức thoải mái"
- No guilt-inducing content

OUTPUT LANGUAGE:
- Technical fields (type, difficulty, tags) in English.
- title, description, reason, instruction must be natural Vietnamese.
- Do NOT mix English words into Vietnamese user-facing fields. Avoid awkward loanwords (like "hydrated", "stretching", "running", "walking") unless there is no common Vietnamese equivalent.`
}

func (b *PromptBuilder) buildUserPrompt(qctx *UserQuestContext) string {
	var sb strings.Builder

	// Determine quest count to generate
	questCount := qctx.DailyQuestCount
	if qctx.PreviewLimit > 0 {
		questCount = qctx.PreviewLimit
	}

	weekday := qctx.LocalDate.Weekday()
	weekdayNum := int(weekday)
	if weekdayNum == 0 {
		weekdayNum = 7 // 1 = Mon, 7 = Sun
	}

	isWorkday := false
	for _, wd := range qctx.WorkWeekdays {
		if wd == weekdayNum {
			isWorkday = true
			break
		}
	}

	// Section A: GENERATION TARGET
	sb.WriteString("A. GENERATION TARGET:\n")
	sb.WriteString(fmt.Sprintf("- Date: %s (%s)\n", timeutil.FormatDateVN(qctx.LocalDate), weekday.String()))
	sb.WriteString(fmt.Sprintf("- Current Local Time: %s\n", timeutil.NowVN().Format("2006-01-02T15:04:05-07:00")))
	sb.WriteString(fmt.Sprintf("- Today's Day of Week: %d (1=Mon, 7=Sun)\n", weekdayNum))
	sb.WriteString(fmt.Sprintf("- Is Today a Scheduled Workday?: %t (based on work_weekdays)\n", isWorkday))
	if qctx.RequestedPreviewLimit != nil {
		sb.WriteString(fmt.Sprintf("- Requested Preview Limit: %d\n", *qctx.RequestedPreviewLimit))
	} else {
		sb.WriteString("- Requested Preview Limit: nil\n")
	}
	sb.WriteString(fmt.Sprintf("- Effective Preview Limit: %d\n", questCount))
	sb.WriteString(fmt.Sprintf("- Daily Quest Target Count: %d\n", qctx.DailyQuestCount))
	sb.WriteString(fmt.Sprintf("- Enabled Categories: %s\n", strings.Join(qctx.EnabledCategories, ", ")))
	if len(qctx.ExistingQuestTitles) > 0 {
		sb.WriteString("- Existing Quests Today (avoid duplicate titles):\n")
		for _, title := range qctx.ExistingQuestTitles {
			sb.WriteString(fmt.Sprintf("  * %s\n", title))
		}
	} else {
		sb.WriteString("- Existing Quests Today: none\n")
	}
	sb.WriteString("\n")

	// Section B: GLOBAL HARD CONSTRAINTS
	sb.WriteString("B. GLOBAL HARD CONSTRAINTS:\n")
	if qctx.QuietAfterTime != "" {
		sb.WriteString(fmt.Sprintf("- quiet_after_time: %s\n", qctx.QuietAfterTime))
	} else {
		sb.WriteString("- quiet_after_time: none\n")
	}
	if qctx.TargetSleepTime != "" {
		sb.WriteString(fmt.Sprintf("- target_sleep_time: %s\n", qctx.TargetSleepTime))
	} else {
		sb.WriteString("- target_sleep_time: none\n")
	}
	if qctx.PreferredDuration != "" {
		sb.WriteString(fmt.Sprintf("- preferred_duration: %s\n", qctx.PreferredDuration))
	}
	if qctx.Difficulty != "" {
		sb.WriteString(fmt.Sprintf("- difficulty: %s\n", qctx.Difficulty))
	}
	sb.WriteString("- STATEMENT: No reminder_time may be scheduled after quiet_after_time.\n\n")

	// Filter enabled categories for rules check
	enabledCats := make(map[string]bool)
	for _, cat := range qctx.EnabledCategories {
		enabledCats[cat] = true
	}

	// Section C: HARD RULE WINDOWS
	sb.WriteString("C. HARD RULE WINDOWS:\n")
	usableRulesCount := 0
	totalCapacity := 0
	for _, rule := range qctx.Rules {
		if !rule.Enabled {
			continue
		}
		if !enabledCats[rule.Type] {
			continue
		}

		sb.WriteString(fmt.Sprintf("- %s:\n", rule.Type))
		sb.WriteString("  enabled: true\n")
		if rule.Difficulty != "" {
			sb.WriteString(fmt.Sprintf("  difficulty: %s\n", rule.Difficulty))
		}
		if rule.MaxPerDay != nil {
			sb.WriteString(fmt.Sprintf("  max_per_day: %d\n", *rule.MaxPerDay))
		} else {
			sb.WriteString("  max_per_day: 999\n")
		}

		rawStart, rawEnd := "00:00", "23:59"
		hasRawRange := false
		if rule.ActiveTimeRange != nil && rule.ActiveTimeRange.Start != "" && rule.ActiveTimeRange.End != "" {
			rawStart = rule.ActiveTimeRange.Start
			rawEnd = rule.ActiveTimeRange.End
			hasRawRange = true
		}

		if hasRawRange {
			sb.WriteString(fmt.Sprintf("  raw_active_time_range: %s-%s\n", rawStart, rawEnd))
		} else {
			sb.WriteString("  raw_active_time_range: not_specified\n")
		}

		usableStart := rawStart
		usableEnd := rawEnd
		if qctx.QuietAfterTime != "" {
			if usableEnd > qctx.QuietAfterTime {
				usableEnd = qctx.QuietAfterTime
			}
		}

		if usableEnd < usableStart {
			sb.WriteString("  usable_reminder_window: unavailable\n")
			sb.WriteString("  hard_rule: DO NOT generate quests for this category unless there are no alternatives\n")
		} else {
			usableRulesCount++
			if rule.MaxPerDay != nil {
				totalCapacity += *rule.MaxPerDay
			} else {
				totalCapacity += 999
			}

			if usableStart == usableEnd {
				sb.WriteString(fmt.Sprintf("  usable_reminder_window: %s-%s\n", usableStart, usableEnd))
				sb.WriteString(fmt.Sprintf("  hard_rule: reminder_time must be exactly %s\n", usableStart))
			} else {
				sb.WriteString(fmt.Sprintf("  usable_reminder_window: %s-%s\n", usableStart, usableEnd))
				sb.WriteString(fmt.Sprintf("  hard_rule: reminder_time must be between %s and %s\n", usableStart, usableEnd))
			}
		}
	}
	sb.WriteString("Any quest outside its type's window is invalid.\n\n")

	// Section C2: CONFIG CAPACITY SUMMARY
	sb.WriteString("CONFIG CAPACITY SUMMARY:\n")
	sb.WriteString(fmt.Sprintf("- Available Enabled Rules Count: %d\n", usableRulesCount))
	sb.WriteString(fmt.Sprintf("- Total Usable Capacity (sum of max_per_day): %d\n", totalCapacity))
	sb.WriteString(fmt.Sprintf("- Requested Preview Limit: %d\n\n", questCount))

	// Section D: USER TIME PREFERENCES
	sb.WriteString("D. USER TIME PREFERENCES:\n")
	hasPreferences := false
	if len(qctx.PreferredFreeTimes) > 0 {
		sb.WriteString(fmt.Sprintf("- preferred_free_times_list: %s\n", strings.Join(qctx.PreferredFreeTimes, ", ")))
		hasPreferences = true
	}
	if len(qctx.LearningTimePreferences) > 0 {
		sb.WriteString(fmt.Sprintf("- learning_time_preferences: %s\n", strings.Join(qctx.LearningTimePreferences, ", ")))
		hasPreferences = true
	}
	if len(qctx.MovementTimePreferences) > 0 {
		sb.WriteString(fmt.Sprintf("- movement_time_preferences: %s\n", strings.Join(qctx.MovementTimePreferences, ", ")))
		hasPreferences = true
	}
	if qctx.SleepTimePreference != "" {
		sb.WriteString(fmt.Sprintf("- sleep_time_preference: %s\n", qctx.SleepTimePreference))
		hasPreferences = true
	}
	if qctx.NutritionTimePreference != "" {
		sb.WriteString(fmt.Sprintf("- nutrition_time_preference: %s\n", qctx.NutritionTimePreference))
		hasPreferences = true
	}
	if qctx.FreeTimeStart != "" || qctx.FreeTimeEnd != "" {
		sb.WriteString(fmt.Sprintf("- preferred_free_times: %s-%s\n", qctx.FreeTimeStart, qctx.FreeTimeEnd))
		hasPreferences = true
	}
	if qctx.WorkStartTime != "" || qctx.WorkEndTime != "" {
		sb.WriteString(fmt.Sprintf("- work_time: %s-%s\n", qctx.WorkStartTime, qctx.WorkEndTime))
		hasPreferences = true
	}
	if qctx.WakeUpTime != "" {
		sb.WriteString(fmt.Sprintf("- wake_up_time: %s\n", qctx.WakeUpTime))
		hasPreferences = true
	}
	if !hasPreferences {
		sb.WriteString("- none\n")
	}
	sb.WriteString("- NOTE: Time preferences are soft context only. Hard rule windows still win.\n\n")

	// Section E: USER PROFILE, GOALS AND LIMITATIONS
	sb.WriteString("E. USER PROFILE, GOALS AND LIMITATIONS:\n")
	sb.WriteString(fmt.Sprintf("- Age: %d\n", qctx.Age))
	sb.WriteString(fmt.Sprintf("- Height: %.1f cm\n", qctx.Height))
	sb.WriteString(fmt.Sprintf("- Weight: %.1f kg\n", qctx.Weight))
	sb.WriteString(fmt.Sprintf("- MainActivity: %s\n", qctx.MainActivity))
	sb.WriteString(fmt.Sprintf("- ActivityLevel: %s\n", qctx.ActivityLevel))
	sb.WriteString(fmt.Sprintf("- LastWorkout: %s\n", qctx.LastWorkout))
	if len(qctx.MainGoals) > 0 {
		sb.WriteString(fmt.Sprintf("- main_goals: %s\n", strings.Join(qctx.MainGoals, ", ")))
	}
	if len(qctx.HealthLimitations) > 0 {
		sb.WriteString(fmt.Sprintf("- health_limitations: avoid tasks unsafe for %s\n", strings.Join(qctx.HealthLimitations, ", ")))
		sb.WriteString("  * Keep movement tasks gentle, generic, and optional. Prefer walking or light mobility.\n")
		sb.WriteString("  * Wording: Use 'trong mức thoải mái' and 'dừng lại nếu thấy khó chịu'.\n")
		sb.WriteString("  * Do not claim a quest treats, reduces, or improves symptoms (e.g. no 'giảm đau lưng').\n")
		sb.WriteString("  * Do not provide medical, rehab, or injury-specific exercise instructions.\n")
		sb.WriteString("  * Guidance Example - Bad: 'Bài tập lưng giúp giảm đau lưng'. Good: 'Vận động nhẹ 10 phút trong mức thoải mái'.\n")
	} else {
		sb.WriteString("- health_limitations: none\n")
	}
	sb.WriteString("\n")

	// Section F: RUNTIME CONTEXT
	sb.WriteString("F. RUNTIME CONTEXT:\n")
	if qctx.TodayCheckIn != nil {
		sb.WriteString("- Today's Check-in:\n")
		sb.WriteString(fmt.Sprintf("  * Mood: %s\n", qctx.TodayCheckIn.Mood))
		sb.WriteString(fmt.Sprintf("  * Energy Level: %s\n", qctx.TodayCheckIn.EnergyLevel))
		sb.WriteString(fmt.Sprintf("  * Availability: %s\n", qctx.TodayCheckIn.Availability))
		sb.WriteString(fmt.Sprintf("  * Today's Priority: %s\n", qctx.TodayCheckIn.Priority))
	} else {
		sb.WriteString("- Today's Check-in: not checked-in yet\n")
	}

	if qctx.PreviousDailyReview != nil {
		sb.WriteString("- Yesterday's Daily Review:\n")
		sb.WriteString(fmt.Sprintf("  * Completion Rate: %.1f%%\n", qctx.PreviousDailyReview.CompletionRate*100))
		sb.WriteString(fmt.Sprintf("  * Completed Quests Count: %d\n", qctx.PreviousDailyReview.CompletedQuestCount))
		sb.WriteString(fmt.Sprintf("  * Skipped Quests Count: %d\n", qctx.PreviousDailyReview.SkippedQuestCount))
	} else {
		sb.WriteString("- Yesterday's Daily Review: none\n")
	}

	if qctx.ActiveLearningPath != nil {
		sb.WriteString("- Active Learning Path:\n")
		sb.WriteString(fmt.Sprintf("  * Roadmap: %s\n", qctx.ActiveLearningPath.RoadmapTitle))
		sb.WriteString(fmt.Sprintf("  * Current Step: %s\n", qctx.ActiveLearningPath.CurrentStepTitle))
		sb.WriteString(fmt.Sprintf("  * Step Description: %s\n", qctx.ActiveLearningPath.Description))
	} else {
		sb.WriteString("- Active Learning Path: none\n")
	}
	sb.WriteString("\n")

	sb.WriteString("Constraints:\n")
	sb.WriteString(fmt.Sprintf("- Generate at most %d quest objects. The daily_quest_count is a target/maximum limit, not mandatory. You may return fewer quests if it is late evening or if there is no active learning path to support high quality learning quests.\n", questCount))
	sb.WriteString("- Use only enabled categories.\n")
	sb.WriteString("- Use only enabled rules.\n")
	reviewEnabled := HasReviewEnabled(qctx.EnabledCategories)
	if reviewEnabled {
		sb.WriteString("- Review/daily_review is enabled. You MUST include at least one review quest.\n")
	}
	sb.WriteString("\nReturn the JSON quest list now.")

	return sb.String()
}

// Debug helper to view the full prompt context (without exposing secrets)
func (b *PromptBuilder) FormatContextForLogging(qctx *UserQuestContext) string {
	if qctx == nil {
		return "nil context"
	}

	data := map[string]interface{}{
		"local_date":            timeutil.FormatDateVN(qctx.LocalDate),
		"timezone":              qctx.Timezone,
		"daily_quest_count":     qctx.DailyQuestCount,
		"enabled_categories":    qctx.EnabledCategories,
		"rules_count":           len(qctx.Rules),
		"existing_quests_count": len(qctx.ExistingQuestTitles),
		"learning_time_prefs":   qctx.LearningTimePreferences,
		"movement_time_prefs":   qctx.MovementTimePreferences,
		"quiet_after_time":      qctx.QuietAfterTime,
	}

	jsonData, _ := json.MarshalIndent(data, "", "  ")
	return string(jsonData)
}
