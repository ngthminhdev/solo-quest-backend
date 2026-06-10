package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/testutils"
)

func validAIResponse() string {
	resp := map[string]interface{}{
		"title":       "Lộ trình học Flutter cơ bản",
		"description": "Lộ trình giúp bạn làm chủ Flutter từ cơ bản đến nâng cao",
		"category":    "Flutter",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Cài đặt Flutter SDK và môi trường", "description": "Cài đặt Flutter SDK, Android Studio và cấu hình môi trường", "order_index": 1, "estimated_minutes": 30, "outcome": "Môi trường Flutter hoạt động"},
			{"title": "Tạo ứng dụng Flutter đầu tiên", "description": "Chạy lệnh flutter create và hiểu cấu trúc project", "order_index": 2, "estimated_minutes": 30, "outcome": "Biết cách tạo project Flutter"},
			{"title": "Hiểu về Widget cơ bản", "description": "Tìm hiểu StatelessWidget, StatefulWidget, và các widget cơ bản", "order_index": 3, "estimated_minutes": 45, "outcome": "Sử dụng được widget cơ bản"},
			{"title": "Làm việc với Layout", "description": "Tìm hiểu Row, Column, Container, Stack", "order_index": 4, "estimated_minutes": 45, "outcome": "Tạo được layout Flutter"},
			{"title": "Xử lý sự kiện với GestureDetector", "description": "Bắt sự kiện tap, long press, swipe", "order_index": 5, "estimated_minutes": 30, "outcome": "Xử lý tương tác người dùng"},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func validAIResponseJsonString() string {
	return validAIResponse()
}

func setupGenerateService(t *testing.T, mockResponse string, mockErr error) (*services.LearningRoadmapService, uuid.UUID, *gorm.DB) {
	t.Helper()
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	mock := ai.NewMockClient(mockResponse, mockErr)
	svc.SetAIClient(mock)
	return svc, userID, db
}

func TestGenerateRoadmap_Success(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter cơ bản",
			Category:     "Flutter",
			Difficulty:   "beginner",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Source != "ai" {
		t.Errorf("expected source='ai', got '%s'", result.Source)
	}
	if result.GeneratedStepCount != 5 {
		t.Errorf("expected 5 steps, got %d", result.GeneratedStepCount)
	}
	if result.Item.Source != "ai" {
		t.Errorf("expected item source='ai', got '%s'", result.Item.Source)
	}
	if result.Item.Status != string(models.UserLearningRoadmapStatusTracking) {
		t.Errorf("expected status tracking, got '%s'", result.Item.Status)
	}
	if result.Item.CompletedSteps != 0 {
		t.Errorf("expected 0 completed_steps, got %d", result.Item.CompletedSteps)
	}
	if result.Item.ProgressPercent != 0 {
		t.Errorf("expected 0 progress_percent, got %d", result.Item.ProgressPercent)
	}
	if result.Item.StartedAt == nil {
		t.Error("expected started_at for auto-followed roadmap")
	}
	if result.Item.Title == "" {
		t.Error("expected non-empty title")
	}
	if result.Item.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(result.Item.Steps) != 5 {
		t.Errorf("expected 5 step items, got %d", len(result.Item.Steps))
	}
	for i, step := range result.Item.Steps {
		if step.OrderIndex != i {
			t.Errorf("step %d: expected order_index %d, got %d", i, i, step.OrderIndex)
		}
		if step.Completed {
			t.Errorf("step %d: expected completed=false", i)
		}
		if step.CompletedAt != nil {
			t.Errorf("step %d: expected completed_at=nil", i)
		}
	}
}

func TestGenerateRoadmap_EmptyGoal(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if !errors.Is(err, services.ErrEmptyLearningGoal) {
		t.Fatalf("expected ErrEmptyLearningGoal, got %v", err)
	}
}

func TestGenerateRoadmap_ShortGoal(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "ab",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if !errors.Is(err, services.ErrEmptyLearningGoal) {
		t.Fatalf("expected ErrEmptyLearningGoal for short goal, got %v", err)
	}
}

func TestGenerateRoadmap_AIDisabled(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if !errors.Is(err, services.ErrAIDisabled) {
		t.Fatalf("expected ErrAIDisabled, got %v", err)
	}
}

func TestGenerateRoadmap_AIProviderError(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, "", errors.New("connection refused"))

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error from AI provider")
	}
	if !errors.Is(err, services.ErrAIProviderFailed) {
		t.Fatalf("expected ErrAIProviderFailed, got %v", err)
	}
}

func TestGenerateRoadmap_InvalidJSON(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, "this is not json", nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, services.ErrAIInvalidOutput) {
		t.Fatalf("expected ErrAIInvalidOutput, got %v", err)
	}
}

func TestGenerateRoadmap_JsonInMarkdownBlock(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, "```json\n"+validAIResponseJsonString()+"\n```", nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error for code-fenced JSON: %v", err)
	}
	if result.GeneratedStepCount != 5 {
		t.Errorf("expected 5 steps from code-fenced JSON, got %d", result.GeneratedStepCount)
	}
}

func TestGenerateRoadmap_TooFewValidSteps(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "test desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Step 1", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Step 2", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error for too few steps")
	}
	if !errors.Is(err, services.ErrAITooFewValidSteps) {
		t.Fatalf("expected ErrAITooFewValidSteps, got %v", err)
	}
}

func TestGenerateRoadmap_TooManyStepsTruncated(t *testing.T) {
	steps := make([]map[string]interface{}, 15)
	for i := 0; i < 15; i++ {
		steps[i] = map[string]interface{}{
			"title":             "Step " + string(rune('A'+i)),
			"description":       "specific description for step",
			"order_index":       i + 1,
			"estimated_minutes": 20,
			"outcome":           "ok",
		}
	}
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "test desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps":       steps,
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  500,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GeneratedStepCount > 12 {
		t.Errorf("expected at most 12 steps, got %d", result.GeneratedStepCount)
	}
	if result.Item.TotalSteps > 12 {
		t.Errorf("expected TotalSteps at most 12, got %d", result.Item.TotalSteps)
	}
}

func TestGenerateRoadmap_GenericStepTitlesRejected(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "test desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Học tập Flutter", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Đọc tài liệu Dart", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Cài đặt Flutter SDK và tạo project đầu tiên", "description": "specific action", "order_index": 3, "estimated_minutes": 30, "outcome": "ok"},
			{"title": "Viết widget đầu tiên với Container và Text", "description": "desc", "order_index": 4, "estimated_minutes": 30, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
			Difficulty:   "beginner",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error after generic filtering leaves too few steps")
	}
	if !errors.Is(err, services.ErrAITooFewValidSteps) {
		t.Fatalf("expected ErrAITooFewValidSteps, got %v", err)
	}
}

func TestGenerateRoadmap_MaxDurationLimitsTotal(t *testing.T) {
	steps := []map[string]interface{}{
		{"title": "Step 1 specific title", "description": "desc", "order_index": 1, "estimated_minutes": 30, "outcome": "ok"},
		{"title": "Step 2 specific title", "description": "desc", "order_index": 2, "estimated_minutes": 30, "outcome": "ok"},
		{"title": "Step 3 specific title", "description": "desc", "order_index": 3, "estimated_minutes": 30, "outcome": "ok"},
		{"title": "Step 4 specific title", "description": "desc", "order_index": 4, "estimated_minutes": 30, "outcome": "ok"},
		{"title": "Step 5 specific title", "description": "desc", "order_index": 5, "estimated_minutes": 30, "outcome": "ok"},
		{"title": "Step 6 specific title", "description": "desc", "order_index": 6, "estimated_minutes": 30, "outcome": "ok"},
	}
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "test desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps":       steps,
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  100,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.EstimatedMinutes > 100 {
		t.Errorf("expected estimated_minutes <= 100, got %d", result.Item.EstimatedMinutes)
	}
	if result.GeneratedStepCount < 3 {
		t.Errorf("expected at least 3 steps after truncation, got %d", result.GeneratedStepCount)
	}
}

func TestGenerateRoadmap_DifficultyAny(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			Difficulty:   "any",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error with difficulty='any': %v", err)
	}
	if result.Item.Difficulty == "" {
		t.Error("expected non-empty difficulty")
	}
}

func TestGenerateRoadmap_CategoryOverride(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "desc",
		"category":    "AI_Category",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Specific step 1", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 2", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 3", "description": "desc", "order_index": 3, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			Category:     "UserCategory",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.Category != "UserCategory" {
		t.Errorf("expected category='UserCategory' (user override), got '%s'", result.Item.Category)
	}
}

func TestGenerateRoadmap_NoWriteOnAIError(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, "", errors.New("timeout"))

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error")
	}

	var count int64
	db := testutils.SetupTestDB(t)
	db.Model(&models.LearningRoadmap{}).Count(&count)
	if count > 0 {
		t.Error("expected no roadmaps written to DB on AI error")
	}
}

func TestGenerateRoadmap_DefaultMaxDuration(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error with default max_duration: %v", err)
	}
	if result.GeneratedStepCount != 5 {
		t.Errorf("expected 5 steps, got %d", result.GeneratedStepCount)
	}
}

func TestGenerateRoadmap_InvalidMaxDuration(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  10,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if !errors.Is(err, services.ErrInvalidMaxDuration) {
		t.Fatalf("expected ErrInvalidMaxDuration, got %v", err)
	}
}

func TestGenerateRoadmap_UserOwned(t *testing.T) {
	svc, userID, db := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var roadmap models.LearningRoadmap
	if err := db.Where("id = ?", result.Item.ID).First(&roadmap).Error; err != nil {
		t.Fatalf("failed to find created roadmap: %v", err)
	}
	if roadmap.CreatedByUserID == nil || *roadmap.CreatedByUserID != userID {
		t.Errorf("expected created_by_user_id=%v, got %v", userID, roadmap.CreatedByUserID)
	}
	if roadmap.Source != models.LearningRoadmapSourceAI {
		t.Errorf("expected source='ai', got '%s'", roadmap.Source)
	}
	if !roadmap.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestGenerateRoadmap_StepsClampedMinutes(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Specific step 1", "description": "desc", "order_index": 1, "estimated_minutes": 5, "outcome": "ok"},
			{"title": "Specific step 2", "description": "desc", "order_index": 2, "estimated_minutes": 100, "outcome": "ok"},
			{"title": "Specific step 3", "description": "desc", "order_index": 3, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.Steps[0].EstimatedMinutes != 10 {
		t.Errorf("step 0: expected clamped to 10, got %d", result.Item.Steps[0].EstimatedMinutes)
	}
	if result.Item.Steps[1].EstimatedMinutes != 45 {
		t.Errorf("step 1: expected clamped to 45, got %d", result.Item.Steps[1].EstimatedMinutes)
	}
	if result.Item.Steps[2].EstimatedMinutes != 20 {
		t.Errorf("step 2: expected unchanged 20, got %d", result.Item.Steps[2].EstimatedMinutes)
	}
}

func TestGenerateRoadmap_MissingTitle(t *testing.T) {
	resp := map[string]interface{}{
		"description": "desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Specific step 1", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 2", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 3", "description": "desc", "order_index": 3, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	if !errors.Is(err, services.ErrAIInvalidOutput) {
		t.Fatalf("expected ErrAIInvalidOutput, got %v", err)
	}
}

func TestGenerateRoadmap_WordyJsonOk(t *testing.T) {
	raw := `Here is the roadmap you requested:

` + "```json\n" + validAIResponseJsonString() + "\n```" + `

I hope this helps you learn Flutter!`
	svc, userID, _ := setupGenerateService(t, raw, nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error with wordy response: %v", err)
	}
	if result.GeneratedStepCount != 5 {
		t.Errorf("expected 5 steps, got %d", result.GeneratedStepCount)
	}
}

func TestGenerateRoadmap_AutoFollowedForUserList(t *testing.T) {
	svc, userID, db := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("first generation error: %v", err)
	}

	var count int64
	db.Model(&models.UserLearningRoadmap{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 follow record for generated roadmap, got %d", count)
	}
}

func TestGenerateRoadmap_CategoryIsLong(t *testing.T) {
	svc, userID, _ := setupGenerateService(t, validAIResponseJsonString(), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			Category:     strings.Repeat("x", 81),
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err == nil {
		t.Fatal("expected error for long category")
	}
}

func TestGenerateRoadmap_EmptyStepsDropped(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Specific step 1", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 2", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "", "description": "desc", "order_index": 3, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 4", "description": "", "order_index": 4, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 5", "description": "desc", "order_index": 5, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GeneratedStepCount != 3 {
		t.Errorf("expected 3 valid steps (2 dropped), got %d", result.GeneratedStepCount)
	}
}

func TestGenerateRoadmap_DifficultyOverride(t *testing.T) {
	resp := map[string]interface{}{
		"title":       "Test",
		"description": "desc",
		"category":    "Test",
		"difficulty":  "beginner",
		"steps": []map[string]interface{}{
			{"title": "Specific step 1", "description": "desc", "order_index": 1, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 2", "description": "desc", "order_index": 2, "estimated_minutes": 20, "outcome": "ok"},
			{"title": "Specific step 3", "description": "desc", "order_index": 3, "estimated_minutes": 20, "outcome": "ok"},
		},
	}
	b, _ := json.Marshal(resp)
	svc, userID, _ := setupGenerateService(t, string(b), nil)

	req := dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter",
			Difficulty:   "advanced",
			MaxDuration:  300,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := svc.GenerateRoadmapFromPreferences(ctx, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item.Difficulty != "advanced" {
		t.Errorf("expected difficulty='advanced' (user override), got '%s'", result.Item.Difficulty)
	}
}
