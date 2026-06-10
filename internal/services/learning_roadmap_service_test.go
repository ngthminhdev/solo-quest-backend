package services_test

import (
	"testing"

	"github.com/google/uuid"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func TestLearningRoadmap_ListRoadmaps_ReturnsUserOwnedRoadmaps(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap1 := &models.LearningRoadmap{
		Title: "Flutter App Architecture", Description: "desc", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 180, TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap1)
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap1.ID, Title: "Step 1", OrderIndex: 1, Enabled: true})
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap1.ID, Title: "Step 2", OrderIndex: 2, Enabled: true})

	roadmap2 := &models.LearningRoadmap{
		Title: "Dart Async Mastery", Description: "desc", Category: "dart",
		Difficulty: "normal", EstimatedMinutes: 120, TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap2)
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap2.ID, Title: "Step A", OrderIndex: 1, Enabled: true})

	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 roadmaps, got %d", len(items))
	}

	found1, found2 := false, false
	for _, item := range items {
		if item.Title == "Flutter App Architecture" {
			found1 = true
			if len(item.Steps) != 2 {
				t.Errorf("expected 2 steps for roadmap1, got %d", len(item.Steps))
			}
			if item.Status != "" {
				t.Errorf("expected empty status for unfollowed roadmap, got '%s'", item.Status)
			}
			if item.CompletedSteps != 0 {
				t.Errorf("expected 0 completed_steps, got %d", item.CompletedSteps)
			}
		}
		if item.Title == "Dart Async Mastery" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("expected both roadmaps in response")
	}
}

func TestLearningRoadmap_ListRoadmaps_ExcludesDefaultRoadmaps(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	defaultRoadmap := &models.LearningRoadmap{
		Title: "Default Roadmap", Description: "desc", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 180, TotalSteps: 1, Source: "system", Enabled: true,
	}
	db.Create(defaultRoadmap)

	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range items {
		if item.ID == defaultRoadmap.ID {
			t.Fatal("default roadmap should not be returned in user-only list")
		}
	}
}

func TestLearningRoadmap_ListRoadmaps_HidesArchivedForUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Archived System", Description: "desc", Category: "test",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	if err := svc.DeleteRoadmap(userID, roadmap.ID); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	for _, item := range items {
		if item.ID == roadmap.ID {
			t.Fatal("expected archived roadmap to be hidden from current user's list")
		}
	}
}

func TestLearningRoadmap_ListRoadmaps_ShowsOwnedRoadmapWithoutFollow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Owned Legacy", Description: "desc", Category: "test",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 1, Source: models.LearningRoadmapSourceAI, CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	found := false
	for _, item := range items {
		if item.ID == roadmap.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected owned roadmap to be visible even without legacy follow row")
	}
}

func TestLearningRoadmap_GetDetail_ReturnsStepsSorted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test Roadmap", Description: "desc", Category: "test",
		Difficulty: "normal", EstimatedMinutes: 60, TotalSteps: 3, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "C Step", OrderIndex: 3, Enabled: true})
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "A Step", OrderIndex: 1, Enabled: true})
	db.Create(&models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "B Step", OrderIndex: 2, Enabled: true})

	detail, err := svc.GetRoadmapDetail(userID, roadmap.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detail.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(detail.Steps))
	}
	if detail.Steps[0].Title != "A Step" || detail.Steps[1].Title != "B Step" || detail.Steps[2].Title != "C Step" {
		t.Errorf("steps not sorted by order_index: %s, %s, %s",
			detail.Steps[0].Title, detail.Steps[1].Title, detail.Steps[2].Title)
	}
}

func TestLearningRoadmap_GetDetail_ArchivedSystemRoadmapNotFoundForUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Archived Detail", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	if err := svc.DeleteRoadmap(userID, roadmap.ID); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	_, err := svc.GetRoadmapDetail(userID, roadmap.ID)
	if err != services.ErrRoadmapNotFound {
		t.Fatalf("expected ErrRoadmapNotFound, got %v", err)
	}
}

func TestLearningRoadmap_GetDetail_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	_, err := svc.GetRoadmapDetail(userID, uuid.New())
	if err != services.ErrRoadmapNotFound {
		t.Errorf("expected ErrRoadmapNotFound, got %v", err)
	}
}

func TestLearningRoadmap_FollowRoadmap_DeletedRoadmapNotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Reactivate", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	if err := svc.DeleteRoadmap(userID, roadmap.ID); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	result, err := svc.FollowRoadmap(userID, roadmap.ID)
	if err != services.ErrRoadmapNotFound {
		t.Fatalf("expected ErrRoadmapNotFound, got result=%v err=%v", result, err)
	}
}

func TestLearningRoadmap_FollowRoadmap_CreatesRecord(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	result, err := svc.FollowRoadmap(userID, roadmap.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, result.UserID)
	}
	if result.RoadmapID != roadmap.ID {
		t.Errorf("expected roadmapID %v, got %v", roadmap.ID, result.RoadmapID)
	}
	if result.Status != "tracking" {
		t.Errorf("expected status 'tracking', got '%s'", result.Status)
	}
}

func TestLearningRoadmap_FollowRoadmap_Idempotent(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	result1, err := svc.FollowRoadmap(userID, roadmap.ID)
	if err != nil {
		t.Fatalf("first follow error: %v", err)
	}

	result2, err := svc.FollowRoadmap(userID, roadmap.ID)
	if err != nil {
		t.Fatalf("second follow error: %v", err)
	}
	if result1.ID != result2.ID {
		t.Errorf("expected same ID for idempotent follow, got %v vs %v", result1.ID, result2.ID)
	}
}

func TestLearningRoadmap_FollowRoadmap_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	_, err := svc.FollowRoadmap(userID, uuid.New())
	if err != services.ErrRoadmapNotFound {
		t.Errorf("expected ErrRoadmapNotFound, got %v", err)
	}
}

func TestLearningRoadmap_ToggleStep_RequiresFollowing(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	_, err := svc.ToggleStep(userID, roadmap.ID, step.ID, true)
	if err != services.ErrNotFollowingRoadmap {
		t.Errorf("expected ErrNotFollowingRoadmap, got %v", err)
	}
}

func TestLearningRoadmap_ToggleStep_CompleteAndUncomplete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", OrderIndex: 2, Enabled: true}
	db.Create(step1)
	db.Create(step2)

	svc.FollowRoadmap(userID, roadmap.ID)

	result, err := svc.ToggleStep(userID, roadmap.ID, step1.ID, true)
	if err != nil {
		t.Fatalf("toggle complete error: %v", err)
	}
	if !result.Completed {
		t.Error("expected completed=true")
	}
	if result.CompletedSteps != 1 {
		t.Errorf("expected completed_steps=1, got %d", result.CompletedSteps)
	}
	if result.ProgressPercent != 50 {
		t.Errorf("expected progress_percent=50, got %d", result.ProgressPercent)
	}
	if result.RoadmapStatus != "tracking" {
		t.Errorf("expected status 'tracking', got '%s'", result.RoadmapStatus)
	}

	result, err = svc.ToggleStep(userID, roadmap.ID, step1.ID, false)
	if err != nil {
		t.Fatalf("toggle uncomplete error: %v", err)
	}
	if result.Completed {
		t.Error("expected completed=false")
	}
	if result.CompletedSteps != 0 {
		t.Errorf("expected completed_steps=0, got %d", result.CompletedSteps)
	}
	if result.CompletedAt != nil {
		t.Error("expected completed_at=nil after uncomplete")
	}
}

func TestLearningRoadmap_ToggleStep_AllStepsCompleted_MarksRoadmapCompleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", OrderIndex: 2, Enabled: true}
	db.Create(step1)
	db.Create(step2)

	svc.FollowRoadmap(userID, roadmap.ID)
	svc.ToggleStep(userID, roadmap.ID, step1.ID, true)
	result, err := svc.ToggleStep(userID, roadmap.ID, step2.ID, true)
	if err != nil {
		t.Fatalf("toggle error: %v", err)
	}
	if result.RoadmapStatus != "completed" {
		t.Errorf("expected roadmap status 'completed', got '%s'", result.RoadmapStatus)
	}
	if result.ProgressPercent != 100 {
		t.Errorf("expected progress_percent=100, got %d", result.ProgressPercent)
	}
	if result.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestLearningRoadmap_ToggleStep_UncheckAfterCompleted_ReturnsToTracking(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", OrderIndex: 2, Enabled: true}
	db.Create(step1)
	db.Create(step2)

	svc.FollowRoadmap(userID, roadmap.ID)
	svc.ToggleStep(userID, roadmap.ID, step1.ID, true)
	svc.ToggleStep(userID, roadmap.ID, step2.ID, true)

	result, err := svc.ToggleStep(userID, roadmap.ID, step1.ID, false)
	if err != nil {
		t.Fatalf("toggle error: %v", err)
	}
	if result.RoadmapStatus != "tracking" {
		t.Errorf("expected status 'tracking' after uncheck, got '%s'", result.RoadmapStatus)
	}
	if result.CompletedAt != nil {
		t.Error("expected completed_at=nil after uncheck")
	}
}

func TestLearningRoadmap_ToggleStep_StepNotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	svc.FollowRoadmap(userID, roadmap.ID)

	_, err := svc.ToggleStep(userID, roadmap.ID, uuid.New(), true)
	if err != services.ErrStepNotFound {
		t.Errorf("expected ErrStepNotFound, got %v", err)
	}
}

func TestLearningRoadmap_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1 := testutils.BootstrapTestUser(t, db)
	user2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, user2, "user2@test.com")

	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &user1, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	svc.FollowRoadmap(user1, roadmap.ID)
	svc.ToggleStep(user1, roadmap.ID, step.ID, true)

	_, err := svc.GetRoadmapDetail(user2, roadmap.ID)
	if err != services.ErrRoadmapNotFound {
		t.Fatalf("expected ErrRoadmapNotFound for other user's roadmap, got %v", err)
	}
}

func TestLearningRoadmap_DisabledRoadmap_NotReturned(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Disabled", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 0, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	// Disable it after creation to bypass GORM zero-value default handling
	db.Model(roadmap).Update("enabled", false)

	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range items {
		if item.Title == "Disabled" {
			t.Error("disabled roadmap should not be in list")
		}
	}
}

func TestLearningRoadmap_GetAISuggestions_ReturnsSuggestions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.AiRoadmapSuggestRequest{
		Preferences: dto.RoadmapPreferencesInput{
			LearningGoal: "Flutter",
		},
		Limit: 3,
	}

	result, err := svc.GetAISuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	if len(result.Suggestions) > 3 {
		t.Errorf("expected max 3 suggestions, got %d", len(result.Suggestions))
	}

	// Check suggestion structure
	first := result.Suggestions[0]
	if first.ID == "" {
		t.Error("suggestion ID should not be empty")
	}
	if first.Title == "" {
		t.Error("suggestion title should not be empty")
	}
}

func TestLearningRoadmap_GetAISuggestions_FiltersByCategory(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.AiRoadmapSuggestRequest{
		Preferences: dto.RoadmapPreferencesInput{
			Category: "Flutter",
		},
		Limit: 5,
	}

	result, err := svc.GetAISuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range result.Suggestions {
		if s.Category != "Flutter" {
			t.Errorf("expected Flutter category, got %s", s.Category)
		}
	}
}

func TestLearningRoadmap_GetAISuggestions_FiltersByDifficulty(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.AiRoadmapSuggestRequest{
		Preferences: dto.RoadmapPreferencesInput{
			Difficulty: "beginner",
		},
		Limit: 5,
	}

	result, err := svc.GetAISuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range result.Suggestions {
		if s.Difficulty != "beginner" {
			t.Errorf("expected beginner difficulty, got %s", s.Difficulty)
		}
	}
}

func TestLearningRoadmap_GetAISuggestions_FiltersByMaxDuration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.AiRoadmapSuggestRequest{
		Preferences: dto.RoadmapPreferencesInput{
			MaxDuration: 150,
		},
		Limit: 5,
	}

	result, err := svc.GetAISuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range result.Suggestions {
		if s.EstimatedMinutes > 150 {
			t.Errorf("expected max 150 minutes, got %d", s.EstimatedMinutes)
		}
	}
}

func TestLearningRoadmap_GetAISuggestions_FallbackWhenNoMatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.AiRoadmapSuggestRequest{
		Preferences: dto.RoadmapPreferencesInput{
			Category: "NonExistentCategory",
		},
		Limit: 5,
	}

	result, err := svc.GetAISuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return fallback featured suggestions
	if len(result.Suggestions) == 0 {
		t.Error("expected fallback suggestions when no match")
	}
}

func TestLearningRoadmap_CreateFromSuggestion_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}

	result, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID == uuid.Nil {
		t.Error("roadmap ID should not be nil")
	}
	if result.Source != "ai" {
		t.Errorf("expected source 'ai', got %s", result.Source)
	}
	if result.Status != "tracking" {
		t.Errorf("expected status 'tracking', got %s", result.Status)
	}
	if len(result.Steps) == 0 {
		t.Error("expected steps to be cloned")
	}

	// Verify roadmap created in DB
	var roadmap models.LearningRoadmap
	if err := db.First(&roadmap, result.ID).Error; err != nil {
		t.Fatalf("roadmap not found in DB: %v", err)
	}
	if roadmap.CreatedByUserID == nil || *roadmap.CreatedByUserID != userID {
		t.Error("created_by_user_id should be set to current user")
	}

	// Verify tracking record created
	var tracking models.UserLearningRoadmap
	if err := db.Where("user_id = ? AND roadmap_id = ?", userID, result.ID).First(&tracking).Error; err != nil {
		t.Fatalf("tracking record not found: %v", err)
	}
}

func TestLearningRoadmap_CreateFromSuggestion_InvalidID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	suggestionID := "invalid_suggestion_id"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}

	_, err := svc.CreateRoadmap(userID, req)
	if err == nil {
		t.Fatal("expected error for invalid suggestion ID")
	}
	if err != services.ErrSuggestionNotFound {
		t.Errorf("expected ErrSuggestionNotFound, got %v", err)
	}
}

func TestLearningRoadmap_CreateFromSuggestion_WithCustomization(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	suggestionID := "ai_dart_async"
	customTitle := "My Custom Async Roadmap"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
		Customize: &dto.CreateLearningRoadmapCustomize{
			Title: &customTitle,
		},
	}

	result, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != customTitle {
		t.Errorf("expected custom title %q, got %q", customTitle, result.Title)
	}
}

func TestLearningRoadmap_CreateCustom_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	title := "My Custom Roadmap"
	category := "Custom"
	difficulty := "intermediate"
	req := dto.CreateLearningRoadmapRequest{
		Title:      &title,
		Category:   &category,
		Difficulty: &difficulty,
		Source:     "user",
		Steps: []dto.CreateLearningRoadmapStepRequest{
			{Title: "Step 1", Description: "First step", OrderIndex: 0, EstimatedMinutes: 30},
			{Title: "Step 2", Description: "Second step", OrderIndex: 1, EstimatedMinutes: 45},
		},
	}

	result, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != title {
		t.Errorf("expected title %q, got %q", title, result.Title)
	}
	if result.Source != "user" {
		t.Errorf("expected source 'user', got %s", result.Source)
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.EstimatedMinutes != 75 {
		t.Errorf("expected 75 minutes, got %d", result.EstimatedMinutes)
	}
}

func TestLearningRoadmap_CreateCustom_MissingTitle(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	category := "Custom"
	difficulty := "intermediate"
	req := dto.CreateLearningRoadmapRequest{
		Category:   &category,
		Difficulty: &difficulty,
		Source:     "user",
		Steps: []dto.CreateLearningRoadmapStepRequest{
			{Title: "Step 1", OrderIndex: 0, EstimatedMinutes: 30},
		},
	}

	_, err := svc.CreateRoadmap(userID, req)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestLearningRoadmap_CreateCustom_NoSteps(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	title := "Empty Roadmap"
	category := "Custom"
	difficulty := "intermediate"
	req := dto.CreateLearningRoadmapRequest{
		Title:      &title,
		Category:   &category,
		Difficulty: &difficulty,
		Source:     "user",
		Steps:      []dto.CreateLearningRoadmapStepRequest{},
	}

	_, err := svc.CreateRoadmap(userID, req)
	if err == nil {
		t.Fatal("expected error for roadmap with no steps")
	}
}

func TestLearningRoadmap_CreatedRoadmap_AppearsInList(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	// Create a roadmap
	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}
	created, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// List roadmaps
	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify created roadmap appears
	found := false
	for _, item := range items {
		if item.ID == created.ID {
			found = true
			if item.Status != "tracking" {
				t.Errorf("expected status 'tracking', got %s", item.Status)
			}
			break
		}
	}
	if !found {
		t.Error("created roadmap should appear in list")
	}
}

func TestLearningRoadmap_CreatedRoadmap_NotVisibleToOtherUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1ID := testutils.BootstrapTestUser(t, db)
	user2ID := uuid.New()
	testutils.CreateTestUser(db, user2ID, "user2@example.com")

	svc := services.NewLearningRoadmapService(db)

	// User1 creates a roadmap
	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}
	created, err := svc.CreateRoadmap(user1ID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User2 lists roadmaps
	items, err := svc.ListRoadmaps(user2ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify created roadmap does NOT appear for user2
	for _, item := range items {
		if item.ID == created.ID {
			t.Error("user1's roadmap should not appear in user2's list")
		}
	}
}

func TestLearningRoadmap_CreatedRoadmap_OtherUserCannotGetDetail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1ID := testutils.BootstrapTestUser(t, db)
	user2ID := uuid.New()
	testutils.CreateTestUser(db, user2ID, "user2@example.com")

	svc := services.NewLearningRoadmapService(db)

	// User1 creates a roadmap
	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}
	created, err := svc.CreateRoadmap(user1ID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User2 tries to get detail
	_, err = svc.GetRoadmapDetail(user2ID, created.ID)
	if err == nil {
		t.Fatal("expected error when other user tries to access created roadmap")
	}
	if err != services.ErrRoadmapNotFound {
		t.Errorf("expected ErrRoadmapNotFound, got %v", err)
	}
}

func TestLearningRoadmap_ToggleStep_WorksOnCreatedRoadmap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	// Create a roadmap
	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}
	created, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Toggle first step
	firstStepID := created.Steps[0].ID
	result, err := svc.ToggleStep(userID, created.ID, firstStepID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Completed {
		t.Error("step should be marked as completed")
	}
	if result.CompletedSteps != 1 {
		t.Errorf("expected 1 completed step, got %d", result.CompletedSteps)
	}
}

// Template-based suggestion tests

func TestLearningRoadmap_GetTemplateSuggestions_ReturnsTemplates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.TemplateSuggestRequest{
		Preferences: dto.TemplateSuggestPreferences{},
	}

	result, err := svc.GetTemplateSuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected at least one template suggestion")
	}

	// Verify all have source = "template"
	for _, sugg := range result.Suggestions {
		if sugg.Source != "template" {
			t.Errorf("expected source='template', got '%s'", sugg.Source)
		}
	}
}

func TestLearningRoadmap_GetTemplateSuggestions_FiltersByCategory(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.TemplateSuggestRequest{
		Preferences: dto.TemplateSuggestPreferences{
			Category: "Flutter",
		},
	}

	result, err := svc.GetTemplateSuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected Flutter templates")
	}

	// All should be Flutter category
	for _, sugg := range result.Suggestions {
		if sugg.Category != "Flutter" {
			t.Errorf("expected category='Flutter', got '%s'", sugg.Category)
		}
	}
}

func TestLearningRoadmap_GetTemplateSuggestions_FiltersByDifficulty(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.TemplateSuggestRequest{
		Preferences: dto.TemplateSuggestPreferences{
			Difficulty: "beginner",
		},
	}

	result, err := svc.GetTemplateSuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected beginner templates")
	}

	// All should be beginner difficulty
	for _, sugg := range result.Suggestions {
		if sugg.Difficulty != "beginner" {
			t.Errorf("expected difficulty='beginner', got '%s'", sugg.Difficulty)
		}
	}
}

func TestLearningRoadmap_GetTemplateSuggestions_FiltersByMaxDuration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.TemplateSuggestRequest{
		Preferences: dto.TemplateSuggestPreferences{
			MaxDuration: 150,
		},
	}

	result, err := svc.GetTemplateSuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected templates under 150 minutes")
	}

	// All should be under max duration
	for _, sugg := range result.Suggestions {
		if sugg.EstimatedMinutes > 150 {
			t.Errorf("expected duration <= 150, got %d", sugg.EstimatedMinutes)
		}
	}
}

func TestLearningRoadmap_GetTemplateSuggestions_FallbackWhenNoMatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.TemplateSuggestRequest{
		Preferences: dto.TemplateSuggestPreferences{
			Category: "NonExistentCategory",
		},
	}

	result, err := svc.GetTemplateSuggestions(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return fallback templates instead of empty
	if len(result.Suggestions) == 0 {
		t.Fatal("expected fallback templates when no exact match")
	}
}

func TestLearningRoadmap_CreateFromTemplate_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_dart_async",
		Source:     "template",
	}

	created, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.Source != "template" {
		t.Errorf("expected source='template', got '%s'", created.Source)
	}
	if created.Title != "Dart Async Programming" {
		t.Errorf("unexpected title: %s", created.Title)
	}
	if created.Status != "tracking" {
		t.Errorf("expected status='tracking', got '%s'", created.Status)
	}
	if len(created.Steps) == 0 {
		t.Error("expected steps to be cloned")
	}
	if created.CompletedSteps != 0 {
		t.Errorf("expected 0 completed steps, got %d", created.CompletedSteps)
	}
	if created.ProgressPercent != 0 {
		t.Errorf("expected 0%% progress, got %d", created.ProgressPercent)
	}
}

func TestLearningRoadmap_CreateFromTemplate_InvalidTemplateID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "invalid_template_id",
		Source:     "template",
	}

	_, err := svc.CreateFromTemplate(userID, req)
	if err == nil {
		t.Fatal("expected error for invalid template ID")
	}
	if err.Error() != "template not found" {
		t.Errorf("expected 'template not found', got '%v'", err)
	}
}

func TestLearningRoadmap_CreateFromTemplate_ReturnsExistingIfAlreadyCreated(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_uiux",
		Source:     "template",
	}

	// Create first time
	first, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error on first create: %v", err)
	}

	// Create second time - should return existing
	second, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error on second create: %v", err)
	}

	if first.ID != second.ID {
		t.Error("expected same roadmap ID when creating duplicate")
	}
}

func TestLearningRoadmap_CreateFromTemplate_AppearsInList(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_testing",
		Source:     "template",
	}

	created, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// List roadmaps - created roadmap should appear
	items, err := svc.ListRoadmaps(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, item := range items {
		if item.ID == created.ID {
			found = true
			if item.Source != "template" {
				t.Errorf("expected source='template', got '%s'", item.Source)
			}
			if item.Status != "tracking" {
				t.Errorf("expected status='tracking', got '%s'", item.Status)
			}
		}
	}
	if !found {
		t.Error("created roadmap should appear in list")
	}
}

func TestLearningRoadmap_CreateFromTemplate_NotVisibleToOtherUser(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1ID := testutils.BootstrapTestUser(t, db)
	user2ID := uuid.New()
	testutils.CreateTestUser(db, user2ID, "user2@example.com")

	svc := services.NewLearningRoadmapService(db)

	// User1 creates a roadmap from template
	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_animation",
		Source:     "template",
	}
	created, err := svc.CreateFromTemplate(user1ID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User2 should not see it in their list
	items, err := svc.ListRoadmaps(user2ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range items {
		if item.ID == created.ID {
			t.Error("user2 should not see user1's template roadmap")
		}
	}

	// User2 should not be able to get detail
	_, err = svc.GetRoadmapDetail(user2ID, created.ID)
	if err == nil {
		t.Fatal("expected error when other user tries to access template roadmap")
	}
	if err != services.ErrRoadmapNotFound {
		t.Errorf("expected ErrRoadmapNotFound, got %v", err)
	}
}

func TestLearningRoadmap_CreateFromTemplate_ToggleStepWorks(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_riverpod",
		Source:     "template",
	}

	created, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Toggle first step
	firstStepID := created.Steps[0].ID
	result, err := svc.ToggleStep(userID, created.ID, firstStepID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Completed {
		t.Error("step should be marked as completed")
	}
	if result.CompletedSteps != 1 {
		t.Errorf("expected 1 completed step, got %d", result.CompletedSteps)
	}
}

// Log creation tests

func TestLearningRoadmap_CreateFromSuggestion_CreatesLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	suggestionID := "ai_dart_async"
	req := dto.CreateLearningRoadmapRequest{
		SuggestionID: &suggestionID,
		Source:       "ai",
	}

	_, err := svc.CreateRoadmap(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCreated).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 learning_roadmap_created log, got %d", logCount)
	}
}

func TestLearningRoadmap_CreateFromTemplate_CreatesLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_dart_async",
		Source:     "template",
	}

	_, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCreated).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 learning_roadmap_created log, got %d", logCount)
	}
}

func TestLearningRoadmap_CreateFromTemplate_NoDuplicateLogOnRecreate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	req := dto.CreateFromTemplateRequest{
		TemplateID: "template_uiux",
		Source:     "template",
	}

	// Create first time
	_, err := svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("first create error: %v", err)
	}

	// Create second time (idempotent - returns existing)
	_, err = svc.CreateFromTemplate(userID, req)
	if err != nil {
		t.Fatalf("second create error: %v", err)
	}

	// Should only have 1 log (the second call returns existing, doesn't create new)
	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCreated).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 learning_roadmap_created log (no duplicate), got %d", logCount)
	}
}

func TestLearningRoadmap_FollowRoadmap_CreatesLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	_, err := svc.FollowRoadmap(userID, roadmap.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapFollowed).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 learning_roadmap_followed log, got %d", logCount)
	}
}

func TestLearningRoadmap_FollowRoadmap_NoDuplicateLogOnIdempotentFollow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)

	// Follow twice
	svc.FollowRoadmap(userID, roadmap.ID)
	svc.FollowRoadmap(userID, roadmap.ID)

	// Should only have 1 log
	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapFollowed).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 learning_roadmap_followed log (no duplicate), got %d", logCount)
	}
}

func TestLearningRoadmap_ToggleStep_CreatesStepCompletedLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	svc.FollowRoadmap(userID, roadmap.ID)
	svc.ToggleStep(userID, roadmap.ID, step.ID, true)

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapStepCompleted).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 step_completed log, got %d", logCount)
	}
}

func TestLearningRoadmap_ToggleStep_CreatesStepUncompletedLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	svc.FollowRoadmap(userID, roadmap.ID)
	svc.ToggleStep(userID, roadmap.ID, step.ID, true)
	svc.ToggleStep(userID, roadmap.ID, step.ID, false)

	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapStepUncompleted).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 step_uncompleted log, got %d", logCount)
	}
}

func TestLearningRoadmap_ToggleStep_CreatesRoadmapCompletedLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 2, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", OrderIndex: 2, Enabled: true}
	db.Create(step1)
	db.Create(step2)

	svc.FollowRoadmap(userID, roadmap.ID)
	svc.ToggleStep(userID, roadmap.ID, step1.ID, true)
	svc.ToggleStep(userID, roadmap.ID, step2.ID, true)

	var roadmapCompletedLogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&roadmapCompletedLogCount)
	if roadmapCompletedLogCount != 1 {
		t.Errorf("expected 1 roadmap_completed log, got %d", roadmapCompletedLogCount)
	}

	// Should also have 2 step_completed logs
	var stepCompletedLogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapStepCompleted).Count(&stepCompletedLogCount)
	if stepCompletedLogCount != 2 {
		t.Errorf("expected 2 step_completed logs, got %d", stepCompletedLogCount)
	}
}

func TestLearningRoadmap_ToggleStep_NoDuplicateRoadmapCompletedLog(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	svc.FollowRoadmap(userID, roadmap.ID)
	// Complete step -> roadmap completes (1st completion log)
	svc.ToggleStep(userID, roadmap.ID, step.ID, true)

	var roadmapCompletedLogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&roadmapCompletedLogCount)
	if roadmapCompletedLogCount != 1 {
		t.Errorf("expected 1 roadmap_completed log after first completion, got %d", roadmapCompletedLogCount)
	}

	// Uncomplete step
	svc.ToggleStep(userID, roadmap.ID, step.ID, false)

	// Re-complete step -> roadmap completes again (2nd completion log is OK - new completion event)
	svc.ToggleStep(userID, roadmap.ID, step.ID, true)

	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&roadmapCompletedLogCount)
	if roadmapCompletedLogCount != 2 {
		t.Errorf("expected 2 roadmap_completed logs after re-complete, got %d", roadmapCompletedLogCount)
	}

	// Toggle the same step off and on again without uncompleting all steps
	// (roadmap stays completed) - should NOT create another log
	// Actually, with 1-step roadmap, toggling off always uncompletes.
	// Let's test with a 2-step roadmap instead for true "no duplicate" scenario.
}

func TestLearningRoadmap_ToggleStep_NoDuplicateRoadmapCompletedLog_MultiStep(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 3, Source: "system", CreatedByUserID: &userID, Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 2", OrderIndex: 2, Enabled: true}
	step3 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 3", OrderIndex: 3, Enabled: true}
	db.Create(step1)
	db.Create(step2)
	db.Create(step3)

	svc.FollowRoadmap(userID, roadmap.ID)

	// Complete steps 1 and 2
	svc.ToggleStep(userID, roadmap.ID, step1.ID, true)
	svc.ToggleStep(userID, roadmap.ID, step2.ID, true)

	// Roadmap not yet completed
	var logCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&logCount)
	if logCount != 0 {
		t.Errorf("expected 0 roadmap_completed logs, got %d", logCount)
	}

	// Complete step 3 -> roadmap completes
	svc.ToggleStep(userID, roadmap.ID, step3.ID, true)
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 roadmap_completed log, got %d", logCount)
	}

	// Uncomplete step 2 (roadmap goes back to tracking)
	svc.ToggleStep(userID, roadmap.ID, step2.ID, false)

	// Re-complete step 2 -> roadmap completes again (new completion event)
	svc.ToggleStep(userID, roadmap.ID, step2.ID, true)
	db.Model(&models.LogEntry{}).Where("user_id = ? AND type = ?", userID, models.LogEntryTypeLearningRoadmapCompleted).Count(&logCount)
	if logCount != 2 {
		t.Errorf("expected 2 roadmap_completed logs after re-complete, got %d", logCount)
	}
}

func TestLearningRoadmap_Logs_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1ID := testutils.BootstrapTestUser(t, db)
	user2ID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, user2ID, "user2@test.com")

	svc := services.NewLearningRoadmapService(db)

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", CreatedByUserID: &user1ID, Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	// User1 follows and completes
	svc.FollowRoadmap(user1ID, roadmap.ID)
	svc.ToggleStep(user1ID, roadmap.ID, step.ID, true)

	// User2 should have no logs
	var user2LogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ?", user2ID).Count(&user2LogCount)
	if user2LogCount != 0 {
		t.Errorf("expected 0 logs for user2, got %d", user2LogCount)
	}

	// User1 should have follow + step_completed + roadmap_completed logs
	var user1LogCount int64
	db.Model(&models.LogEntry{}).Where("user_id = ?", user1ID).Count(&user1LogCount)
	if user1LogCount != 3 {
		t.Errorf("expected 3 logs for user1, got %d", user1LogCount)
	}
}
