package quest_generation

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
)

func planForContext(qctx *UserQuestContext, needed int) QuestCompositionPlan {
	qctx.DailyQuestCount = needed
	qctx.PreviewLimit = needed
	return BuildQuestCompositionPlan(qctx)
}

func TestSmartFallback_NeverGeneratesWater(t *testing.T) {
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "review", "sleep", "breakTime"},
		Rules: []QuestRuleContext{
			wideRule("movement"), wideRule("learning"), wideRule("review"), wideRule("sleep"), wideRule("breakTime"),
		},
	}
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
		if q.Type == models.QuestTypeWater {
			t.Fatalf("fallback produced a water quest: %+v", q)
		}
	}
}

func TestSmartFallback_RespectsMovementCap(t *testing.T) {
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "review", "sleep", "breakTime"},
		Rules:             []QuestRuleContext{wideRule("movement"), wideRule("review"), wideRule("sleep"), wideRule("breakTime")},
	}
	plan := planForContext(qctx, 6)

	// A movement quest is already kept, so the movement cap (1) is used up.
	kept := []models.Quest{{Title: "Đi bộ", Type: models.QuestTypeMovement}}

	smart, err := BuildSmartFallbackQuests(qctx, plan, kept, 5, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}
	for _, q := range smart {
		if q.Type == models.QuestTypeMovement {
			t.Fatalf("smart fallback exceeded movement cap: %+v", q)
		}
	}
}

func TestSmartFallback_UsesActiveRoadmapStep(t *testing.T) {
	step := "Goroutines"
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "review", "movement"},
		Rules:             []QuestRuleContext{wideRule("learning"), wideRule("review"), wideRule("movement")},
		ActiveLearningPath: &ActiveLearningPathDetail{
			RoadmapID:        "r1",
			StepID:           "s1",
			RoadmapTitle:     "Học Go",
			CurrentStepTitle: step,
			TotalSteps:       5,
		},
	}
	plan := planForContext(qctx, 6)
	if !plan.RequireLearningRoadmap {
		t.Fatal("expected RequireLearningRoadmap true with an active roadmap step")
	}

	smart, err := BuildSmartFallbackQuests(qctx, plan, nil, 6, time.Now())
	if err != nil {
		t.Fatalf("smart fallback error: %v", err)
	}

	foundStepRef := false
	for _, q := range smart {
		if q.Type == models.QuestTypeLearning {
			if strings.Contains(q.Title+" "+q.Description, step) {
				foundStepRef = true
			}
			if len(q.LearningMetadata) == 0 {
				t.Errorf("roadmap learning quest missing learning metadata: %+v", q)
			}
		}
	}
	if !foundStepRef {
		t.Errorf("expected at least one learning quest referencing step %q, got %+v", step, smart)
	}
}

func TestLastResortFill_ProducesUniqueTitlesAndExactCount(t *testing.T) {
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		PreferredDuration: "medium",
		EnabledCategories: []string{"review", "movement", "breakTime"},
		Rules:             []QuestRuleContext{wideRule("review"), wideRule("movement"), wideRule("breakTime")},
	}
	plan := planForContext(qctx, 10)

	lr, err := BuildLastResortFill(qctx, plan, nil, 10, time.Now())
	if err != nil {
		t.Fatalf("last-resort error: %v", err)
	}
	if len(lr) != 10 {
		t.Fatalf("expected exactly 10 last-resort quests, got %d", len(lr))
	}
	seen := map[string]bool{}
	for _, q := range lr {
		key := strings.ToLower(strings.TrimSpace(q.Title))
		if seen[key] {
			t.Errorf("duplicate last-resort title: %q", q.Title)
		}
		seen[key] = true
		if q.ReminderTime == nil {
			t.Errorf("last-resort quest missing reminder time: %+v", q)
		}
	}
}
