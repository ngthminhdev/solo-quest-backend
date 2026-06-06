package quest_generation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPromptBuilder_BuildDailyQuestPrompt(t *testing.T) {
	builder := NewPromptBuilder()

	t.Run("builds prompt with enabled categories", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   8,
			EnabledCategories: []string{"learning", "movement", "water"},
			Difficulty:        "normal",
			PreferredDuration: "medium",
		}

		sysPrompt, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if sysPrompt == "" {
			t.Error("system prompt should not be empty")
		}

		if userPrompt == "" {
			t.Error("user prompt should not be empty")
		}

		// Check system prompt contains key instructions
		if !contains(sysPrompt, "JSON") {
			t.Error("system prompt should mention JSON output")
		}
		if !contains(sysPrompt, "difficulty") {
			t.Error("system prompt should mention difficulty")
		}

		// Check user prompt contains context
		if !contains(userPrompt, "Generate exactly 8") {
			t.Error("user prompt should contain quest count in format 'Generate exactly 8 quests'")
		}
		if !contains(userPrompt, "learning, movement, water") {
			t.Error("user prompt should contain enabled categories")
		}
	})

	t.Run("builds prompt with learning and movement time preferences", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:                  uuid.New(),
			LocalDate:               time.Now(),
			Timezone:                "Asia/Ho_Chi_Minh",
			DisplayName:             "Test User",
			DailyQuestCount:         5,
			EnabledCategories:       []string{"learning", "movement"},
			LearningTimePreferences: []string{"morning", "afternoon"},
			MovementTimePreferences: []string{"evening"},
		}

		_, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !contains(userPrompt, "learning_time_preferences") || !contains(userPrompt, "movement_time_preferences") {
			t.Error("user prompt should contain time preferences")
		}
		if !contains(userPrompt, "morning") || !contains(userPrompt, "evening") {
			t.Error("user prompt should contain specific time preference values")
		}
	})

	t.Run("includes JSON output schema in system prompt", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
		}

		sysPrompt, _, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !contains(sysPrompt, "quests") {
			t.Error("system prompt should include JSON schema with 'quests' field")
		}
		if !contains(sysPrompt, "title") && !contains(sysPrompt, "difficulty") {
			t.Error("system prompt should include quest field names")
		}
		// Check for the critical instruction about message.content
		if !contains(sysPrompt, "message.content") {
			t.Error("system prompt should instruct to put final answer in message.content")
		}
		if !contains(sysPrompt, "reasoning_content") {
			t.Error("system prompt should warn against putting JSON in reasoning_content")
		}
	})

	t.Run("does not include user_id or API key", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
		}

		sysPrompt, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		combined := sysPrompt + userPrompt
		userIDStr := qctx.UserID.String()
		if contains(combined, userIDStr) {
			t.Error("prompts should not contain user_id")
		}
		if contains(combined, "API_KEY") || contains(combined, "api_key") || contains(combined, "sk-") {
			t.Error("prompts should not contain API key")
		}
	})

	t.Run("returns error when context is nil", func(t *testing.T) {
		_, _, err := builder.BuildDailyQuestPrompt(nil)
		if err == nil {
			t.Error("expected error when context is nil")
		}
	})

	t.Run("returns error when enabled categories is empty", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   3,
			EnabledCategories: []string{},
		}

		_, _, err := builder.BuildDailyQuestPrompt(qctx)
		if err == nil {
			t.Error("expected error when enabled_categories is empty")
		}
	})

	t.Run("returns error when daily quest count is zero", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   0,
			EnabledCategories: []string{"water"},
		}

		_, _, err := builder.BuildDailyQuestPrompt(qctx)
		if err == nil {
			t.Error("expected error when daily_quest_count is 0")
		}
	})

	t.Run("uses preview_limit when set", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   8,
			PreviewLimit:      3, // Preview only 3
			EnabledCategories: []string{"water"},
		}

		_, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !contains(userPrompt, "Effective Preview Limit: 3") {
			t.Error("user prompt should contain effective preview limit of 3")
		}
		if !contains(userPrompt, "Daily Quest Target Count: 8") {
			t.Error("user prompt should contain Daily Quest Target Count: 8")
		}
	})

	t.Run("uses daily_quest_count when preview_limit not set", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test",
			DailyQuestCount:   5,
			PreviewLimit:      0, // Not set
			EnabledCategories: []string{"water"},
		}

		_, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if !contains(userPrompt, "Effective Preview Limit: 5") {
			t.Error("user prompt should say 'Effective Preview Limit: 5' when preview_limit not set")
		}
	})

	t.Run("prompts include hard config constraints", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			EnabledCategories: []string{"sleep", "learning"},
			Rules: []QuestRuleContext{
				{
					Type:    "sleep",
					Enabled: true,
					ActiveTimeRange: &TimeRangeContext{
						Start: "22:00",
						End:   "23:30",
					},
				},
				{
					Type:    "learning",
					Enabled: true,
					ActiveTimeRange: &TimeRangeContext{
						Start: "19:30",
						End:   "22:00",
					},
				},
			},
		}

		sysPrompt, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// 1. System prompt contains "User configuration is authoritative"
		if !contains(sysPrompt, "User configuration is authoritative") {
			t.Error("system prompt should state that user configuration is authoritative")
		}

		// 2. Prompt says active_time_range is a hard constraint
		if !contains(sysPrompt, "Quest rules are hard constraints") || !contains(sysPrompt, "active_time_range") {
			t.Error("prompt should state that quest rules/active_time_range are hard constraints")
		}

		// 3. User prompt lists hard rule windows
		if !contains(userPrompt, "HARD RULE WINDOWS:") {
			t.Error("user prompt should contain 'HARD RULE WINDOWS:' section")
		}
		if !contains(userPrompt, "usable_reminder_window: 22:00-23:30") {
			t.Error("user prompt should list sleep hard rule window")
		}
		if !contains(userPrompt, "usable_reminder_window: 19:30-22:00") {
			t.Error("user prompt should list learning hard rule window")
		}

		// 4. User prompt says reminders outside type window are invalid
		if !contains(userPrompt, "Any quest outside its type's window is invalid.") {
			t.Error("user prompt should say reminders outside type window are invalid")
		}
	})

	t.Run("prompts include enriched config and preference context", func(t *testing.T) {
		maxPerDaySleep := 1
		maxPerDayLearning := 2
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			QuietAfterTime:    "22:00",
			TargetSleepTime:   "22:30",
			FreeTimeStart:     "18:00",
			FreeTimeEnd:       "20:00",
			WorkStartTime:     "09:00",
			WorkEndTime:       "17:00",
			WakeUpTime:        "07:00",
			EnabledCategories: []string{"sleep", "learning"},
			LearningTimePreferences: []string{"evening"},
			MovementTimePreferences: []string{"morning"},
			Rules: []QuestRuleContext{
				{
					Type:    "sleep",
					Enabled: true,
					MaxPerDay: &maxPerDaySleep,
					ActiveTimeRange: &TimeRangeContext{
						Start: "22:00",
						End:   "23:30",
					},
				},
				{
					Type:    "learning",
					Enabled: true,
					MaxPerDay: &maxPerDayLearning,
					ActiveTimeRange: &TimeRangeContext{
						Start: "19:30",
						End:   "22:00",
					},
				},
			},
		}

		sysPrompt, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// 1. Prompt includes quiet_after_time.
		if !contains(userPrompt, "quiet_after_time: 22:00") {
			t.Error("prompt should include quiet_after_time: 22:00")
		}

		// 2. Prompt includes target_sleep_time.
		if !contains(userPrompt, "target_sleep_time: 22:30") {
			t.Error("prompt should include target_sleep_time: 22:30")
		}

		// 3. Prompt includes learning_time_preferences and movement_time_preferences.
		if !contains(userPrompt, "learning_time_preferences: evening") {
			t.Error("prompt should include learning_time_preferences")
		}
		if !contains(userPrompt, "movement_time_preferences: morning") {
			t.Error("prompt should include movement_time_preferences")
		}

		// 4. Prompt includes work/free time context when present.
		if !contains(userPrompt, "preferred_free_times: 18:00-20:00") {
			t.Error("prompt should include preferred_free_times")
		}
		if !contains(userPrompt, "work_time: 09:00-17:00") {
			t.Error("prompt should include work_time")
		}
		if !contains(userPrompt, "wake_up_time: 07:00") {
			t.Error("prompt should include wake_up_time")
		}

		// 5. Prompt renders usable_reminder_window by intersecting active_time_range with quiet_after_time.
		// For sleep raw 22:00-23:30 and quiet_after_time 22:00, prompt renders usable window 22:00-22:00.
		if !contains(userPrompt, "usable_reminder_window: 22:00-22:00") {
			t.Error("prompt should render usable sleep window intersected with quiet_after_time as 22:00-22:00")
		}
		if !contains(userPrompt, "raw_active_time_range: 22:00-23:30") {
			t.Error("prompt should include raw_active_time_range for sleep")
		}

		// 6. Prompt states hard constraints beat soft preferences.
		if !contains(userPrompt, "Time preferences are soft context only. Hard rule windows still win.") {
			t.Error("prompt should explicitly state time preferences are soft context and hard rule windows win")
		}

		// 7. Prompt does not include user_id.
		if contains(userPrompt, qctx.UserID.String()) || contains(sysPrompt, qctx.UserID.String()) {
			t.Error("prompts should not include user_id")
		}

		// 8. Prompt does not include raw onboarding JSON.
		if contains(userPrompt, "{") && contains(userPrompt, "user_id") {
			t.Error("user prompt should not include raw onboarding JSON representation")
		}
	})

	t.Run("prompts include language naturalness and health safety guidance", func(t *testing.T) {
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
			HealthLimitations: []string{"back_pain"},
		}

		sysPrompt, userPrompt, err := builder.BuildDailyQuestPrompt(qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// 1. System prompt includes natural Vietnamese requirement
		if !contains(sysPrompt, "must be natural Vietnamese") {
			t.Error("system prompt should contain natural Vietnamese requirement")
		}

		// 2. System prompt warns not to mix English in Vietnamese fields
		if !contains(sysPrompt, "Do NOT mix English words into Vietnamese user-facing fields") {
			t.Error("system prompt should warn against mixing English in Vietnamese fields")
		}

		// 3. System prompt warns not to claim treating/reducing symptoms
		if !contains(sysPrompt, "Do not claim a quest treats, reduces, or improves symptoms") {
			t.Error("system prompt should warn against claiming treatment or reduction of symptoms")
		}

		// 4. System prompt includes gentle movement guidance for health limitations
		if !contains(sysPrompt, "Keep movement tasks gentle, generic, and optional") {
			t.Error("system prompt should include gentle movement guidance")
		}

		// 5. User prompt includes health safety guidance and example
		if !contains(userPrompt, "Vận động nhẹ 10 phút trong mức thoải mái") {
			t.Error("user prompt should include positive example for health limitations")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
