package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/testutils"
)

func validGenerateRoadmapRequest() dto.GenerateLearningRoadmapRequest {
	return dto.GenerateLearningRoadmapRequest{
		Preferences: dto.TemplateSuggestPreferences{
			LearningGoal: "Học Flutter cơ bản",
			Category:     "Flutter",
			Difficulty:   "beginner",
			MaxDuration:  300,
		},
	}
}

func TestLearningRoadmapRequestHash_NormalizesWhitespaceAndCase(t *testing.T) {
	req1 := validGenerateRoadmapRequest()
	req1.Preferences.LearningGoal = "  Học Flutter Cơ Bản  "
	req1.Preferences.Category = " Flutter "
	req1.Preferences.Difficulty = " BEGINNER "

	req2 := validGenerateRoadmapRequest()
	req2.Preferences.LearningGoal = "học flutter cơ bản"
	req2.Preferences.Category = "flutter"
	req2.Preferences.Difficulty = "beginner"

	hash1, _, err := services.BuildLearningRoadmapRequestHash(req1)
	if err != nil {
		t.Fatal(err)
	}
	hash2, _, err := services.BuildLearningRoadmapRequestHash(req2)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected stable normalized hash, got %s and %s", hash1, hash2)
	}
}

func TestStartRoadmapGeneration_DeduplicatesSamePreferences(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	svc.SetAIClient(ai.NewMockClient(validAIResponseJsonString(), nil))
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})

	req := validGenerateRoadmapRequest()
	res1, err := svc.StartRoadmapGeneration(context.Background(), userID, req)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := svc.StartRoadmapGeneration(context.Background(), userID, req)
	if err != nil {
		t.Fatal(err)
	}
	if res1.JobID != res2.JobID {
		t.Fatalf("expected duplicate request to reuse job, got %s and %s", res1.JobID, res2.JobID)
	}

	var count int64
	db.Model(&models.LearningRoadmapGenerationJob{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 job, got %d", count)
	}
}

func TestStartRoadmapGeneration_DifferentPreferencesCreateDifferentJobs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	svc.SetAIClient(ai.NewMockClient(validAIResponseJsonString(), nil))
	svc.SetRoadmapGenerationWorkerLauncher(func(uuid.UUID) {})

	req1 := validGenerateRoadmapRequest()
	req2 := validGenerateRoadmapRequest()
	req2.Preferences.LearningGoal = "Học Go concurrency"

	res1, err := svc.StartRoadmapGeneration(context.Background(), userID, req1)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := svc.StartRoadmapGeneration(context.Background(), userID, req2)
	if err != nil {
		t.Fatal(err)
	}
	if res1.JobID == res2.JobID {
		t.Fatal("expected different preferences to create different jobs")
	}
}

func TestGetRoadmapGenerationStatus_Generating(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(validGenerateRoadmapRequest())
	now := time.Now()
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusGenerating,
		StartedAt:   &now,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	status, err := svc.GetRoadmapGenerationStatus(context.Background(), userID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "generating" {
		t.Fatalf("expected generating, got %s", status.Status)
	}
	if status.Item != nil {
		t.Fatal("expected no item while generating")
	}
}

func TestProcessRoadmapGenerationJob_SuccessCompletesJob(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	svc.SetAIClient(ai.NewMockClient(validAIResponseJsonString(), nil))
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(validGenerateRoadmapRequest())
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusPending,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	svc.ProcessRoadmapGenerationJob(context.Background(), job.ID)

	var reloaded models.LearningRoadmapGenerationJob
	if err := db.First(&reloaded, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != models.LearningRoadmapGenJobStatusCompleted {
		t.Fatalf("expected completed, got %s (%v)", reloaded.Status, reloaded.ErrorMessage)
	}
	if reloaded.RoadmapID == nil {
		t.Fatal("expected roadmap_id")
	}
	if reloaded.GeneratedStepCount != 5 {
		t.Fatalf("expected 5 steps, got %d", reloaded.GeneratedStepCount)
	}

	status, err := svc.GetRoadmapGenerationStatus(context.Background(), userID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Item == nil || len(status.Item.Steps) != 5 {
		t.Fatalf("expected completed status with item steps, got %#v", status.Item)
	}
}

func TestProcessRoadmapGenerationJob_FailureUsesSafeError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	svc.SetAIClient(ai.NewMockClient("", errors.New("provider token leaked detail")))
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(validGenerateRoadmapRequest())
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusPending,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	svc.ProcessRoadmapGenerationJob(context.Background(), job.ID)

	status, err := svc.GetRoadmapGenerationStatus(context.Background(), userID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "failed" {
		t.Fatalf("expected failed, got %s", status.Status)
	}
	if status.Error == nil || status.Error.Type != models.LearningRoadmapGenErrorAI {
		t.Fatalf("expected safe AI error, got %#v", status.Error)
	}
	if status.Error.Message == "provider token leaked detail" {
		t.Fatal("raw provider error leaked to status response")
	}
}

func TestStartRoadmapGeneration_InvalidRequestCreatesNoJob(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	req := validGenerateRoadmapRequest()
	req.Preferences.LearningGoal = ""

	_, err := svc.StartRoadmapGeneration(context.Background(), userID, req)
	if !errors.Is(err, services.ErrEmptyLearningGoal) {
		t.Fatalf("expected ErrEmptyLearningGoal, got %v", err)
	}
	var count int64
	db.Model(&models.LearningRoadmapGenerationJob{}).Where("user_id = ?", userID).Count(&count)
	if count != 0 {
		t.Fatalf("expected no job for invalid request, got %d", count)
	}
}

func TestGetRoadmapGenerationStatus_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	otherUser := uuid.New()
	testutils.CreateTestUser(db, otherUser, "other@example.com")
	svc := services.NewLearningRoadmapService(db)
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(validGenerateRoadmapRequest())
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusGenerating,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.GetRoadmapGenerationStatus(context.Background(), otherUser, job.ID)
	if !errors.Is(err, services.ErrRoadmapNotFound) {
		t.Fatalf("expected not found for other user, got %v", err)
	}
}

func TestProcessRoadmapGenerationJob_CompletedJobDoesNotDuplicateRoadmap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	svc.SetAIClient(ai.NewMockClient(validAIResponseJsonString(), nil))
	hash, prefs, _ := services.BuildLearningRoadmapRequestHash(validGenerateRoadmapRequest())
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: hash,
		Preferences: prefs,
		Status:      models.LearningRoadmapGenJobStatusPending,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	svc.ProcessRoadmapGenerationJob(context.Background(), job.ID)
	svc.ProcessRoadmapGenerationJob(context.Background(), job.ID)

	var count int64
	db.Model(&models.LearningRoadmap{}).
		Where("created_by_user_id = ? AND source = ?", userID, models.LearningRoadmapSourceAI).
		Count(&count)
	if count != 1 {
		t.Fatalf("expected no duplicate roadmap for completed job retry, got %d", count)
	}
}

func TestDecodeRoadmapGenerationRequestRejectsInvalidPreferences(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })
	userID := testutils.BootstrapTestUser(t, db)
	svc := services.NewLearningRoadmapService(db)
	job := models.LearningRoadmapGenerationJob{
		UserID:      userID,
		RequestHash: "bad",
		Preferences: datatypes.JSON([]byte(`{"learning_goal":""}`)),
		Status:      models.LearningRoadmapGenJobStatusPending,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	svc.ProcessRoadmapGenerationJob(context.Background(), job.ID)
	status, err := svc.GetRoadmapGenerationStatus(context.Background(), userID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "failed" || status.Error == nil || status.Error.Type != models.LearningRoadmapGenErrorValidation {
		t.Fatalf("expected validation failure, got %#v", status)
	}
}
