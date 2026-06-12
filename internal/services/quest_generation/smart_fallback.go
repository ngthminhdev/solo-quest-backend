package quest_generation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

// fallbackTemplate is a concrete, context-safe quest used to fill a missing
// composition slot. Templates never produce water and never invent learning
// topics without an active roadmap step. Each template carries an explicit
// EstMinutes that must be preserved on the saved quest (never overwritten by a
// generic duration default).
type fallbackTemplate struct {
	normType      string // movement | learning | review | sleep | breakTime
	title         string
	description   string
	difficulty    string
	estMinutes    int
	source        string
	attachRoadmap bool // attach learning metadata for roadmap-linked quests
}

// fallbackReminderMinGap is the minimum spacing enforced between the reminder
// times of generated (and existing) quests so two pending quests never share a
// slot.
const fallbackReminderMinGap = 15 * time.Minute

// BuildSmartFallbackQuests fills up to `missing` quests from the composition
// plan, deterministically and context-aware. It:
//   - never generates water (cap 0),
//   - respects per-type remaining caps (plan_cap - existing - already-kept),
//   - prefers roadmap learning quests when an active roadmap step exists,
//   - avoids duplicate titles against existing + kept quests,
//   - picks an alternative template from the type's pool when the first is taken,
//   - preserves each template's explicit estimated_minutes,
//   - allocates non-colliding reminder times.
//
// It returns quests WITHOUT IDs (the caller persists them). It may return fewer
// than `missing` when caps are exhausted; LastResortFill covers the remainder.
func BuildSmartFallbackQuests(qctx *UserQuestContext, plan QuestCompositionPlan, kept []models.Quest, missing int, now time.Time) ([]models.Quest, error) {
	if qctx == nil || missing <= 0 {
		return nil, nil
	}

	counts := currentTypeCounts(qctx, kept)
	seen := seenTitleSet(qctx, kept)
	alloc := newReminderAllocator(qctx, kept)
	allowed := allowedTypeSet(plan.AllowedTypes)
	pools := fallbackPoolsByType(qctx, plan)
	cursor := make(map[string]int)

	priority := smartFillPriority(qctx)

	var out []models.Quest
	// Diverse, cap-respecting passes: cycle through the priority order adding one
	// quest at a time until the target is met or no type can contribute.
	for len(out) < missing {
		progressed := false
		for _, typ := range priority {
			if len(out) >= missing {
				break
			}
			if !allowed[typ] {
				continue
			}
			if remainingCap(plan.Caps, typ, counts) <= 0 {
				continue
			}
			tpl, ok := nextTemplate(pools[typ], cursor, typ, seen)
			if !ok {
				continue
			}
			q, err := buildFallbackQuest(qctx, tpl, len(kept)+len(out), now, alloc)
			if err != nil {
				return nil, err
			}
			out = append(out, q)
			counts[typ]++
			seen[strings.ToLower(strings.TrimSpace(tpl.title))] = true
			progressed = true
		}
		if !progressed {
			break
		}
	}

	return out, nil
}

// BuildLastResortFill fills the remaining `missing` slots after the smart
// fallback. It first tries cap-respecting diverse fills, then — only to reach the
// daily target — adds extra soft-type quests (review / break) using distinct
// semantic alternative templates. It never exceeds a hard cap, never adds a type
// whose cap is already consumed by EXISTING quests, never generates water, and
// never disambiguates titles by suffixing "(2)". It may return fewer than
// `missing` if no non-capped alternatives remain.
func BuildLastResortFill(qctx *UserQuestContext, plan QuestCompositionPlan, kept []models.Quest, missing int, now time.Time) ([]models.Quest, error) {
	if qctx == nil || missing <= 0 {
		return nil, nil
	}

	counts := currentTypeCounts(qctx, kept)
	seen := seenTitleSet(qctx, kept)
	alloc := newReminderAllocator(qctx, kept)
	allowed := allowedTypeSet(plan.AllowedTypes)
	pools := fallbackPoolsByType(qctx, plan)
	cursor := make(map[string]int)

	var out []models.Quest
	seq := func() int { return len(kept) + len(out) }

	// Tier 1: cap-respecting diverse fill (covers the case where this is called
	// without a prior smart-fallback pass).
	for len(out) < missing {
		progressed := false
		for _, typ := range smartFillPriority(qctx) {
			if len(out) >= missing {
				break
			}
			if !allowed[typ] || remainingCap(plan.Caps, typ, counts) <= 0 {
				continue
			}
			tpl, ok := nextTemplate(pools[typ], cursor, typ, seen)
			if !ok {
				continue
			}
			q, err := buildFallbackQuest(qctx, tpl, seq(), now, alloc)
			if err != nil {
				return nil, err
			}
			out = append(out, q)
			counts[typ]++
			seen[strings.ToLower(strings.TrimSpace(tpl.title))] = true
			progressed = true
		}
		if !progressed {
			break
		}
	}

	// Tier 2: reach the target by adding extra soft-type quests (review, then
	// break) beyond their per-day cap, using distinct semantic alternatives. This
	// ONLY uses types that are enabled quest categories for the user (`allowed`):
	// reminder-only / disabled categories (e.g. breakTime, which is a notification
	// habit, or a disabled review) are never turned into quests just to hit the
	// count. A type whose cap is already consumed by EXISTING quests is skipped
	// too. Movement/sleep/learning are never padded, so movement is never
	// duplicated. If no enabled soft type can fill, the day stays below target
	// rather than inventing off-plan quests.
	for len(out) < missing {
		progressed := false
		for _, typ := range []string{"review", "breakTime"} {
			if len(out) >= missing {
				break
			}
			if !allowed[typ] || isHardLocked(plan, qctx, typ) {
				continue
			}
			tpl, ok := nextTemplate(pools[typ], cursor, typ, seen)
			if !ok {
				continue
			}
			q, err := buildFallbackQuest(qctx, tpl, seq(), now, alloc)
			if err != nil {
				return nil, err
			}
			out = append(out, q)
			counts[typ]++
			seen[strings.ToLower(strings.TrimSpace(tpl.title))] = true
			progressed = true
		}
		if !progressed {
			break
		}
	}

	return out, nil
}

// smartFillPriority is the diversity order for cap-respecting fills. Roadmap
// learning comes first when an active roadmap step exists.
func smartFillPriority(qctx *UserQuestContext) []string {
	hasRoadmap := qctx != nil && qctx.ActiveLearningPath != nil && qctx.ActiveLearningPath.StepID != ""
	if hasRoadmap {
		return []string{"learning", "movement", "review", "sleep", "breakTime"}
	}
	return []string{"movement", "review", "sleep", "breakTime", "learning"}
}

// fallbackPoolsByType returns the ordered alternative templates per quest type.
// Each pool entry has a distinct title and an explicit estimated-minutes value.
// learning is populated only when an active roadmap step exists (a generic
// learning quest must never invent a topic).
func fallbackPoolsByType(qctx *UserQuestContext, plan QuestCompositionPlan) map[string][]fallbackTemplate {
	pools := map[string][]fallbackTemplate{
		"review": {
			{normType: "review", title: "Ghi lại 1 việc đã hoàn thành", description: "Viết nhanh một việc nhỏ bạn đã làm được hôm nay.", difficulty: "easy", estMinutes: 5, source: "daily_review"},
			{normType: "review", title: "Chọn 1 việc cần cải thiện ngày mai", description: "Nghĩ về một điều bạn muốn làm tốt hơn vào ngày mai.", difficulty: "easy", estMinutes: 5, source: "daily_review"},
			{normType: "review", title: "Viết 1 dòng cảm nhận về hôm nay", description: "Ghi lại một câu ngắn về cảm xúc của bạn hôm nay.", difficulty: "easy", estMinutes: 3, source: "daily_review"},
			{normType: "review", title: "Chọn 1 việc nhỏ cho ngày mai", description: "Đặt ra một việc nhỏ, dễ làm cho ngày mai.", difficulty: "easy", estMinutes: 3, source: "daily_review"},
			{normType: "review", title: "Ghi lại 1 điều bạn biết ơn hôm nay", description: "Viết nhanh một điều nhỏ khiến bạn thấy ổn hôm nay.", difficulty: "easy", estMinutes: 3, source: "daily_review"},
			{normType: "review", title: "Nhìn lại 3 điều đã làm hôm nay", description: "Liệt kê nhanh 3 việc bạn đã hoàn thành trong hôm nay.", difficulty: "easy", estMinutes: 5, source: "daily_review"},
		},
		"breakTime": {
			{normType: "breakTime", title: "Nghỉ mắt 1 phút", description: "Rời mắt khỏi màn hình và nhìn xa trong 1 phút.", difficulty: "easy", estMinutes: 1, source: "screen_rest"},
			{normType: "breakTime", title: "Hít thở sâu 1 phút", description: "Hít vào thật chậm và thở ra nhẹ nhàng trong 1 phút.", difficulty: "easy", estMinutes: 1, source: "screen_rest"},
			{normType: "breakTime", title: "Đứng dậy vươn vai 2 phút", description: "Đứng dậy, vươn vai và thả lỏng cơ thể trong 2 phút.", difficulty: "easy", estMinutes: 2, source: "screen_rest"},
			{normType: "breakTime", title: "Thả lỏng vai gáy 2 phút", description: "Xoay nhẹ vai và gáy để giảm căng cứng trong 2 phút.", difficulty: "easy", estMinutes: 2, source: "screen_rest"},
			{normType: "breakTime", title: "Nhắm mắt thư giãn 1 phút", description: "Nhắm mắt, hít thở đều và thư giãn trong 1 phút.", difficulty: "easy", estMinutes: 1, source: "screen_rest"},
			{normType: "breakTime", title: "Rời màn hình nhìn ra xa 1 phút", description: "Nhìn ra cửa sổ hoặc một điểm xa trong 1 phút.", difficulty: "easy", estMinutes: 1, source: "screen_rest"},
		},
		"movement": {
			{normType: "movement", title: "Giãn vai nhẹ 2 phút", description: "Xoay vai, thả lỏng cổ và đứng dậy đi vài bước nhẹ.", difficulty: "easy", estMinutes: 5, source: "health_goal"},
			{normType: "movement", title: "Đi bộ nhẹ vài bước", description: "Đứng dậy và đi lại nhẹ nhàng trong vài phút.", difficulty: "easy", estMinutes: 5, source: "health_goal"},
			{normType: "movement", title: "Xoay cổ tay và cổ chân", description: "Xoay nhẹ cổ tay, cổ chân để cơ thể linh hoạt hơn.", difficulty: "easy", estMinutes: 5, source: "health_goal"},
			{normType: "movement", title: "Đứng dậy vận động nhẹ 5 phút", description: "Đứng dậy, đi lại và vận động nhẹ trong 5 phút.", difficulty: "easy", estMinutes: 5, source: "health_goal"},
		},
		"sleep": {
			{normType: "sleep", title: "Chuẩn bị ngủ gọn nhẹ", description: "Tắt bớt màn hình hoặc dọn nhanh chỗ ngủ trong 5 phút.", difficulty: "easy", estMinutes: 5, source: "sleep_goal"},
			{normType: "sleep", title: "Tắt thiết bị trước khi ngủ", description: "Đặt điện thoại xuống và giảm ánh sáng trước khi ngủ.", difficulty: "easy", estMinutes: 10, source: "sleep_goal"},
			{normType: "sleep", title: "Thư giãn nhẹ trước khi ngủ", description: "Hít thở chậm và thư giãn để dễ vào giấc ngủ.", difficulty: "easy", estMinutes: 10, source: "sleep_goal"},
		},
	}

	if qctx != nil && qctx.ActiveLearningPath != nil && qctx.ActiveLearningPath.StepID != "" {
		step := strings.TrimSpace(qctx.ActiveLearningPath.CurrentStepTitle)
		if step == "" {
			step = strings.TrimSpace(qctx.ActiveLearningPath.RoadmapTitle)
		}
		if step != "" {
			// Complementary angles on the SAME active step. All link to the current
			// roadmap step (correct metadata), letting learning fill multiple slots
			// when it is the user's main enabled category.
			pools["learning"] = []fallbackTemplate{
				{normType: "learning", title: fmt.Sprintf("Học tiếp: %s", step), description: fmt.Sprintf("Dành 10 phút xử lý một phần nhỏ trong bước %s và ghi lại 1 ý chính.", step), difficulty: "easy", estMinutes: 10, source: "learning_roadmap", attachRoadmap: true},
				{normType: "learning", title: fmt.Sprintf("Ôn lại: %s", step), description: fmt.Sprintf("Ghi lại 3 ý chính hoặc 1 lỗi thường gặp trong bước %s.", step), difficulty: "easy", estMinutes: 5, source: "learning_roadmap", attachRoadmap: true},
				{normType: "learning", title: fmt.Sprintf("Thực hành 1 ví dụ: %s", step), description: fmt.Sprintf("Làm 1 ví dụ nhỏ hoặc bài tập áp dụng cho bước %s.", step), difficulty: "easy", estMinutes: 10, source: "learning_roadmap", attachRoadmap: true},
				{normType: "learning", title: fmt.Sprintf("Tóm tắt ý chính: %s", step), description: fmt.Sprintf("Viết tóm tắt ngắn những gì bạn hiểu được về bước %s.", step), difficulty: "easy", estMinutes: 5, source: "learning_roadmap", attachRoadmap: true},
			}
		}
	}

	return pools
}

// nextTemplate returns the next not-yet-used template from a type's pool,
// advancing the per-type cursor past entries whose title is already seen.
func nextTemplate(pool []fallbackTemplate, cursor map[string]int, typ string, seen map[string]bool) (fallbackTemplate, bool) {
	for cursor[typ] < len(pool) {
		tpl := pool[cursor[typ]]
		cursor[typ]++
		if seen[strings.ToLower(strings.TrimSpace(tpl.title))] {
			continue
		}
		return tpl, true
	}
	return fallbackTemplate{}, false
}

// buildFallbackQuest constructs a pending Quest from a template, preserving the
// template's explicit estimated_minutes, attaching roadmap source/metadata for
// roadmap-linked quests, and allocating a non-colliding reminder time.
func buildFallbackQuest(qctx *UserQuestContext, tpl fallbackTemplate, seq int, now time.Time, alloc *reminderAllocator) (models.Quest, error) {
	today := timeutil.StartOfDayVN(qctx.LocalDate)
	dueDate := today

	reminder := alloc.assign(qctx, tpl.normType, seq, now)

	tags := FilterCanonicalTags(nil, tpl.normType)
	tagsJSON := []byte("[]")
	if len(tags) > 0 {
		if b, mErr := json.Marshal(tags); mErr == nil {
			tagsJSON = b
		}
	}

	source := models.QuestSourceConfigBased
	var learningMeta datatypes.JSON
	if tpl.attachRoadmap {
		if meta := BuildLearningMetadata(qctx.ActiveLearningPath); len(meta) > 0 {
			// Roadmap-linked fallback: label it learning_roadmap (not configBased)
			// and store roadmap/step linkage so completion can update progress.
			source = models.QuestSourceLearningRoadmap
			learningMeta = meta
		}
	}

	q := models.Quest{
		Title:            tpl.title,
		Description:      tpl.description,
		Type:             mapNormTypeToQuestType(tpl.normType),
		Status:           models.QuestStatusPending,
		Difficulty:       mapDifficulty(tpl.difficulty),
		Source:           source,
		XPReward:         XPForDifficulty(tpl.difficulty),
		EstimatedMinutes: fallbackEstimatedMinutes(tpl.estMinutes),
		Reason:           "Nhiệm vụ này giúp bạn hoàn thành mục tiêu nhỏ trong hôm nay.",
		Instruction:      defaultInstructionFor(QuestCandidate{Type: tpl.normType}),
		Tags:             datatypes.JSON(tagsJSON),
		Date:             today,
		DueDate:          &dueDate,
		ReminderTime:     &reminder,
		LearningMetadata: learningMeta,
	}
	return q, nil
}

// fallbackEstimatedMinutes preserves an authored template duration. Unlike
// ClampEstimatedMinutes it does NOT enforce a minimum (templates such as
// "Nghỉ mắt 1 phút" intentionally use 1 minute); it only guards against
// non-positive values and an absurd upper bound.
func fallbackEstimatedMinutes(m int) int {
	if m <= 0 {
		return MinEstimatedMinutes
	}
	if m > MaxEstimatedMinutes {
		return MaxEstimatedMinutes
	}
	return m
}

// ---------------------------------------------------------------------------
// Reminder time allocation
// ---------------------------------------------------------------------------

// reminderAllocator hands out reminder times that never collide (within
// fallbackReminderMinGap) with each other or with existing quests. It is seeded
// with existing quests' reminder times and the reminder times of quests already
// kept in this run.
type reminderAllocator struct {
	used []time.Time
}

func newReminderAllocator(qctx *UserQuestContext, kept []models.Quest) *reminderAllocator {
	a := &reminderAllocator{}
	if qctx != nil {
		a.used = append(a.used, qctx.ExistingReminderTimes...)
	}
	for _, q := range kept {
		if q.ReminderTime != nil {
			a.used = append(a.used, *q.ReminderTime)
		}
	}
	return a
}

// assign computes a preferred reminder time and window for the given quest type,
// then reserves the nearest free slot honoring the minimum gap.
func (a *reminderAllocator) assign(qctx *UserQuestContext, normType string, seq int, now time.Time) time.Time {
	today := timeutil.StartOfDayVN(qctx.LocalDate)
	isToday := today.Equal(timeutil.StartOfDayVN(now))
	_, sleepReminder, _ := CalculateSleepTimes(qctx.LocalDate, qctx.TargetSleepTime)

	switch normType {
	case "sleep":
		pref := sleepReminder
		if pref.IsZero() {
			pref = atVN(today, 22, 30)
		}
		if isToday && pref.Before(now) {
			pref = now.Add(5 * time.Minute)
		}
		return a.reserve(pref, pref.Add(-60*time.Minute), pref.Add(15*time.Minute))
	case "review":
		pref := atVN(today, 21, 30)
		latest := atVN(today, 22, 30)
		if !sleepReminder.IsZero() {
			pref = sleepReminder.Add(-30 * time.Minute)
			latest = sleepReminder.Add(-5 * time.Minute)
		}
		if isToday && pref.Before(now) {
			pref = now.Add(5 * time.Minute)
			if !sleepReminder.IsZero() && pref.After(latest) {
				pref = latest
			}
		}
		return a.reserve(pref, latest.Add(-120*time.Minute), latest)
	default:
		cutoff := GetCutoffTimeForNormalQuests(qctx.LocalDate, qctx)
		var base time.Time
		if isToday {
			base = NextSafeTimeSlot(now)
		} else {
			base = atVN(today, 9, 0)
		}
		latest := cutoff.Add(-1 * time.Minute)
		pref := base.Add(time.Duration(seq) * 30 * time.Minute)
		if !pref.Before(cutoff) {
			pref = latest
		}
		return a.reserve(pref, base, latest)
	}
}

// reserve picks the nearest slot to pref within [earliest, latest] that is at
// least fallbackReminderMinGap from every already-used time, searching forward
// then backward. As a last resort (window saturated) it returns pref clamped
// into the window.
func (a *reminderAllocator) reserve(pref, earliest, latest time.Time) time.Time {
	if latest.Before(earliest) {
		earliest, latest = latest, earliest
	}
	if pref.Before(earliest) {
		pref = earliest
	}
	if pref.After(latest) {
		pref = latest
	}
	const step = 5 * time.Minute
	for t := pref; !t.After(latest); t = t.Add(step) {
		if a.free(t) {
			a.used = append(a.used, t)
			return t
		}
	}
	for t := pref.Add(-step); !t.Before(earliest); t = t.Add(-step) {
		if a.free(t) {
			a.used = append(a.used, t)
			return t
		}
	}
	a.used = append(a.used, pref)
	return pref
}

func (a *reminderAllocator) free(t time.Time) bool {
	for _, u := range a.used {
		d := t.Sub(u)
		if d < 0 {
			d = -d
		}
		if d < fallbackReminderMinGap {
			return false
		}
	}
	return true
}

func atVN(day time.Time, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, timeutil.LocationVN)
}

// ---------------------------------------------------------------------------
// Cap / count helpers
// ---------------------------------------------------------------------------

// normalizedTypeCounts re-keys a raw quest-type count map by normalized quest
// type (e.g. break_time -> breakTime, daily_review -> review).
func normalizedTypeCounts(raw map[string]int) map[string]int {
	counts := make(map[string]int, len(raw))
	for t, c := range raw {
		counts[NormalizeType(t)] += c
	}
	return counts
}

// currentTypeCounts returns the per-type counts of existing quests (any status)
// plus the quests already kept in this run, keyed by normalized type.
func currentTypeCounts(qctx *UserQuestContext, kept []models.Quest) map[string]int {
	counts := normalizedTypeCounts(qctx.ExistingQuestTypeCount)
	for _, q := range kept {
		counts[NormalizeType(string(q.Type))]++
	}
	return counts
}

func seenTitleSet(qctx *UserQuestContext, kept []models.Quest) map[string]bool {
	seen := make(map[string]bool)
	for _, title := range qctx.ExistingQuestTitles {
		seen[strings.ToLower(strings.TrimSpace(title))] = true
	}
	for _, q := range kept {
		seen[strings.ToLower(strings.TrimSpace(q.Title))] = true
	}
	return seen
}

// remainingCap returns plan_cap[type] minus the current count of that type
// (existing + kept). A negative cap means unlimited. Unknown types default to a
// cap of 1.
func remainingCap(caps map[string]int, normType string, counts map[string]int) int {
	c, ok := caps[normType]
	if !ok {
		c = 1
	}
	if c < 0 {
		return 1 << 30
	}
	rem := c - counts[normType]
	if rem < 0 {
		return 0
	}
	return rem
}

// isHardLocked reports whether a type's cap is already fully consumed by
// EXISTING quests (those present before this generation run). Such a type must
// never receive a new quest, even by the count-guaranteeing last-resort tier —
// this is what stops a second movement quest when one already exists today.
func isHardLocked(plan QuestCompositionPlan, qctx *UserQuestContext, normType string) bool {
	existing := normalizedTypeCounts(qctx.ExistingQuestTypeCount)[normType]
	c, ok := plan.Caps[normType]
	if !ok {
		c = 1
	}
	if c < 0 {
		return false
	}
	return existing >= c
}

func allowedTypeSet(allowedTypes []string) map[string]bool {
	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		allowed[NormalizeType(t)] = true
	}
	return allowed
}

func mapNormTypeToQuestType(normType string) models.QuestType {
	switch normType {
	case "movement":
		return models.QuestTypeMovement
	case "learning":
		return models.QuestTypeLearning
	case "sleep":
		return models.QuestTypeSleep
	case "review":
		return models.QuestTypeReview
	case "breakTime":
		return models.QuestTypeBreak
	default:
		return models.QuestType(normType)
	}
}

func mapDifficulty(difficulty string) models.QuestDifficulty {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return models.QuestDifficultyEasy
	case "hard":
		return models.QuestDifficultyHard
	default:
		return models.QuestDifficultyMedium
	}
}
