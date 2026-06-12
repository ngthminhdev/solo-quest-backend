package quest_generation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/datatypes"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// TestGenerateToday_MovementCapDropsExtrasAndFallbackFills reproduces the
// original bug: AI returns 6 quests, all movement, but the movement cap is 1. The
// quality gate keeps a single movement quest, the smart/last-resort fallback
// fills the remaining 5 with non-movement quests, and the job succeeds with
// exactly the target number of quests.
func TestGenerateToday_MovementCapDropsExtrasAndFallbackFills(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)
	userID := testutils.BootstrapTestUser(t, db)

	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time": "09:00", "work_end_time": "18:00",
		"wake_up_time": "07:00", "target_sleep_time": "23:00", "quiet_after_time": "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	max1 := 1
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "review", "sleep", "breakTime"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "easy", MaxPerDay: &max1, ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_review", Type: "review", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_sleep", Type: "sleep", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "20:00", End: "23:30"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_break", Type: "breakTime", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
	})
	db.Create(&models.QuestSettings{
		UserID: userID, DailyQuestCount: 6, Difficulty: "easy",
		EnabledCategories: datatypes.JSON(catsJSON), PreferredDuration: "short", Rules: datatypes.JSON(rulesJSON),
	})

	// AI returns 6 movement quests; the cap of 1 means 5 are dropped.
	aiResp := `{"quests":[
		{"type":"movement","title":"Đi bộ nhẹ 1","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"09:00"},
		{"type":"movement","title":"Đi bộ nhẹ 2","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"10:00"},
		{"type":"movement","title":"Đi bộ nhẹ 3","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"11:00"},
		{"type":"movement","title":"Đi bộ nhẹ 4","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"13:00"},
		{"type":"movement","title":"Đi bộ nhẹ 5","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"14:00"},
		{"type":"movement","title":"Đi bộ nhẹ 6","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"15:00"}
	]}`
	aiGen := quest_generation.NewAIGenerator(&ai.MockClient{ResponseText: aiResp, Model: "m", FinishReason: "stop"})
	service := quest_generation.NewGenerationService(
		db, quest_generation.NewUserQuestContextBuilder(db), aiGen, quest_generation.NewRuleBasedGenerator(db),
	)

	preferAI := true
	date := timeutil.FormatDateVN(timeutil.TodayVN().AddDate(0, 0, 1))
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		Date: &date, PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(result.Quests) != 6 {
		t.Fatalf("expected exactly 6 quests saved, got %d", len(result.Quests))
	}

	movementCount := 0
	waterCount := 0
	titles := map[string]int{}
	for _, q := range result.Quests {
		switch q.Type {
		case models.QuestTypeMovement:
			movementCount++
		case models.QuestTypeWater:
			waterCount++
		}
		titles[strings.ToLower(strings.TrimSpace(q.Title))]++
	}
	if movementCount != 1 {
		t.Errorf("expected exactly 1 movement quest (cap=1), got %d", movementCount)
	}
	if waterCount != 0 {
		t.Errorf("expected 0 water quests, got %d", waterCount)
	}
	for title, n := range titles {
		if n > 1 {
			t.Errorf("duplicate title after AI + fallback merge: %q (x%d)", title, n)
		}
	}

	var dbCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&dbCount)
	if dbCount != 6 {
		t.Errorf("expected 6 quests in DB, got %d", dbCount)
	}
}

// TestGenerateToday_PartialValidAIFilledToTarget verifies that when the AI
// returns fewer valid quests than needed, the smart fallback fills the rest and
// exactly needed_count quests are saved.
func TestGenerateToday_PartialValidAIFilledToTarget(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)
	userID := testutils.BootstrapTestUser(t, db)

	obJSON, _ := json.Marshal(map[string]interface{}{
		"work_start_time": "09:00", "work_end_time": "18:00",
		"wake_up_time": "07:00", "target_sleep_time": "23:00", "quiet_after_time": "22:00",
	})
	db.Create(&models.OnboardingAnswer{UserID: userID, Answers: datatypes.JSON(obJSON), Completed: true})

	catsJSON, _ := json.Marshal([]string{"movement", "learning", "review", "sleep", "breakTime"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_review", Type: "review", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_sleep", Type: "sleep", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "20:00", End: "23:30"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_break", Type: "breakTime", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "06:00", End: "22:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
	})
	db.Create(&models.QuestSettings{
		UserID: userID, DailyQuestCount: 6, Difficulty: "easy",
		EnabledCategories: datatypes.JSON(catsJSON), PreferredDuration: "short", Rules: datatypes.JSON(rulesJSON),
	})

	// Only 2 valid quests from AI.
	aiResp := `{"quests":[
		{"type":"movement","title":"Đi bộ nhẹ buổi sáng","description":"Vận động","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["movement"],"reason":"x","instruction":"y","reminder_time":"09:00"},
		{"type":"review","title":"Nhìn lại buổi sáng","description":"Ôn nhanh","difficulty":"easy","estimated_minutes":10,"xp_reward":5,"tags":["review"],"reason":"x","instruction":"y","reminder_time":"21:00"}
	]}`
	aiGen := quest_generation.NewAIGenerator(&ai.MockClient{ResponseText: aiResp, Model: "m", FinishReason: "stop"})
	service := quest_generation.NewGenerationService(
		db, quest_generation.NewUserQuestContextBuilder(db), aiGen, quest_generation.NewRuleBasedGenerator(db),
	)

	preferAI := true
	date := timeutil.FormatDateVN(timeutil.TodayVN().AddDate(0, 0, 1))
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		Date: &date, PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(result.Quests) != 6 {
		t.Fatalf("expected exactly 6 quests saved, got %d", len(result.Quests))
	}
	if result.Source != "ai" {
		t.Errorf("expected source ai (partial AI kept), got %s", result.Source)
	}
	if !result.FallbackUsed {
		t.Error("expected fallback_used true")
	}
	titles := map[string]int{}
	for _, q := range result.Quests {
		if q.Type == models.QuestTypeWater {
			t.Errorf("water quest must not be saved: %+v", q)
		}
		titles[strings.ToLower(strings.TrimSpace(q.Title))]++
	}
	for title, n := range titles {
		if n > 1 {
			t.Errorf("duplicate title after merge: %q (x%d)", title, n)
		}
	}
}
