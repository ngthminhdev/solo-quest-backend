package quest_generation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services/ai"
)

// tomorrow returns tomorrow's date in the user's local zone, which keeps the
// generator's "today" time-shifting logic out of the way so these tests are
// deterministic regardless of wall-clock time.
func tomorrow() time.Time {
	return time.Now().AddDate(0, 0, 1)
}

func wideRule(t string) QuestRuleContext {
	return QuestRuleContext{
		Type:            t,
		Enabled:         true,
		ActiveTimeRange: &TimeRangeContext{Start: "06:00", End: "22:00"},
	}
}

func questByType(quests []models.Quest, qt models.QuestType) (models.Quest, bool) {
	for _, q := range quests {
		if q.Type == qt {
			return q, true
		}
	}
	return models.Quest{}, false
}

func TestAIGenerator_SoftValidationKeepsValidCandidates(t *testing.T) {
	resp := `{"quests": [
		{"type": "learning", "title": "Đọc tài liệu 20 phút", "description": "Học nội dung quan trọng", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 10, "tags": ["learning"], "reason": "x", "instruction": "y", "reminder_time": "14:00"},
		{"type": "movement", "title": "Đi bộ 10 phút", "description": "Vận động nhẹ", "difficulty": "easy", "estimated_minutes": 10, "xp_reward": 5, "tags": ["movement"], "reason": "x", "instruction": "y", "reminder_time": "10:00"},
		{"type": "totally_invalid", "title": "Quest hỏng", "description": "loại không hợp lệ", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 10, "tags": [], "reason": "x", "instruction": "y", "reminder_time": "11:00"}
	]}`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   3,
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning", "movement"},
		Rules:             []QuestRuleContext{wideRule("learning"), wideRule("movement")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("expected success with partial-valid batch, got: %v", err)
	}
	if len(quests) != 2 {
		t.Fatalf("expected 2 valid quests kept, got %d", len(quests))
	}
}

func TestAIGenerator_InvalidXPDoesNotFailBatch(t *testing.T) {
	resp := `{"quests": [
		{"type": "learning", "title": "Ôn lại bài học", "description": "Học", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 15, "tags": ["learning"], "reason": "x", "instruction": "y", "reminder_time": "14:00"}
	]}`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   1,
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning"},
		Rules:             []QuestRuleContext{wideRule("learning")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("wrong xp_reward must not fail the batch, got: %v", err)
	}
	if len(quests) != 1 {
		t.Fatalf("expected 1 quest, got %d", len(quests))
	}
	// Backend normal-difficulty XP is 10, regardless of the AI-provided 15.
	if quests[0].XPReward != 10 {
		t.Errorf("expected backend-computed XP 10 for normal difficulty, got %d", quests[0].XPReward)
	}
}

func TestAIGenerator_ClampEstimatedMinutes(t *testing.T) {
	resp := `{"quests": [
		{"type": "movement", "title": "Đi bộ", "description": "Vận động", "difficulty": "normal", "estimated_minutes": 50, "xp_reward": 10, "tags": ["movement"], "reason": "x", "instruction": "y", "reminder_time": "10:00"},
		{"type": "learning", "title": "Học bài", "description": "Học", "difficulty": "normal", "estimated_minutes": 0, "xp_reward": 10, "tags": ["learning"], "reason": "x", "instruction": "y", "reminder_time": "14:00"},
		{"type": "review", "title": "Nhìn lại ngày hôm nay", "description": "Review", "difficulty": "easy", "estimated_minutes": -5, "xp_reward": 5, "tags": ["review"], "reason": "x", "instruction": "y", "reminder_time": "21:00"}
	]}`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   3,
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "review"},
		Rules:             []QuestRuleContext{wideRule("movement"), wideRule("learning"), wideRule("review")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("out-of-range minutes must be clamped, not rejected, got: %v", err)
	}
	if len(quests) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(quests))
	}
	for _, q := range quests {
		if q.EstimatedMinutes < MinEstimatedMinutes || q.EstimatedMinutes > MaxEstimatedMinutes {
			t.Errorf("quest %q minutes %d out of clamped range [%d,%d]", q.Title, q.EstimatedMinutes, MinEstimatedMinutes, MaxEstimatedMinutes)
		}
	}
	if mv, ok := questByType(quests, models.QuestTypeMovement); ok {
		if mv.EstimatedMinutes != MaxEstimatedMinutes {
			t.Errorf("expected movement minutes clamped to %d, got %d", MaxEstimatedMinutes, mv.EstimatedMinutes)
		}
	} else {
		t.Error("movement quest missing")
	}
}

func TestAIGenerator_DropsUnknownTags(t *testing.T) {
	resp := `{"quests": [
		{"type": "learning", "title": "Học bài", "description": "Học", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 10, "tags": ["learning", "banana-unknown"], "reason": "x", "instruction": "y", "reminder_time": "14:00"}
	]}`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   1,
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning"},
		Rules:             []QuestRuleContext{wideRule("learning")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unknown tag must not drop the candidate, got: %v", err)
	}
	if len(quests) != 1 {
		t.Fatalf("expected 1 quest, got %d", len(quests))
	}

	var tags []string
	if err := json.Unmarshal(quests[0].Tags, &tags); err != nil {
		t.Fatalf("failed to unmarshal tags: %v", err)
	}
	for _, tg := range tags {
		if tg == "banana-unknown" {
			t.Errorf("unknown tag should have been dropped, got tags %v", tags)
		}
	}
	found := false
	for _, tg := range tags {
		if tg == "learning" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected canonical 'learning' tag to remain, got %v", tags)
	}
}

func TestAIGenerator_ParsesRawArrayResponse(t *testing.T) {
	resp := `[
		{"type": "learning", "title": "Học bài 20 phút", "description": "Học", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 10, "tags": ["learning"], "reason": "x", "instruction": "y", "reminder_time": "14:00"}
	]`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   1,
		PreferredDuration: "medium",
		EnabledCategories: []string{"learning"},
		Rules:             []QuestRuleContext{wideRule("learning")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("raw array response must be parsed, got: %v", err)
	}
	if len(quests) != 1 {
		t.Fatalf("expected 1 quest from raw array, got %d", len(quests))
	}
}

func TestAIGenerator_EmptyArrayFallsBackCleanly(t *testing.T) {
	gen := NewAIGenerator(&ai.MockClient{ResponseText: `[]`, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   3,
		EnabledCategories: []string{"learning"},
		Rules:             []QuestRuleContext{wideRule("learning")},
	}

	// Must return an error (so the service falls back) and must not panic.
	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err == nil {
		t.Fatal("expected an error for empty array so the service can fall back")
	}
	if len(quests) != 0 {
		t.Errorf("expected no quests, got %d", len(quests))
	}
}

func TestAIGenerator_DropsWaterEvenWhenEnabled(t *testing.T) {
	// Water is a reminder habit, never an actionable daily quest. Even when a
	// water candidate is produced (and water appears "enabled"), the quality gate
	// must drop it; the remaining valid candidates are kept.
	resp := `{"quests": [
		{"type": "water", "title": "Uống nước", "description": "Bù nước", "difficulty": "easy", "estimated_minutes": 2, "xp_reward": 5, "tags": ["hydration"], "reason": "x", "instruction": "y", "reminder_time": "09:00"},
		{"type": "learning", "title": "Học bài", "description": "Học", "difficulty": "normal", "estimated_minutes": 20, "xp_reward": 10, "tags": ["learning"], "reason": "x", "instruction": "y", "reminder_time": "14:00"}
	]}`

	gen := NewAIGenerator(&ai.MockClient{ResponseText: resp, Model: "m", FinishReason: "stop"})
	qctx := &UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         tomorrow(),
		Timezone:          "Asia/Ho_Chi_Minh",
		DailyQuestCount:   2,
		PreferredDuration: "medium",
		EnabledCategories: []string{"water", "learning"},
		Rules:             []QuestRuleContext{wideRule("water"), wideRule("learning")},
	}

	quests, err := gen.GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("expected learning candidate to be kept after dropping water, got: %v", err)
	}
	if len(quests) != 1 {
		t.Fatalf("expected 1 quest (water dropped), got %d", len(quests))
	}
	for _, q := range quests {
		if q.Type == models.QuestTypeWater {
			t.Errorf("water quest must never be kept, got %+v", q)
		}
	}
	if quests[0].Type != models.QuestTypeLearning {
		t.Errorf("expected the kept quest to be learning, got %s", quests[0].Type)
	}
}
