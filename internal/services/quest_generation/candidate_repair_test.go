package quest_generation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func repairCtx() *UserQuestContext {
	return &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         time.Now().AddDate(0, 0, 1), // tomorrow: skip "today" future checks
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   5,
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "review", "sleep"},
		Rules: []QuestRuleContext{
			wideRule("movement"), wideRule("learning"), wideRule("review"), wideRule("sleep"),
		},
	}
}

func TestRepairAndValidate_FillsEmptyDescriptionAndKeeps(t *testing.T) {
	cands := []QuestCandidate{
		{Type: "learning", Title: "Học bài", Description: "", Difficulty: "normal", EstimatedMinutes: 20, Tags: []string{"learning"}, ReminderTime: "14:00"},
	}
	kept, report := RepairAndValidateCandidates(repairCtx(), cands, time.Now())
	if len(kept) != 1 {
		t.Fatalf("expected candidate kept after repair, got %d (dropped=%v)", len(kept), report.DropReasons)
	}
	if kept[0].Description == "" {
		t.Error("expected empty description to be filled")
	}
	if kept[0].Instruction == "" {
		t.Error("expected instruction to be filled")
	}
	if report.RepairedCount == 0 {
		t.Error("expected repair to be recorded")
	}
}

func TestRepairAndValidate_EmptyTitleDropsOnlyThatCandidate(t *testing.T) {
	cands := []QuestCandidate{
		{Type: "movement", Title: "", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, ReminderTime: "10:00"},
		{Type: "learning", Title: "Học bài", Description: "x", Difficulty: "normal", EstimatedMinutes: 20, Tags: []string{"learning"}, ReminderTime: "14:00"},
	}
	kept, report := RepairAndValidateCandidates(repairCtx(), cands, time.Now())
	if len(kept) != 1 {
		t.Fatalf("expected only the empty-title candidate dropped, kept=%d", len(kept))
	}
	if kept[0].Type != "learning" {
		t.Errorf("expected the learning candidate kept, got %s", kept[0].Type)
	}
	if report.DroppedCount != 1 {
		t.Errorf("expected exactly 1 drop, got %d", report.DroppedCount)
	}
}

func TestRepairAndValidate_DuplicateTitleDropsOnlyDuplicate(t *testing.T) {
	cands := []QuestCandidate{
		{Type: "movement", Title: "Cùng tên", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, ReminderTime: "10:00"},
		{Type: "learning", Title: "Cùng tên", Description: "x", Difficulty: "normal", EstimatedMinutes: 20, Tags: []string{"learning"}, ReminderTime: "14:00"},
	}
	kept, report := RepairAndValidateCandidates(repairCtx(), cands, time.Now())
	if len(kept) != 1 {
		t.Fatalf("expected the duplicate dropped, kept=%d", len(kept))
	}
	if report.DroppedCount != 1 {
		t.Errorf("expected exactly 1 drop, got %d", report.DroppedCount)
	}
}

func TestRepairAndValidate_InvalidReminderDropsOnlyThatCandidate(t *testing.T) {
	cands := []QuestCandidate{
		{Type: "movement", Title: "Đi bộ", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, ReminderTime: "99:99"},
		{Type: "learning", Title: "Học bài", Description: "x", Difficulty: "normal", EstimatedMinutes: 20, Tags: []string{"learning"}, ReminderTime: "14:00"},
	}
	kept, report := RepairAndValidateCandidates(repairCtx(), cands, time.Now())
	if len(kept) != 1 {
		t.Fatalf("expected the bad-reminder candidate dropped, kept=%d", len(kept))
	}
	if kept[0].Type != "learning" {
		t.Errorf("expected learning kept, got %s", kept[0].Type)
	}
	if report.DroppedCount != 1 {
		t.Errorf("expected exactly 1 drop, got %d", report.DroppedCount)
	}
}

func TestRepairAndValidate_ClampsReminderOutsideActiveRange(t *testing.T) {
	qctx := repairCtx()
	// Narrow the movement rule window and feed a time after the window end.
	qctx.Rules = []QuestRuleContext{
		{Type: "movement", Enabled: true, ActiveTimeRange: &TimeRangeContext{Start: "06:00", End: "10:00"}},
	}
	qctx.EnabledCategories = []string{"movement"}
	cands := []QuestCandidate{
		{Type: "movement", Title: "Đi bộ", Description: "x", Difficulty: "easy", EstimatedMinutes: 10, Tags: []string{"movement"}, ReminderTime: "15:00"},
	}
	kept, _ := RepairAndValidateCandidates(qctx, cands, time.Now())
	if len(kept) != 1 {
		t.Fatalf("expected candidate kept after clamping, kept=%d", len(kept))
	}
	if kept[0].ReminderTime != "10:00" {
		t.Errorf("expected reminder clamped to window end 10:00, got %s", kept[0].ReminderTime)
	}
}
