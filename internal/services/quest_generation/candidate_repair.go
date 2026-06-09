package quest_generation

import (
	"fmt"
	"strings"
	"time"

	"solo_quest_backend/internal/pkg/timeutil"
)

// CandidateRepairReport summarizes what the per-candidate quality gate did to a
// raw AI batch. It is used for backend logging/debugging only and never leaves
// the backend (no FE contract impact).
type CandidateRepairReport struct {
	RawCount      int
	KeptCount     int
	RepairedCount int
	DroppedCount  int
	// DropReasons lists, per dropped candidate, a short reason. Capped for log
	// hygiene by the caller.
	DropReasons []string
	// RepairNotes lists short notes about repairs applied to kept candidates.
	RepairNotes []string
}

func (r *CandidateRepairReport) topDropReasons(limit int) string {
	if len(r.DropReasons) == 0 {
		return ""
	}
	if limit > 0 && len(r.DropReasons) > limit {
		return strings.Join(r.DropReasons[:limit], "; ")
	}
	return strings.Join(r.DropReasons, "; ")
}

// dangerWords mirrors the strict validator's unsafe/medical keyword guard.
var dangerWords = []string{
	"suicide", "tự tử", "chết", "nguy hiểm", "extreme", "hurt",
	"doctor", "bác sĩ", "medicine", "thuốc",
}

// learningSpecificKeywords are topics a generic learning quest must not invent
// when there is no active learning path.
var learningSpecificKeywords = []string{
	"english", "tiếng anh", "vocabulary", "từ vựng", "coding", "lập trình",
	"grammar", "ngữ pháp", "book", "sách", "đọc sách", "toán", "math",
}

// RepairAndValidateCandidates replaces the previous all-or-nothing validation
// with a per-candidate quality gate:
//
//   - Repairable issues (wrong/missing xp_reward, out-of-range estimated_minutes,
//     unknown tags, empty description/instruction/reason, missing/invalid or
//     slightly out-of-window reminder_time) are fixed in place.
//   - Unrecoverable issues (invalid/empty type, reminder-only type, disabled
//     category/rule, empty/vague title, duplicate title, unsafe content,
//     un-repairable reminder_time, learning hard-rule violation) drop only that
//     candidate.
//
// The batch as a whole never fails here: callers decide what to do with an empty
// result (the generation service falls back to rule-based). It returns the kept
// (repaired) candidates and a report for logging.
func RepairAndValidateCandidates(qctx *UserQuestContext, candidates []QuestCandidate, now time.Time) ([]QuestCandidate, *CandidateRepairReport) {
	report := &CandidateRepairReport{RawCount: len(candidates)}
	if qctx == nil || len(candidates) == 0 {
		return nil, report
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

	existingTitles := make(map[string]bool)
	for _, title := range qctx.ExistingQuestTitles {
		existingTitles[strings.ToLower(strings.TrimSpace(title))] = true
	}
	usedTitles := make(map[string]bool)

	// Per-type aggregate caps, enforced by dropping extras (keep first).
	typeKept := make(map[string]int)
	movementMax := 1
	if rule, ok := rulesByType["movement"]; ok && rule.MaxPerDay != nil && *rule.MaxPerDay > 0 {
		movementMax = *rule.MaxPerDay
	}
	hasLearningPath := qctx.ActiveLearningPath != nil

	drop := func(c QuestCandidate, reason string) {
		report.DroppedCount++
		title := strings.TrimSpace(c.Title)
		if title == "" {
			title = "(no title)"
		}
		report.DropReasons = append(report.DropReasons, fmt.Sprintf("%s: %s", title, reason))
	}

	kept := make([]QuestCandidate, 0, len(candidates))

	for _, c := range candidates {
		normType := NormalizeType(c.Type)
		repaired := false

		// --- Hard drops: type ---
		if strings.TrimSpace(c.Type) == "" {
			drop(c, "empty type")
			continue
		}
		if IsReminderOnlyDailyType(c.Type) {
			drop(c, fmt.Sprintf("reminder-only type '%s' not allowed as daily quest", c.Type))
			continue
		}
		if !allowedQuestTypes[normType] && !allowedQuestTypes[c.Type] {
			drop(c, fmt.Sprintf("invalid quest type '%s'", c.Type))
			continue
		}
		if !enabledCats[normType] && !enabledCats[c.Type] {
			drop(c, fmt.Sprintf("type '%s' not enabled", c.Type))
			continue
		}
		if rule, ok := rulesByType[normType]; ok && !rule.Enabled {
			drop(c, fmt.Sprintf("rule for type '%s' is disabled", c.Type))
			continue
		}

		// Canonicalize the type so the mapper receives a known value.
		if c.Type != normType {
			c.Type = normType
			repaired = true
		}

		// --- Hard drops: title ---
		title := strings.TrimSpace(c.Title)
		if title == "" {
			drop(c, "empty title")
			continue
		}
		if strings.EqualFold(title, "do something") || strings.EqualFold(title, "làm gì đó") {
			drop(c, "title too vague")
			continue
		}
		if len(title) > 120 {
			title = strings.TrimSpace(title[:120])
			c.Title = title
			repaired = true
		} else if title != c.Title {
			c.Title = title
			repaired = true
		}
		lowerTitle := strings.ToLower(title)
		if usedTitles[lowerTitle] {
			drop(c, "duplicate title in batch")
			continue
		}
		if existingTitles[lowerTitle] {
			drop(c, "title duplicates an existing quest")
			continue
		}

		// --- Hard drop: unsafe/medical content ---
		lowerContent := strings.ToLower(c.Title + " " + c.Description)
		unsafe := false
		for _, w := range dangerWords {
			if strings.Contains(lowerContent, w) {
				drop(c, fmt.Sprintf("unsafe/medical keyword '%s'", w))
				unsafe = true
				break
			}
		}
		if unsafe {
			continue
		}

		// --- Hard drop: learning hard-rule (invented topic without path) ---
		if normType == "learning" && !hasLearningPath {
			invented := false
			for _, kw := range learningSpecificKeywords {
				if strings.Contains(lowerContent, kw) {
					drop(c, fmt.Sprintf("generic learning quest invented specific topic '%s'", kw))
					invented = true
					break
				}
			}
			if invented {
				continue
			}
		}

		// --- Aggregate caps (drop extras) ---
		switch normType {
		case "movement":
			if typeKept["movement"] >= movementMax {
				drop(c, fmt.Sprintf("exceeds movement cap (%d)", movementMax))
				continue
			}
		case "sleep":
			if typeKept["sleep"] >= 1 {
				drop(c, "more than 1 sleep quest")
				continue
			}
		case "review":
			if typeKept["review"] >= 1 {
				drop(c, "more than 1 review quest")
				continue
			}
		case "learning":
			if !hasLearningPath && typeKept["learning"] >= 1 {
				drop(c, "more than 1 learning quest without active learning path")
				continue
			}
		}

		// --- Repairs (never drop on these) ---
		// Difficulty -> canonical
		normDiff := NormalizeDifficulty(c.Difficulty)
		if normDiff != c.Difficulty {
			c.Difficulty = normDiff
			repaired = true
		}
		// XP -> always recompute from difficulty (ignore AI value)
		expectedXP := XPForDifficulty(c.Difficulty)
		if c.XPReward != expectedXP {
			c.XPReward = expectedXP
			repaired = true
		}
		// Estimated minutes -> clamp / default
		clamped := ClampEstimatedMinutes(c.EstimatedMinutes, qctx.PreferredDuration)
		if clamped != c.EstimatedMinutes {
			c.EstimatedMinutes = clamped
			repaired = true
		}
		// Tags -> drop unknown, keep canonical, fall back to type tag
		filteredTags := FilterCanonicalTags(c.Tags, c.Type)
		if !sameStringSlice(filteredTags, c.Tags) {
			c.Tags = filteredTags
			repaired = true
		}
		// Description / instruction / reason -> fill safely if empty
		if strings.TrimSpace(c.Description) == "" {
			c.Description = defaultDescriptionFor(c)
			repaired = true
		}
		if strings.TrimSpace(c.Instruction) == "" {
			c.Instruction = defaultInstructionFor(c)
			repaired = true
		}
		if strings.TrimSpace(c.Reason) == "" {
			c.Reason = defaultReasonFor(c)
			repaired = true
		}

		// --- Reminder time: repair or drop only this candidate ---
		// At this point candidates have already passed through NormalizeCandidates
		// (sleep/review reminders are derived from settings; normal quests are
		// shifted to a safe future slot before the cutoff). We only need to ensure
		// the value is a valid HH:mm and, when slightly outside a rule's active
		// window, clamp it back in.
		repairedTime, ok := repairReminderTime(qctx, c, normType, rulesByType, now)
		if !ok {
			drop(c, "reminder_time could not be repaired")
			continue
		}
		if repairedTime != c.ReminderTime {
			c.ReminderTime = repairedTime
			repaired = true
		}

		// Keep it.
		usedTitles[lowerTitle] = true
		typeKept[normType]++
		if repaired {
			report.RepairedCount++
			report.RepairNotes = append(report.RepairNotes, fmt.Sprintf("%s: repaired", title))
		}
		kept = append(kept, c)
	}

	report.KeptCount = len(kept)
	return kept, report
}

// repairReminderTime validates the reminder time format and, for normal quests
// (not sleep/review, whose reminders are settings-derived), clamps a value that
// is slightly outside the rule's active_time_range back to the nearest boundary.
// It returns the (possibly clamped) "HH:mm" value and whether it is usable.
func repairReminderTime(qctx *UserQuestContext, c QuestCandidate, normType string, rulesByType map[string]QuestRuleContext, now time.Time) (string, bool) {
	rt := strings.TrimSpace(c.ReminderTime)
	if rt == "" {
		return "", false
	}
	if _, err := time.Parse("15:04", rt); err != nil {
		return "", false
	}

	// Sleep/review reminders are authoritative (set by NormalizeCandidates from
	// settings); leave them as-is.
	if normType == "sleep" || normType == "review" {
		return rt, true
	}

	rule, ok := rulesByType[normType]
	if !ok || rule.ActiveTimeRange == nil {
		return rt, true
	}
	start := rule.ActiveTimeRange.Start
	end := rule.ActiveTimeRange.End
	if start == "" || end == "" {
		return rt, true
	}
	// Only handle the common non-crossing-midnight case; for crossing-midnight
	// ranges we conservatively accept the value rather than risk a wrong clamp.
	if start > end {
		return rt, true
	}
	if rt >= start && rt <= end {
		return rt, true
	}

	// Clamp to the nearest boundary inside the window.
	clamped := start
	if rt > end {
		clamped = end
	}

	// If today, the clamped slot must still be in the future; otherwise this
	// candidate cannot be safely placed and is dropped.
	if isSameLocalDate(qctx.LocalDate, now) {
		nowHM := now.Format("15:04")
		if clamped <= nowHM {
			return "", false
		}
	}
	return clamped, true
}

func isSameLocalDate(localDate, now time.Time) bool {
	return timeutil.StartOfDayVN(localDate).Equal(timeutil.StartOfDayVN(now))
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func defaultDescriptionFor(c QuestCandidate) string {
	switch NormalizeType(c.Type) {
	case "movement":
		return "Dành ít phút vận động nhẹ để cơ thể linh hoạt và tỉnh táo hơn."
	case "learning":
		return "Dành thời gian học hoặc ôn lại một nội dung bạn quan tâm hôm nay."
	case "sleep":
		return "Chuẩn bị cho một giấc ngủ ngon và đúng giờ."
	case "review":
		return "Nhìn lại một ngày của bạn và ghi nhận những điều đã làm được."
	default:
		return fmt.Sprintf("Hoàn thành nhiệm vụ: %s.", strings.TrimSpace(c.Title))
	}
}

func defaultInstructionFor(c QuestCandidate) string {
	switch NormalizeType(c.Type) {
	case "movement":
		return "Đứng dậy và vận động nhẹ trong vài phút."
	case "learning":
		return "Tập trung học trong khoảng thời gian đã đặt và ghi lại điều bạn học được."
	case "sleep":
		return "Thư giãn, giảm ánh sáng và chuẩn bị đi ngủ đúng giờ."
	case "review":
		return "Dành vài phút nhìn lại ngày hôm nay và ghi chú ngắn gọn."
	default:
		return "Dành vài phút để hoàn thành nhiệm vụ này."
	}
}

func defaultReasonFor(c QuestCandidate) string {
	return "Nhiệm vụ này giúp bạn duy trì thói quen lành mạnh mỗi ngày."
}
