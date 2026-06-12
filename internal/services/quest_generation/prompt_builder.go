package quest_generation

import (
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
	if len(AllowedAIQuestTypesForContext(qctx)) == 0 {
		return "", "", fmt.Errorf("no enabled quest rules configured")
	}

	systemPrompt = b.buildSystemPrompt(qctx)
	userPrompt = b.buildUserPrompt(qctx)
	return systemPrompt, userPrompt, nil
}

func (b *PromptBuilder) buildSystemPrompt(qctx *UserQuestContext) string {
	plan := BuildQuestCompositionPlan(qctx)

	now := timeutil.NowVN()
	today := timeutil.FormatDateVN(qctx.LocalDate)
	timezone := qctx.Timezone
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	nowLocal := now.Format("15:04")

	allowedTypes := strings.Join(AllowedAIQuestTypesForPrompt(qctx), ", ")

	tpl := DailyQuestSystemPromptTemplate
	tpl = strings.ReplaceAll(tpl, "{{today}}", today)
	tpl = strings.ReplaceAll(tpl, "{{timezone}}", timezone)
	tpl = strings.ReplaceAll(tpl, "{{now_local}}", nowLocal)
	tpl = strings.ReplaceAll(tpl, "{{target_count}}", fmt.Sprintf("%d", plan.TargetCount))
	tpl = strings.ReplaceAll(tpl, "{{existing_count}}", fmt.Sprintf("%d", plan.ExistingCount))
	tpl = strings.ReplaceAll(tpl, "{{needed_count}}", fmt.Sprintf("%d", plan.NeededCount))
	tpl = strings.ReplaceAll(tpl, "{{allowed_types}}", allowedTypes)
	return tpl
}

func (b *PromptBuilder) buildUserPrompt(qctx *UserQuestContext) string {
	var sb strings.Builder

	plan := BuildQuestCompositionPlan(qctx)
	questCount := plan.NeededCount

	weekday := qctx.LocalDate.Weekday()
	weekdayNum := int(weekday)
	if weekdayNum == 0 {
		weekdayNum = 7
	}
	isWorkday := false
	for _, wd := range qctx.WorkWeekdays {
		if wd == weekdayNum {
			isWorkday = true
			break
		}
	}

	// === HỒ SƠ NGƯỜI DÙNG ===
	sb.WriteString("=== HỒ SƠ NGƯỜI DÙNG ===\n")
	sb.WriteString(fmt.Sprintf("- Tên: %s\n", strOrNone(qctx.DisplayName)))
	if len(qctx.MainGoals) > 0 {
		sb.WriteString(fmt.Sprintf("- Mục tiêu chính: %s\n", strings.Join(qctx.MainGoals, ", ")))
	} else {
		sb.WriteString("- Mục tiêu chính: none\n")
	}
	sb.WriteString(fmt.Sprintf("- Mức vận động: %s | Lần tập gần nhất: %s\n",
		strOrNone(qctx.ActivityLevel), strOrNone(qctx.LastWorkout)))
	if len(qctx.HealthLimitations) > 0 {
		sb.WriteString(fmt.Sprintf("- Giới hạn sức khỏe: %s   (TUYỆT ĐỐI tránh động tác gây hại)\n",
			strings.Join(qctx.HealthLimitations, ", ")))
	} else {
		sb.WriteString("- Giới hạn sức khỏe: none\n")
	}
	sb.WriteString("- Chủ đề học: none\n\n")

	// === LỊCH HÔM NAY ===
	sb.WriteString("=== LỊCH HÔM NAY ===\n")
	sb.WriteString(fmt.Sprintf("- Thức dậy: %s | Ngủ mục tiêu: %s\n",
		strOrNone(qctx.WakeUpTime), strOrNone(qctx.TargetSleepTime)))
	if qctx.WorkStartTime != "" || qctx.WorkEndTime != "" {
		sb.WriteString(fmt.Sprintf("- Làm việc: %s–%s\n", strOrNone(qctx.WorkStartTime), strOrNone(qctx.WorkEndTime)))
	} else {
		sb.WriteString("- Làm việc: none\n")
	}
	if qctx.FreeTimeStart != "" || qctx.FreeTimeEnd != "" {
		sb.WriteString(fmt.Sprintf("- Thời gian rảnh: %s–%s\n", strOrNone(qctx.FreeTimeStart), strOrNone(qctx.FreeTimeEnd)))
	} else {
		sb.WriteString("- Thời gian rảnh: none\n")
	}
	learningPref := strOrNone(qctx.LearningTimePreference)
	if len(qctx.LearningTimePreferences) > 0 {
		learningPref = strings.Join(qctx.LearningTimePreferences, ", ")
	}
	movementPref := strOrNone(qctx.MovementTimePreference)
	if len(qctx.MovementTimePreferences) > 0 {
		movementPref = strings.Join(qctx.MovementTimePreferences, ", ")
	}
	sb.WriteString(fmt.Sprintf("- Ưu tiên giờ học: %s | giờ vận động: %s\n", learningPref, movementPref))
	sb.WriteString(fmt.Sprintf("- Loại ngày: %s\n\n", deriveDayType(qctx)))

	// === CHECK-IN HÔM NAY ===
	sb.WriteString("=== CHECK-IN HÔM NAY ===\n")
	if qctx.TodayCheckIn != nil {
		sb.WriteString(fmt.Sprintf("- Tâm trạng: %s | Năng lượng: %s | Mức bận: %s\n",
			qctx.TodayCheckIn.Mood, qctx.TodayCheckIn.EnergyLevel, qctx.TodayCheckIn.Availability))
		sb.WriteString(fmt.Sprintf("- Ưu tiên user chọn: %s\n\n", strOrNone(qctx.TodayCheckIn.Priority)))
	} else {
		sb.WriteString("- Chưa check-in hôm nay\n\n")
	}

	// === ROADMAP ===
	sb.WriteString("=== ROADMAP ===\n")
	if qctx.ActiveLearningPath != nil {
		sb.WriteString("- Có roadmap đang hoạt động: true\n")
		sb.WriteString(fmt.Sprintf("- Tên roadmap: %s\n", qctx.ActiveLearningPath.RoadmapTitle))
		sb.WriteString(fmt.Sprintf("- Danh mục: %s\n", qctx.ActiveLearningPath.RoadmapCategory))
		sb.WriteString(fmt.Sprintf("- Bước hiện tại (id=%s): %s\n", qctx.ActiveLearningPath.StepID, qctx.ActiveLearningPath.CurrentStepTitle))
		sb.WriteString(fmt.Sprintf("- Mô tả bước: %s\n", qctx.ActiveLearningPath.Description))
		if qctx.ActiveLearningPath.StepEstimatedMinutes > 0 {
			sb.WriteString(fmt.Sprintf("- Thời gian ước tính bước này: %d phút\n", qctx.ActiveLearningPath.StepEstimatedMinutes))
		}
		sb.WriteString(fmt.Sprintf("- Tiến độ: %d/%d bước đã hoàn thành\n", qctx.ActiveLearningPath.CompletedSteps, qctx.ActiveLearningPath.TotalSteps))
		sb.WriteString(fmt.Sprintf("- QUAN TRỌNG: Khi tạo learning quest, PHẢI dựa trên bước hiện tại '%s' và đặt roadmap_step_id='%s'\n\n", qctx.ActiveLearningPath.CurrentStepTitle, qctx.ActiveLearningPath.StepID))
	} else {
		sb.WriteString("- Có roadmap đang hoạt động: false\n")
		sb.WriteString("- Nếu tạo learning quest: phải cụ thể, gắn với mục tiêu hoặc lĩnh vực user quan tâm, không được dùng title chung chung bị cấm\n\n")
	}

	// === LỊCH SỬ GẦN ĐÂY ===
	sb.WriteString("=== LỊCH SỬ GẦN ĐÂY (để tạo sự ĐA DẠNG, đừng lặp lại) ===\n")
	sb.WriteString("- Quest user hay BỎ QUA: none\n")
	sb.WriteString("- Quest đã làm hôm qua: none\n")
	if qctx.PreviousDailyReview != nil {
		sb.WriteString(fmt.Sprintf("- Hôm qua hoàn thành: %.0f%%\n\n", qctx.PreviousDailyReview.CompletionRate*100))
	} else {
		sb.WriteString("- Hôm qua hoàn thành: none\n\n")
	}

	// === QUEST ĐÃ CÓ HÔM NAY ===
	sb.WriteString("=== QUEST ĐÃ CÓ HÔM NAY (đừng tạo trùng nghĩa với những cái này) ===\n")
	if len(qctx.ExistingQuestTitles) > 0 {
		for _, title := range qctx.ExistingQuestTitles {
			sb.WriteString(fmt.Sprintf("  * %s\n", title))
		}
	} else {
		sb.WriteString("none\n")
	}
	sb.WriteString("\n")

	// --- RÀNG BUỘC CỨNG ---

	// Section A: GENERATION TARGET (kept for backward compatibility + test assertions)
	sb.WriteString("A. GENERATION TARGET:\n")
	sb.WriteString(fmt.Sprintf("- Date: %s (%s)\n", timeutil.FormatDateVN(qctx.LocalDate), weekday.String()))
	sb.WriteString(fmt.Sprintf("- Current Local Time: %s\n", timeutil.NowVN().Format("2006-01-02T15:04:05-07:00")))
	sb.WriteString(fmt.Sprintf("- Today's Day of Week: %d (1=Mon, 7=Sun)\n", weekdayNum))
	sb.WriteString(fmt.Sprintf("- Is Today a Scheduled Workday?: %t (based on work_weekdays)\n", isWorkday))
	sb.WriteString(fmt.Sprintf("- Is Weekend?: %t\n", qctx.IsWeekend))
	sb.WriteString(fmt.Sprintf("- Rest Day Enabled?: %t\n", qctx.RestDayEnabled))
	if qctx.IsRestDay {
		sb.WriteString("- REST DAY POLICY: Today is a rest day. Generate FEWER quests (3-4 total), prefer easy difficulty and short durations, prioritize sleep / review / gentle movement, and avoid heavy or long learning and work-style productivity tasks.\n")
	}
	if qctx.RequestedPreviewLimit != nil {
		sb.WriteString(fmt.Sprintf("- Requested Preview Limit: %d\n", *qctx.RequestedPreviewLimit))
	} else {
		sb.WriteString("- Requested Preview Limit: nil\n")
	}
	sb.WriteString(fmt.Sprintf("- Effective Preview Limit: %d\n", questCount))
	sb.WriteString(fmt.Sprintf("- Daily Quest Target Count: %d\n", plan.TargetCount))
	sb.WriteString(fmt.Sprintf("- Existing Quest Count: %d\n", plan.ExistingCount))
	sb.WriteString(fmt.Sprintf("- Needed Quest Count: %d\n", plan.NeededCount))
	sb.WriteString(fmt.Sprintf("- Enabled Categories: %s\n", strings.Join(qctx.EnabledCategories, ", ")))
	sb.WriteString(fmt.Sprintf("- allowed_types: %s\n", strings.Join(AllowedAIQuestTypesForPrompt(qctx), ", ")))
	if len(qctx.ExistingQuestTitles) > 0 {
		sb.WriteString("- Existing Quests Today (avoid duplicate titles):\n")
		for _, title := range qctx.ExistingQuestTitles {
			sb.WriteString(fmt.Sprintf("  * %s\n", title))
		}
	} else {
		sb.WriteString("- Existing Quests Today: none\n")
	}
	sb.WriteString("\n")

	sb.WriteString("A2. QUEST COMPOSITION PLAN (follow this plan exactly):\n")
	sb.WriteString(fmt.Sprintf("- needed_count: %d\n", plan.NeededCount))
	sb.WriteString(fmt.Sprintf("- allowed_types: %s\n", strings.Join(AllowedAIQuestTypesForPrompt(qctx), ", ")))
	sb.WriteString("- water_policy: never generate water quests (water max_count is 0)\n")
	if plan.RequireLearningRoadmap {
		sb.WriteString(fmt.Sprintf("- learning_required: true, preferred_learning_count: %d\n", plan.PreferredLearningCount))
		sb.WriteString(fmt.Sprintf("- learning_must_reference_step: %s\n", qctx.ActiveLearningPath.CurrentStepTitle))
	} else {
		sb.WriteString("- learning_required: false\n")
	}
	if plan.EasyAndShort {
		sb.WriteString("- energy_availability_policy: keep every quest easy and short\n")
	}
	if plan.GentleMovement {
		sb.WriteString("- movement_policy: gentle movement only\n")
	}
	sb.WriteString(fmt.Sprintf("- max_break_time_count: %d\n", plan.MaxBreakTimeCount))
	// Per-type caps. The AI must not exceed these in a single day.
	sb.WriteString("- caps (max per day per type):\n")
	for _, t := range []string{"movement", "learning", "review", "sleep", "breakTime", "water"} {
		if c, ok := plan.Caps[t]; ok {
			label := t
			if t == "breakTime" {
				label = "break_time"
			}
			sb.WriteString(fmt.Sprintf("  * %s: %d\n", label, c))
		}
	}
	// Explicit per-slot composition: exactly one quest per slot.
	if len(plan.Slots) > 0 {
		sb.WriteString("- slots (generate exactly one quest per slot, do not exceed caps):\n")
		for i, slot := range plan.Slots {
			typeLabel := slot.Type
			if slot.Type == "breakTime" {
				typeLabel = "break_time"
			}
			line := fmt.Sprintf("  %d. type=%s source=%s", i+1, typeLabel, slot.Source)
			if slot.Required {
				line += " required=true"
			}
			if slot.Style != "" {
				line += " style=" + slot.Style
			}
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("- composition_rules: generate one quest per slot; do not exceed caps; do not fill all quests with the same type; if constraints are tight, create smaller quests, not fewer quests; never return an empty quests array when needed_count > 0.\n\n")

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

	// Section C: HARD RULE WINDOWS
	enabledCats := make(map[string]bool)
	for _, cat := range qctx.EnabledCategories {
		enabledCats[cat] = true
	}
	sb.WriteString("C. HARD RULE WINDOWS:\n")
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

		usableStart, usableEnd := rawStart, rawEnd
		if qctx.QuietAfterTime != "" && usableEnd > qctx.QuietAfterTime {
			usableEnd = qctx.QuietAfterTime
		}
		if usableEnd < usableStart {
			sb.WriteString("  usable_reminder_window: unavailable\n")
			sb.WriteString("  hard_rule: DO NOT generate quests for this category unless there are no alternatives\n")
		} else if usableStart == usableEnd {
			sb.WriteString(fmt.Sprintf("  usable_reminder_window: %s-%s\n", usableStart, usableEnd))
			sb.WriteString(fmt.Sprintf("  hard_rule: reminder_time must be exactly %s\n", usableStart))
		} else {
			sb.WriteString(fmt.Sprintf("  usable_reminder_window: %s-%s\n", usableStart, usableEnd))
			sb.WriteString(fmt.Sprintf("  hard_rule: reminder_time must be between %s and %s\n", usableStart, usableEnd))
		}
	}
	sb.WriteString("Any quest outside its type's window is invalid.\n\n")

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
		sb.WriteString(fmt.Sprintf("  * Roadmap ID: %s\n", qctx.ActiveLearningPath.RoadmapID))
		sb.WriteString(fmt.Sprintf("  * Roadmap: %s\n", qctx.ActiveLearningPath.RoadmapTitle))
		sb.WriteString(fmt.Sprintf("  * Category: %s\n", qctx.ActiveLearningPath.RoadmapCategory))
		sb.WriteString(fmt.Sprintf("  * Current Step ID: %s\n", qctx.ActiveLearningPath.StepID))
		sb.WriteString(fmt.Sprintf("  * Current Step: %s (step %d/%d)\n", qctx.ActiveLearningPath.CurrentStepTitle, qctx.ActiveLearningPath.StepOrderIndex+1, qctx.ActiveLearningPath.TotalSteps))
		sb.WriteString(fmt.Sprintf("  * Step Description: %s\n", qctx.ActiveLearningPath.Description))
		if qctx.ActiveLearningPath.StepEstimatedMinutes > 0 {
			sb.WriteString(fmt.Sprintf("  * Estimated Duration: %d minutes\n", qctx.ActiveLearningPath.StepEstimatedMinutes))
		}
		sb.WriteString(fmt.Sprintf("  * Progress: %d/%d steps completed\n", qctx.ActiveLearningPath.CompletedSteps, qctx.ActiveLearningPath.TotalSteps))
		sb.WriteString(fmt.Sprintf("  * INSTRUCTION: When generating a learning quest, it MUST be based on step '%s' (id=%s). Set roadmap_step_id to this ID.\n", qctx.ActiveLearningPath.CurrentStepTitle, qctx.ActiveLearningPath.StepID))
	} else {
		sb.WriteString("- Active Learning Path: none\n")
	}
	sb.WriteString("\n")

	// Section G: BUSY SCHEDULE BLOCKS
	sb.WriteString("G. BUSY SCHEDULE BLOCKS (do not place quests in these windows):\n")
	hasBusy := false
	for _, block := range qctx.ScheduleBlocks {
		if !block.IsBusy {
			continue
		}
		appliesToday := false
		for _, d := range block.DaysOfWeek {
			if d == weekdayNum {
				appliesToday = true
				break
			}
		}
		if !appliesToday {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s-%s\n", block.Title, block.StartTime, block.EndTime))
		hasBusy = true
	}
	if !hasBusy {
		sb.WriteString("- none\n")
	}
	sb.WriteString("\n")

	// Final constraints
	sb.WriteString(fmt.Sprintf("Constraints:\n- Generate exactly %d quest objects.\n", questCount))
	sb.WriteString("- The quests array must not be empty when needed_count > 0.\n")
	sb.WriteString("- Do not decide that the user has enough tasks.\n")
	sb.WriteString("- If constraints are tight, generate easier or shorter quests, not fewer quests.\n")
	sb.WriteString("- Never generate water quests.\n")
	sb.WriteString("- Use only enabled categories.\n")
	sb.WriteString("- Use only enabled rules.\n")
	if HasReviewEnabled(qctx.EnabledCategories) {
		sb.WriteString("- Review/daily_review is enabled. You MUST include at least one review quest.\n")
	}
	sb.WriteString("\nReturn the JSON quest list now.")

	return sb.String()
}

func strOrNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// deriveDayType infers a human-readable day type from context.
func deriveDayType(qctx *UserQuestContext) string {
	if qctx.IsRestDay {
		return "rest"
	}
	if qctx.TodayCheckIn != nil {
		if qctx.TodayCheckIn.EnergyLevel == "low" || qctx.TodayCheckIn.EnergyLevel == "very_low" {
			return "low_energy"
		}
		if qctx.TodayCheckIn.Availability == "busy" {
			return "busy"
		}
	}
	return "normal"
}
