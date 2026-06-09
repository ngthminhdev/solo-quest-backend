package quest_generation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/pkg/logger"
)

func init() {
	logger.InitForTest()
}

func TestAIGenerator_GeminiSmokeTest(t *testing.T) {
	_ = godotenv.Overload("../../../.env.test")

	if os.Getenv("AI_PROVIDER") != "gemini" {
		t.Skip("skipping: AI_PROVIDER != gemini")
	}
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("skipping: GEMINI_API_KEY not set")
	}
	if os.Getenv("AI_ENABLED") != "true" {
		t.Skip("skipping: AI_ENABLED != true")
	}

	cfg := ai.LoadConfig()
	client, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create AI client: %v", err)
	}

	maxPerDay1 := 1
	maxPerDay3 := 3

	qctx := &quest_generation.UserQuestContext{
		UserID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		LocalDate:        timeutil.TodayVN(),
		Timezone:         "Asia/Ho_Chi_Minh",
		DisplayName:      "Minh Thanh",
		Age:              30,
		Height:           170,
		Weight:           65,
		MainActivity:     "Developer",
		ActivityLevel:    "sedentary",
		LastWorkout:      "3 days ago",
		MainGoals:        []string{"learning", "health", "sleep"},
		HealthLimitations: []string{},
		WorkScheduleType: "weekdays",
		WorkWeekdays:     []int{1, 2, 3, 4, 5},
		WorkStartTime:    "09:00",
		WorkEndTime:      "18:00",
		WakeUpTime:       "07:00",
		TargetSleepTime:  "23:30",
		QuietAfterTime:   "22:00",
		FreeTimeStart:    "19:00",
		FreeTimeEnd:      "22:00",
		LearningTimePreferences: []string{"evening"},
		MovementTimePreferences: []string{"morning"},
		DailyQuestCount:  5,
		EnabledCategories: []string{"learning", "movement", "sleep", "review"},
		Rules: []quest_generation.QuestRuleContext{
			{
				ID:      "rule-learning",
				Type:    "learning",
				Title:   "Học tập",
				Enabled: true,
				MaxPerDay: &maxPerDay3,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "07:00", End: "22:00"},
			},
			{
				ID:      "rule-movement",
				Type:    "movement",
				Title:   "Vận động",
				Enabled: true,
				MaxPerDay: &maxPerDay1,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "07:00", End: "21:00"},
			},
			{
				ID:      "rule-sleep",
				Type:    "sleep",
				Title:   "Ngủ",
				Enabled: true,
				MaxPerDay: &maxPerDay1,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "21:00", End: "22:00"},
			},
			{
				ID:      "rule-review",
				Type:    "review",
				Title:   "Điểm lại ngày",
				Enabled: true,
				MaxPerDay: &maxPerDay1,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "20:00", End: "22:00"},
			},
		},
		TodayCheckIn: &quest_generation.TodayCheckInDetail{
			Mood:         "good",
			EnergyLevel:  "medium",
			Availability: "normal",
			Priority:     "learning",
		},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapTitle:     "Flutter App Architecture",
			CurrentStepTitle: "State Management",
			Description:      "Học các pattern quản lý state phổ biến: Provider, Riverpod, Bloc",
		},
	}

	promptBuilder := quest_generation.NewPromptBuilder()
	systemPrompt, userPrompt, err := promptBuilder.BuildDailyQuestPrompt(qctx)
	if err != nil {
		t.Fatalf("failed to build prompt: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.QuestTimeoutSeconds)*time.Second)
	defer cancel()

	t.Logf("=== Generating quests via %s (%s) ===", cfg.Provider, cfg.GeminiModel)
	t.Logf("Date: %s | Count target: %d", qctx.LocalDate.Format("2006-01-02"), qctx.DailyQuestCount)

	aiResp, err := client.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.2,
		MaxTokens:    cfg.QuestMaxTokens,
	})
	if err != nil {
		t.Fatalf("AI call failed: %v", err)
	}

	candidates, err := quest_generation.ParseQuestCandidateResponse(aiResp.Text)
	if err != nil {
		t.Logf("Raw AI text:\n%s", aiResp.Text)
		t.Fatalf("failed to parse response: %v", err)
	}

	if candidates == nil || len(candidates.Quests) == 0 {
		t.Fatal("no quest candidates returned")
	}

	// Print raw results (no validation — shows full Gemini output for comparison)
	fmt.Printf("\n=== GEMINI RAW RESULTS — %s | latency=%dms | %d quests ===\n",
		aiResp.Model, aiResp.LatencyMs, len(candidates.Quests))
	for i, q := range candidates.Quests {
		fmt.Printf("\n[%d] %s\n", i+1, q.Title)
		fmt.Printf("    type=%-10s difficulty=%-6s xp=%d  duration=%dmin  reminder=%s\n",
			q.Type, q.Difficulty, q.XPReward, q.EstimatedMinutes, q.ReminderTime)
		fmt.Printf("    %s\n", q.Description)
		if q.Instruction != "" {
			fmt.Printf("    → %s\n", q.Instruction)
		}
		tagsJSON, _ := json.Marshal(q.Tags)
		fmt.Printf("    tags: %s\n", string(tagsJSON))
		if q.Reason != "" {
			fmt.Printf("    reason: %s\n", q.Reason)
		}
	}
	fmt.Println()

	t.Logf("PASS: model=%s latency=%dms quests=%d", aiResp.Model, aiResp.LatencyMs, len(candidates.Quests))
}

func formatReminderTime(t *time.Time) string {
	if t == nil {
		return "none"
	}
	return t.Format("15:04")
}
