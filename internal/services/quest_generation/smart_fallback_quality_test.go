package quest_generation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
)

// qctxWith builds a minimal manual context enabling the given types (with wide
// rules so they are always allowed) for the deterministic tomorrow date.
func qctxWith(types ...string) *UserQuestContext {
	rules := make([]QuestRuleContext, 0, len(types))
	for _, t := range types {
		rules = append(rules, wideRule(t))
	}
	return &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		PreferredDuration: "medium",
		TargetSleepTime:   "23:00",
		EnabledCategories: types,
		Rules:             rules,
	}
}

// TestSmartFallback_ExistingMovementCapBlocksNewMovement covers requirement #1:
// an existing movement quest consumes the movement cap (1), so neither the smart
// fallback nor the last-resort fill may add another movement quest.
func TestSmartFallback_ExistingMovementCapBlocksNewMovement(t *testing.T) {
	qctx := qctxWith("movement", "review", "sleep", "breakTime")
	qctx.ExistingQuestTypeCount = map[string]int{"movement": 1}
	plan := planForContext(qctx, 6)

	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 6, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	lr, err := BuildLastResortFill(qctx, plan, smart, 6-len(smart), time.Now())
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	for _, q := range append(smart, lr...) {
		if q.Type == models.QuestTypeMovement {
			t.Fatalf("fallback added a movement quest despite existing movement cap consumed: %+v", q)
		}
	}
}

// TestQualityGate_ExistingCompletedCountsTowardCap covers requirement #7: an
// existing (completed) quest of a type consumes its cap in the AI quality gate,
// so a new AI candidate of that type is dropped.
func TestQualityGate_ExistingCompletedCountsTowardCap(t *testing.T) {
	qctx := qctxWith("movement", "review")
	// Simulate a movement quest already completed earlier today.
	qctx.ExistingQuestTypeCount = map[string]int{"movement": 1}

	candidates := []QuestCandidate{
		{Type: "movement", Title: "Đi bộ 10 phút", Description: "Vận động nhẹ", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, Reason: "x", Instruction: "y", ReminderTime: "10:00"},
		{Type: "review", Title: "Nhìn lại hôm nay", Description: "Ôn nhanh", Difficulty: "easy", EstimatedMinutes: 5, Tags: []string{"review"}, Reason: "x", Instruction: "y", ReminderTime: "21:00"},
	}
	kept, _ := RepairAndValidateCandidates(qctx, candidates, time.Now())
	for _, c := range kept {
		if NormalizeType(c.Type) == "movement" {
			t.Fatalf("quality gate kept a movement candidate although existing movement consumed the cap: %+v", c)
		}
	}
}

// TestFallback_BreakTemplatePreservesOneMinute covers requirement #3: the
// "Nghỉ mắt 1 phút" template must keep estimated_minutes=1 (not be clamped to the
// generic minimum or overwritten by a duration default).
func TestFallback_BreakTemplatePreservesOneMinute(t *testing.T) {
	qctx := qctxWith("breakTime")
	plan := planForContext(qctx, 3)

	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 3, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	found := false
	for _, q := range smart {
		if q.Title == "Nghỉ mắt 1 phút" {
			found = true
			if q.EstimatedMinutes != 1 {
				t.Fatalf("expected 'Nghỉ mắt 1 phút' estimated_minutes=1, got %d", q.EstimatedMinutes)
			}
		}
	}
	if !found {
		t.Fatalf("expected a 'Nghỉ mắt 1 phút' break quest, got %+v", smart)
	}
}

// TestReviewFallback_NeverSuffixesTitles covers requirement #2: filling many
// review slots must use distinct semantic alternatives, never "(2)"-style
// suffixes, and never duplicate a title.
func TestReviewFallback_NeverSuffixesTitles(t *testing.T) {
	qctx := qctxWith("review")
	plan := planForContext(qctx, 8)

	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 8, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	lr, err := BuildLastResortFill(qctx, plan, smart, 8-len(smart), time.Now())
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	seen := map[string]bool{}
	for _, q := range append(smart, lr...) {
		if strings.Contains(q.Title, "(2)") || strings.Contains(q.Title, "(3)") {
			t.Fatalf("fallback suffixed a title to disambiguate: %q", q.Title)
		}
		key := strings.ToLower(strings.TrimSpace(q.Title))
		if seen[key] {
			t.Fatalf("duplicate fallback title: %q", q.Title)
		}
		seen[key] = true
	}
}

// TestReviewFallback_PicksAlternativeWhenFirstTemplateExists covers requirement
// #2: when the first review template title already exists, the fallback picks a
// different semantic template rather than suffixing the duplicate.
func TestReviewFallback_PicksAlternativeWhenFirstTemplateExists(t *testing.T) {
	qctx := qctxWith("review")
	// The first review template title already exists today (as a title only — its
	// type cap is NOT consumed), so the fallback must pick different semantic
	// review templates rather than reusing/suffixing that title.
	qctx.ExistingQuestTitles = []string{"Ghi lại 1 việc đã hoàn thành"}
	plan := planForContext(qctx, 6)

	lr, err := BuildLastResortFill(qctx, plan, nil, 3, time.Now())
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	if len(lr) == 0 {
		t.Fatal("expected last-resort to produce review alternatives")
	}
	for _, q := range lr {
		if q.Title == "Ghi lại 1 việc đã hoàn thành" {
			t.Fatalf("fallback reused the existing review title instead of an alternative: %q", q.Title)
		}
		if strings.Contains(q.Title, "(") {
			t.Fatalf("fallback suffixed a title: %q", q.Title)
		}
	}
}

// TestFallback_NoSharedReminderTime covers requirement #4: generated pending
// quests must not share a reminder time (and must respect existing reminder
// times, which here is empty).
func TestFallback_NoSharedReminderTime(t *testing.T) {
	qctx := qctxWith("movement", "review", "sleep")
	plan := planForContext(qctx, 6)

	now := time.Now()
	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 6, now)
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	lr, err := BuildLastResortFill(qctx, plan, smart, 6-len(smart), now)
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	all := append(smart, lr...)
	if len(all) < 4 {
		t.Fatalf("expected several generated quests, got %d", len(all))
	}
	times := map[string]int{}
	for _, q := range all {
		if q.ReminderTime == nil {
			t.Fatalf("generated quest missing reminder time: %+v", q)
		}
		key := q.ReminderTime.Format("2006-01-02T15:04")
		times[key]++
		if times[key] > 1 {
			t.Fatalf("two generated quests share reminder time %s", key)
		}
	}
}

// TestFallback_NoDuplicate2300Review covers requirement #4 specifically: an
// existing review reminder at 23:00 must not be reused by a generated review.
func TestFallback_NoDuplicate2300Review(t *testing.T) {
	qctx := qctxWith("review", "breakTime")
	at2300 := time.Date(tomorrow().Year(), tomorrow().Month(), tomorrow().Day(), 23, 0, 0, 0, mustLoadVN(t))
	qctx.ExistingReminderTimes = []time.Time{at2300}
	plan := planForContext(qctx, 5)

	now := time.Now()
	lr, err := BuildLastResortFill(qctx, plan, nil, 5, now)
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	for _, q := range lr {
		if q.ReminderTime != nil && q.ReminderTime.Equal(at2300) {
			t.Fatalf("generated quest reused existing 23:00 reminder: %+v", q)
		}
	}
}

// TestLearningRoadmapFallback_PreservesSourceAndMetadata covers requirement #5:
// a roadmap-linked fallback quest is labeled learning_roadmap (not configBased)
// and carries roadmap/step linkage in its metadata.
func TestLearningRoadmapFallback_PreservesSourceAndMetadata(t *testing.T) {
	qctx := qctxWith("learning", "review")
	qctx.ActiveLearningPath = &ActiveLearningPathDetail{
		RoadmapID:        "roadmap-1",
		StepID:           "step-1",
		RoadmapTitle:     "Học Go",
		CurrentStepTitle: "Goroutines",
		StepOrderIndex:   2,
		TotalSteps:       5,
	}
	plan := planForContext(qctx, 6)

	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 6, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	var learning *models.Quest
	for i := range smart {
		if smart[i].Type == models.QuestTypeLearning {
			learning = &smart[i]
			break
		}
	}
	if learning == nil {
		t.Fatalf("expected a roadmap learning fallback quest, got %+v", smart)
	}
	if learning.Source != models.QuestSourceLearningRoadmap {
		t.Fatalf("expected source=learning_roadmap, got %q", learning.Source)
	}
	if len(learning.LearningMetadata) == 0 {
		t.Fatal("expected learning metadata on roadmap fallback quest")
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(learning.LearningMetadata, &meta); err != nil {
		t.Fatalf("failed to parse learning metadata: %v", err)
	}
	if meta["generation_source"] != "learning_roadmap" {
		t.Errorf("expected generation_source=learning_roadmap, got %v", meta["generation_source"])
	}
	if meta["learning_roadmap_id"] != "roadmap-1" {
		t.Errorf("expected learning_roadmap_id=roadmap-1, got %v", meta["learning_roadmap_id"])
	}
	if meta["learning_step_id"] != "step-1" {
		t.Errorf("expected learning_step_id=step-1, got %v", meta["learning_step_id"])
	}
}

// TestFallback_ReachesCountWhenAlternativesExist covers requirement #6: when
// enough non-capped alternatives exist (review is an unbounded soft filler), the
// fallback reaches exactly the needed count.
func TestFallback_ReachesCountWhenAlternativesExist(t *testing.T) {
	qctx := qctxWith("movement", "review", "sleep")
	plan := planForContext(qctx, 6)

	now := time.Now()
	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 6, now)
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	lr, err := BuildLastResortFill(qctx, plan, smart, 6-len(smart), now)
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	total := len(smart) + len(lr)
	if total != 6 {
		t.Fatalf("expected exactly 6 generated quests, got %d (smart=%d, last-resort=%d)", total, len(smart), len(lr))
	}
	// Movement stays within its cap of 1.
	movement := 0
	for _, q := range append(smart, lr...) {
		if q.Type == models.QuestTypeMovement {
			movement++
		}
	}
	if movement > 1 {
		t.Fatalf("movement cap exceeded: got %d movement quests", movement)
	}
}

// TestMapCandidate_RoadmapMetadataOnlyOnLearning covers the boundary that only
// learning quests carry roadmap linkage: a movement/sleep AI quest must never get
// learning_roadmap metadata even when an active roadmap exists.
func TestMapCandidate_RoadmapMetadataOnlyOnLearning(t *testing.T) {
	qctx := qctxWith("movement", "learning", "sleep")
	qctx.ActiveLearningPath = &ActiveLearningPathDetail{
		RoadmapID: "roadmap-1", StepID: "step-1", RoadmapTitle: "Học Go", CurrentStepTitle: "Goroutines", TotalSteps: 5,
	}
	cands := []QuestCandidate{
		{Type: "learning", Title: "Học Goroutines", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"learning"}, Reason: "r", Instruction: "i", ReminderTime: "20:00"},
		{Type: "movement", Title: "Đi bộ nhẹ", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, Reason: "r", Instruction: "i", ReminderTime: "10:00"},
		{Type: "sleep", Title: "Đi ngủ đúng giờ", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"sleep"}, Reason: "r", Instruction: "i", ReminderTime: "22:30"},
	}
	quests, err := MapCandidatesToQuests(qctx, cands)
	if err != nil {
		t.Fatalf("map error: %v", err)
	}
	for _, q := range quests {
		switch q.Type {
		case models.QuestTypeLearning:
			if len(q.LearningMetadata) == 0 {
				t.Errorf("learning quest should carry roadmap metadata: %+v", q)
			}
		default:
			if len(q.LearningMetadata) != 0 {
				t.Errorf("%s quest must NOT carry learning metadata: %s", q.Type, string(q.LearningMetadata))
			}
		}
	}
}

func mustLoadVN(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("failed to load VN location: %v", err)
	}
	return loc
}
