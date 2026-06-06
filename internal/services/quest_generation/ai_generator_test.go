package quest_generation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services/ai"
)

func TestAIGenerator_GenerateDailyQuests(t *testing.T) {
	t.Run("generates quests with valid AI response", func(t *testing.T) {
		validJSON := `{
			"quests": [
				{
					"type": "learning",
					"title": "Đọc 10 trang sách",
					"description": "Đọc sách về phát triển bản thân",
					"difficulty": "normal",
					"estimated_minutes": 20,
					"xp_reward": 10,
					"tags": ["learning"],
					"reason": "Phát triển kỹ năng mới",
					"instruction": "Chọn một quyển sách và đọc 10 trang",
					"reminder_time": "14:00"
				}
			]
		}`

		mockClient := &ai.MockClient{
			ResponseText: validJSON,
			Model:        "test-model",
			FinishReason: "stop",
		}

		generator := NewAIGenerator(mockClient)
		qctx := &UserQuestContext{
			UserID:               uuid.New(),
			LocalDate:            time.Now(),
			Timezone:             "Asia/Ho_Chi_Minh",
			DisplayName:          "Test User",
			DailyQuestCount:      1,
			EnabledCategories:    []string{"learning"},
			Difficulty:           "normal",
			PreferredDuration:    "medium",
			ExistingQuestTitles:  []string{},
			LearningTimePreferences: []string{"afternoon"},
			Rules: []QuestRuleContext{
				{
					ID:      "learning-rule",
					Type:    "learning",
					Title:   "Learning Quest",
					Enabled: true,
					ActiveTimeRange: &TimeRangeContext{
						Start: "08:00",
						End:   "22:00",
					},
				},
			},
		}

		quests, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(quests) != 1 {
			t.Fatalf("expected 1 quest, got %d", len(quests))
		}

		if quests[0].Source != models.QuestSourceAI {
			t.Errorf("expected source to be 'ai', got %s", quests[0].Source)
		}

		if quests[0].Title != "Đọc 10 trang sách" {
			t.Errorf("expected title 'Đọc 10 trang sách', got %s", quests[0].Title)
		}
	})

	t.Run("returns error when AI response is invalid JSON", func(t *testing.T) {
		mockClient := &ai.MockClient{
			ResponseText: "This is not valid JSON",
			Model:        "test-model",
			FinishReason: "stop",
		}

		generator := NewAIGenerator(mockClient)
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
			Rules:             []QuestRuleContext{},
		}

		_, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err == nil {
			t.Error("expected error for invalid JSON response")
		}
	})

	t.Run("returns error when AI client fails", func(t *testing.T) {
		mockClient := &ai.MockClient{
			Err: fmt.Errorf("AI service unavailable"),
		}

		generator := NewAIGenerator(mockClient)
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
		}

		_, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err == nil {
			t.Error("expected error when AI client fails")
		}
	})

	t.Run("returns error when candidate validation fails", func(t *testing.T) {
		invalidJSON := `{
			"quests": [
				{
					"type": "invalid_type",
					"title": "Invalid Quest",
					"description": "This quest has invalid type",
					"difficulty": "normal",
					"estimated_minutes": 20,
					"xp_reward": 10,
					"tags": ["test"],
					"reason": "Test",
					"instruction": "Test",
					"reminder_time": "14:00"
				}
			]
		}`

		mockClient := &ai.MockClient{
			ResponseText: invalidJSON,
			Model:        "test-model",
			FinishReason: "stop",
		}

		generator := NewAIGenerator(mockClient)
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   1,
			EnabledCategories: []string{"water"},
			Rules:             []QuestRuleContext{},
		}

		_, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err == nil {
			t.Error("expected validation error for invalid quest type")
		}
	})

	t.Run("returns error when context is nil", func(t *testing.T) {
		mockClient := &ai.MockClient{}
		generator := NewAIGenerator(mockClient)

		_, err := generator.GenerateDailyQuests(context.Background(), nil)
		if err == nil {
			t.Error("expected error when context is nil")
		}
	})

	t.Run("returns error when AI client is nil", func(t *testing.T) {
		generator := NewAIGenerator(nil)
		qctx := &UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         time.Now(),
			Timezone:          "Asia/Ho_Chi_Minh",
			DisplayName:       "Test User",
			DailyQuestCount:   3,
			EnabledCategories: []string{"water"},
		}

		_, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err == nil {
			t.Error("expected error when AI client is nil")
		}
	})

	t.Run("sets source to AI in mapped quests", func(t *testing.T) {
		validJSON := `{
			"quests": [
				{
					"type": "water",
					"title": "Uống 1 ly nước",
					"description": "Bổ sung nước cho cơ thể",
					"difficulty": "easy",
					"estimated_minutes": 5,
					"xp_reward": 5,
					"tags": ["water"],
					"reason": "Duy trì sức khỏe",
					"instruction": "Uống 250ml nước",
					"reminder_time": "10:00"
				},
				{
					"type": "movement",
					"title": "Đi bộ 10 phút",
					"description": "Vận động nhẹ nhàng",
					"difficulty": "normal",
					"estimated_minutes": 10,
					"xp_reward": 10,
					"tags": ["movement"],
					"reason": "Cải thiện sức khỏe",
					"instruction": "Đi bộ quanh nhà",
					"reminder_time": "15:00"
				}
			]
		}`

		mockClient := &ai.MockClient{
			ResponseText: validJSON,
			Model:        "test-model",
			FinishReason: "stop",
		}

		generator := NewAIGenerator(mockClient)
		qctx := &UserQuestContext{
			UserID:               uuid.New(),
			LocalDate:            time.Now(),
			Timezone:             "Asia/Ho_Chi_Minh",
			DisplayName:          "Test User",
			DailyQuestCount:      2,
			EnabledCategories:    []string{"water", "movement"},
			Difficulty:           "normal",
			PreferredDuration:    "short",
			ExistingQuestTitles:  []string{},
			Rules: []QuestRuleContext{
				{
					Type:    "water",
					Enabled: true,
					ActiveTimeRange: &TimeRangeContext{
						Start: "06:00",
						End:   "22:00",
					},
				},
				{
					Type:    "movement",
					Enabled: true,
					ActiveTimeRange: &TimeRangeContext{
						Start: "06:00",
						End:   "22:00",
					},
				},
			},
		}

		quests, err := generator.GenerateDailyQuests(context.Background(), qctx)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(quests) != 2 {
			t.Fatalf("expected 2 quests, got %d", len(quests))
		}

		for i, quest := range quests {
			if quest.Source != models.QuestSourceAI {
				t.Errorf("quest[%d] expected source 'ai', got %s", i, quest.Source)
			}
		}
	})
}
