package quest_generation_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/ai"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// --- F. Zero-UUID fix / synchronous rule-based path ------------------------

func TestGenerateToday_SyncRuleBased_ReturnsRealIDsAndTimestamps(t *testing.T) {
	db, userID, _, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	// Enable a sleep rule so at least one quest is always produced
	// regardless of the wall-clock hour (sleep quests survive the
	// past-reminder normalization, unlike daytime movement quests).
	catsJSON, _ := json.Marshal([]string{"movement", "learning", "sleep"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "easy", ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_sleep", Type: "sleep", Enabled: true, Difficulty: "easy", ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
	})
	if err := db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"enabled_categories": datatypes.JSON(catsJSON),
		"rules":              datatypes.JSON(rulesJSON),
	}).Error; err != nil {
		t.Fatal(err)
	}

	// Use the REAL rule-based generator so we exercise the DB insert path
	// that previously returned a zero UUID / zero timestamps.
	realRule := quest_generation.NewRuleBasedGenerator(db)
	svc := quest_generation.NewGenerationService(
		db, quest_generation.NewUserQuestContextBuilder(db), nil, realRule,
	)

	preferAI := false
	result, err := svc.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Source != "rule_based" {
		t.Fatalf("expected source rule_based, got %s", result.Source)
	}
	if len(result.Quests) == 0 {
		t.Fatal("expected at least one generated quest")
	}

	for i, q := range result.Quests {
		if q.ID == uuid.Nil {
			t.Errorf("quest[%d] has zero UUID", i)
		}
		if q.CreatedAt.IsZero() {
			t.Errorf("quest[%d] has zero CreatedAt", i)
		}
		if q.UpdatedAt.IsZero() {
			t.Errorf("quest[%d] has zero UpdatedAt", i)
		}
	}

	// The returned IDs must match the persisted rows.
	for _, q := range result.Quests {
		var dbQuest models.Quest
		if err := db.First(&dbQuest, "id = ?", q.ID).Error; err != nil {
			t.Errorf("quest %s not found in DB: %v", q.ID, err)
		}
	}
}

// --- C/D. Async start: 202 + duplicate-worker prevention -------------------

func TestStartTodayGeneration_Returns202AndPreventsDuplicateWorker(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	var launchCount int32
	var launchedID uuid.UUID
	service.SetWorkerLauncher(func(jobID uuid.UUID) {
		atomic.AddInt32(&launchCount, 1)
		launchedID = jobID
	})

	preferAI := true
	req := quest_generation.GenerateTodayRequest{PreferAI: &preferAI}

	res1, err := service.StartTodayGeneration(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1.Job == nil {
		t.Fatal("expected a background job (202), got existing result")
	}
	if res1.Job.Status != "generating" {
		t.Errorf("expected status generating, got %s", res1.Job.Status)
	}
	if res1.Job.JobID == "" {
		t.Error("expected a job_id")
	}

	// Second call while job is still generating must NOT spawn a 2nd worker
	// nor create a duplicate job.
	res2, err := service.StartTodayGeneration(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if res2.Job == nil || res2.Job.JobID != res1.Job.JobID {
		t.Errorf("expected the same job to be reused, got %+v", res2.Job)
	}

	if got := atomic.LoadInt32(&launchCount); got != 1 {
		t.Errorf("expected worker launched exactly once, got %d", got)
	}
	if launchedID.String() != res1.Job.JobID {
		t.Errorf("launched job id mismatch: %s vs %s", launchedID, res1.Job.JobID)
	}

	// Exactly one job row exists for the user/date.
	var jobCount int64
	db.Model(&models.DailyQuestGenerationJob{}).Where("user_id = ?", userID).Count(&jobCount)
	if jobCount != 1 {
		t.Errorf("expected 1 job row, got %d", jobCount)
	}
}

func TestStartTodayGeneration_ExistingQuestsReturnedSync(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	for i := 0; i < 5; i++ {
		db.Create(&models.Quest{
			ID: uuid.New(), UserID: userID, Title: "Existing", Date: today, Status: models.QuestStatusPending,
		})
	}

	var launched int32
	service.SetWorkerLauncher(func(uuid.UUID) { atomic.AddInt32(&launched, 1) })

	preferAI := true
	force := false
	res, err := service.StartTodayGeneration(context.Background(), userID, quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI, Force: &force,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Existing == nil {
		t.Fatal("expected existing quests to be returned synchronously")
	}
	if res.Job != nil {
		t.Error("no background job should be created when quests already exist")
	}
	if atomic.LoadInt32(&launched) != 0 {
		t.Error("worker should not be launched when quests already exist")
	}

	// No job row created.
	var jobCount int64
	db.Model(&models.DailyQuestGenerationJob{}).Where("user_id = ?", userID).Count(&jobCount)
	if jobCount != 0 {
		t.Errorf("expected 0 job rows, got %d", jobCount)
	}
}

func TestStartTodayGeneration_StaleJobReclaimedWithForce(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	day := timeutil.StartOfDayVN(timeutil.TodayVN())
	// Older than the 900s stale threshold.
	staleStart := timeutil.NowVN().Add(-16 * time.Minute)
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: day,
		Status: models.QuestGenJobStatusGenerating, StartedAt: &staleStart, PreferAI: true,
	}
	db.Create(&job)

	var launched int32
	var launchedID uuid.UUID
	service.SetWorkerLauncher(func(jobID uuid.UUID) {
		atomic.AddInt32(&launched, 1)
		launchedID = jobID
	})

	preferAI := true
	force := true
	res, err := service.StartTodayGeneration(context.Background(), userID, quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI, Force: &force,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Job == nil || res.Job.JobID != job.ID.String() {
		t.Fatalf("expected stale job to be reused, got %+v", res.Job)
	}
	if atomic.LoadInt32(&launched) != 1 {
		t.Errorf("expected stale job to be reclaimed and worker relaunched once, got %d", launched)
	}
	if launchedID != job.ID {
		t.Errorf("expected relaunch of job %s, got %s", job.ID, launchedID)
	}

	// started_at must have been refreshed to a recent time.
	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.StartedAt == nil || timeutil.NowVN().Sub(*reloaded.StartedAt) > time.Minute {
		t.Errorf("expected started_at to be refreshed, got %v", reloaded.StartedAt)
	}
}

// --- D. Worker: completion, fallback, failure ------------------------------

func TestProcessJob_CompletesWithAISource(t *testing.T) {
	db, userID, service, mockAI, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{Title: "AI Quest 1", Source: models.QuestSourceAI, Type: models.QuestTypeMovement, Date: today},
	}

	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(today),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed, got %s (err=%v)", reloaded.Status, reloaded.ErrorMessage)
	}
	if reloaded.Source == nil || *reloaded.Source != models.QuestGenJobSourceAI {
		t.Errorf("expected source ai, got %v", reloaded.Source)
	}
	if reloaded.GeneratedCount < 1 {
		t.Errorf("expected generated_count >= 1, got %d", reloaded.GeneratedCount)
	}
	if reloaded.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	var questCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&questCount)
	if questCount < 1 {
		t.Errorf("expected quests created, got %d", questCount)
	}
}

func TestProcessJob_AITimeoutFallsBackToRuleBased(t *testing.T) {
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.err = errors.New("context deadline exceeded: timeout")
	mockRule.quests = []models.Quest{
		{Title: "Rule Quest 1", Source: models.QuestSourceConfigBased, Type: models.QuestTypeMovement, Date: today},
	}

	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(today),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed, got %s", reloaded.Status)
	}
	if reloaded.Source == nil || *reloaded.Source != models.QuestGenJobSourceRuleBased {
		t.Errorf("expected source rule_based, got %v", reloaded.Source)
	}
	if !reloaded.FallbackUsed {
		t.Error("expected fallback_used true")
	}
	if reloaded.AIErrorType == nil || *reloaded.AIErrorType != "timeout" {
		t.Errorf("expected ai_error_type timeout, got %v", reloaded.AIErrorType)
	}

	var questCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&questCount)
	if questCount < 1 {
		t.Errorf("expected fallback quests created, got %d", questCount)
	}
}

func TestProcessJob_BothGeneratorsFailStillCompletesViaSmartFallback(t *testing.T) {
	// Even when both the AI and rule-based generators error, the job must not
	// fail: the smart fallback / last-resort fill deterministically produces the
	// daily target so the job completes.
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	mockAI.err = errors.New("boom timeout")
	mockRule.err = errors.New("rule generation exploded")

	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN()),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed via smart fallback, got %s (err=%v)", reloaded.Status, reloaded.ErrorMessage)
	}
	if !reloaded.FallbackUsed {
		t.Error("expected fallback_used true")
	}
	if reloaded.GeneratedCount != 5 {
		t.Errorf("expected generated_count 5 (smart fallback fill), got %d", reloaded.GeneratedCount)
	}

	var questCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&questCount)
	if questCount != 5 {
		t.Errorf("expected 5 quests persisted, got %d", questCount)
	}
}

func TestProcessJob_ZeroValidAIBelowTargetCompletesViaSmartFallback(t *testing.T) {
	// AI produced zero valid quests and there is no rule-based output, but the
	// job must still complete: the smart fallback fills the full target.
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	mockAI.err = errors.New("candidate validation failed: disabled quest type")
	mockRule.quests = nil

	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN()),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status == models.QuestGenJobStatusGenerating {
		t.Fatal("job must not remain generating after ProcessJob exits")
	}
	if reloaded.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed via smart fallback, got %s (err=%v)", reloaded.Status, reloaded.ErrorMessage)
	}
	if reloaded.CompletedAt == nil {
		t.Fatal("expected completed_at/finished_at to be set")
	}
	if reloaded.TargetCount != 5 {
		t.Errorf("expected target_count 5, got %d", reloaded.TargetCount)
	}
	if reloaded.GeneratedCount != 5 {
		t.Errorf("expected generated_count 5 (smart fallback fill), got %d", reloaded.GeneratedCount)
	}
	if !reloaded.FallbackUsed {
		t.Error("expected fallback_used true")
	}

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if status.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected status endpoint to return completed, got %s", status.Status)
	}
	if status.FinishedAt == nil {
		t.Fatal("expected status finished_at to be set")
	}
}

func TestProcessJob_ZeroGeneratedAtTargetMarksCompletedExisting(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	for i := 0; i < 5; i++ {
		db.Create(&models.Quest{
			ID: uuid.New(), UserID: userID, Title: "Existing target quest", Date: today, Status: models.QuestStatusPending,
		})
	}
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(today),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status != models.QuestGenJobStatusCompletedExisting {
		t.Fatalf("expected completed_existing, got %s", reloaded.Status)
	}
	if reloaded.GeneratedCount != 0 {
		t.Errorf("expected generated_count 0, got %d", reloaded.GeneratedCount)
	}
	if reloaded.ExistingCount != 5 || reloaded.TargetCount != 5 {
		t.Errorf("expected existing=target=5, got existing=%d target=%d", reloaded.ExistingCount, reloaded.TargetCount)
	}
}

// --- D.5 Partial AI top-up -------------------------------------------------

func TestGenerateToday_PartialAITopsUpWithRuleBased(t *testing.T) {
	// daily_quest_count=5 (from setupServiceTest). AI returns only 2 quests,
	// so the service must top up the remaining 3 with rule-based quests.
	db, userID, service, mockAI, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockAI.quests = []models.Quest{
		{Title: "AI Q1", Source: models.QuestSourceAI, Type: models.QuestTypeMovement, Date: today},
		{Title: "AI Q2", Source: models.QuestSourceAI, Type: models.QuestTypeMovement, Date: today},
	}
	// Rule-based top-up source (more than the shortfall to verify truncation
	// to the effective target).
	mockRule.quests = []models.Quest{
		{Title: "Rule TopUp 1", Source: models.QuestSourceConfigBased, Type: models.QuestTypeLearning, Date: today},
		{Title: "Rule TopUp 2", Source: models.QuestSourceConfigBased, Type: models.QuestTypeLearning, Date: today},
		{Title: "Rule TopUp 3", Source: models.QuestSourceConfigBased, Type: models.QuestTypeLearning, Date: today},
		{Title: "Rule TopUp 4", Source: models.QuestSourceConfigBased, Type: models.QuestTypeLearning, Date: today},
	}

	preferAI := true
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Source != "ai" {
		t.Errorf("expected primary source ai, got %s", result.Source)
	}
	// 2 AI + 3 rule-based top-up (capped at the remaining target of 5).
	if result.GeneratedCount != 5 {
		t.Errorf("expected 5 quests after top-up, got %d", result.GeneratedCount)
	}
	if !result.FallbackUsed {
		t.Error("expected fallback_used true when rule-based top-up was applied")
	}

	var questCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&questCount)
	if int(questCount) != 5 {
		t.Errorf("expected 5 quests in DB, got %d", questCount)
	}
}

// --- E. Status endpoint ----------------------------------------------------

func TestGetJobStatus_NotStarted(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "not_started" {
		t.Errorf("expected not_started, got %s", status.Status)
	}
	if status.JobID != nil {
		t.Errorf("expected nil job_id, got %v", status.JobID)
	}
}

func TestGetJobStatus_CompletedReportsDBQuestCount(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	for i := 0; i < 3; i++ {
		db.Create(&models.Quest{ID: uuid.New(), UserID: userID, Title: "Q", Date: today, Status: models.QuestStatusPending})
	}
	src := models.QuestGenJobSourceAI
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(today),
		Status: models.QuestGenJobStatusCompleted, Source: &src, GeneratedCount: 3,
	}
	db.Create(&job)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "completed" {
		t.Errorf("expected completed, got %s", status.Status)
	}
	if status.QuestCount != 3 {
		t.Errorf("expected quest_count 3, got %d", status.QuestCount)
	}
	if status.Source == nil || *status.Source != "ai" {
		t.Errorf("expected source ai, got %v", status.Source)
	}
}

func TestGetJobStatus_StaleGeneratingJob(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	staleStart := timeutil.NowVN().Add(-16 * time.Minute) // > 900s threshold
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN()),
		Status: models.QuestGenJobStatusGenerating, StartedAt: &staleStart, PreferAI: true,
	}
	db.Create(&job)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "stale" {
		t.Errorf("expected stale, got %s", status.Status)
	}
}

func TestGetJobStatus_LongRunningGeneratingNotStale(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	// Generating for 10 minutes — under the 900s (15 min) stale threshold, so
	// a slow AI run must still report "generating", not an error/stale.
	start := timeutil.NowVN().Add(-10 * time.Minute)
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN()),
		Status: models.QuestGenJobStatusGenerating, StartedAt: &start, PreferAI: true,
	}
	db.Create(&job)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "generating" {
		t.Errorf("expected generating (under stale threshold), got %s", status.Status)
	}
}

// --- A/B. Timeout config + context separation regression -------------------

func TestTimeoutConfig_AIBoundedAndSmallerThanWorker(t *testing.T) {
	if quest_generation.AICallTimeout != 600*time.Second {
		t.Errorf("expected AICallTimeout 600s, got %v", quest_generation.AICallTimeout)
	}
	if quest_generation.WorkerTimeout <= quest_generation.AICallTimeout {
		t.Errorf("worker timeout (%v) must exceed AI timeout (%v)",
			quest_generation.WorkerTimeout, quest_generation.AICallTimeout)
	}
	if quest_generation.StaleJobThreshold != 900*time.Second {
		t.Errorf("expected StaleJobThreshold 900s, got %v", quest_generation.StaleJobThreshold)
	}
	// The stale threshold must outlast a full-length worker run so a slow AI
	// job is never prematurely marked stale.
	if quest_generation.StaleJobThreshold <= quest_generation.WorkerTimeout {
		t.Errorf("stale threshold (%v) must exceed worker timeout (%v)",
			quest_generation.StaleJobThreshold, quest_generation.WorkerTimeout)
	}
}

// aiTimeoutGenerator blocks until its context deadline fires, then returns a
// context-deadline error — simulating a real provider timeout that consumes the
// whole AI deadline.
type aiTimeoutGenerator struct{ called int32 }

func (g *aiTimeoutGenerator) GenerateDailyQuests(ctx context.Context, qctx *quest_generation.UserQuestContext) ([]models.Quest, error) {
	atomic.AddInt32(&g.called, 1)
	<-ctx.Done()
	return nil, errors.New("AI call failed: context deadline exceeded")
}

func TestGenerateToday_AITimeoutDoesNotCancelFallbackDBSave(t *testing.T) {
	db, userID, _, _, mockRule := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	today := timeutil.TodayVN()
	mockRule.quests = []models.Quest{
		{Title: "Fallback Quest", Source: models.QuestSourceConfigBased, Type: models.QuestTypeMovement, Date: today},
	}

	aiGen := &aiTimeoutGenerator{}
	svc := quest_generation.NewGenerationService(
		db, quest_generation.NewUserQuestContextBuilder(db), aiGen, mockRule,
	)
	// Short AI deadline so the test runs fast; the parent context (Background)
	// stays alive, proving the fallback + DB save use a live context.
	svc.SetAICallTimeout(50 * time.Millisecond)

	preferAI := true
	result, err := svc.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if atomic.LoadInt32(&aiGen.called) != 1 {
		t.Errorf("expected AI generator to be called once, got %d", aiGen.called)
	}
	if !result.FallbackUsed {
		t.Error("expected fallback_used true")
	}
	if result.Source != "rule_based" {
		t.Errorf("expected source rule_based, got %s", result.Source)
	}
	if result.AIErrorType != "timeout" {
		t.Errorf("expected ai_error_type timeout, got %s", result.AIErrorType)
	}
	if result.GeneratedCount < 1 {
		t.Errorf("expected fallback quest inserted, got %d", result.GeneratedCount)
	}

	var dbCount int64
	db.Model(&models.Quest{}).Where("user_id = ?", userID).Count(&dbCount)
	if dbCount < 1 {
		t.Errorf("expected fallback quest persisted to DB, got %d", dbCount)
	}
}

func TestGetJobStatus_FailedReportsError(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	msg := "provider error"
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN()),
		Status: models.QuestGenJobStatusFailed, ErrorMessage: &msg, FallbackUsed: true,
	}
	db.Create(&job)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "failed" {
		t.Errorf("expected failed, got %s", status.Status)
	}
	if status.ErrorMessage == nil || *status.ErrorMessage != "provider error" {
		t.Errorf("expected error message, got %v", status.ErrorMessage)
	}
}

func TestGetJobStatus_ScopedByAuthenticatedUserAndDate(t *testing.T) {
	db, userID, service, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	otherID := uuid.New()
	testutils.CreateTestUser(db, otherID, "other@example.com")

	today := timeutil.StartOfDayVN(timeutil.TodayVN())
	finished := timeutil.NowVN()
	srcAI := models.QuestGenJobSourceAI
	srcRule := models.QuestGenJobSourceRuleBased
	msg := "other user failed"
	otherJob := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: otherID, Date: today,
		Status: models.QuestGenJobStatusFailed, Source: &srcRule,
		ErrorMessage: &msg, CompletedAt: &finished, TargetCount: 5,
	}
	userJob := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: today,
		Status: models.QuestGenJobStatusCompleted, Source: &srcAI,
		CompletedAt: &finished, TargetCount: 5, ExistingCount: 2, GeneratedCount: 2,
	}
	db.Create(&otherJob)
	db.Create(&userJob)

	status, err := service.GetJobStatus(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.JobID == nil || *status.JobID != userJob.ID.String() {
		t.Fatalf("expected user job %s, got %v", userJob.ID, status.JobID)
	}
	if status.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed for authenticated user, got %s", status.Status)
	}
	if status.ErrorMessage != nil {
		t.Fatalf("status leaked other user's error: %v", *status.ErrorMessage)
	}

	otherStatus, err := service.GetJobStatus(context.Background(), otherID, nil)
	if err != nil {
		t.Fatalf("unexpected other status error: %v", err)
	}
	if otherStatus.JobID == nil || *otherStatus.JobID != otherJob.ID.String() {
		t.Fatalf("expected other job %s, got %v", otherJob.ID, otherStatus.JobID)
	}
	if otherStatus.Status != models.QuestGenJobStatusFailed {
		t.Fatalf("expected failed for other user, got %s", otherStatus.Status)
	}
}

func TestGenerateToday_AIDisabledTypesFallbackToEnabledRules(t *testing.T) {
	db, userID, _, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	catsJSON, _ := json.Marshal([]string{"movement"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "easy", ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_learning", Type: "learning", Enabled: false, Difficulty: "medium", ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
	})
	if err := db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"daily_quest_count":  1,
		"enabled_categories": datatypes.JSON(catsJSON),
		"rules":              datatypes.JSON(rulesJSON),
	}).Error; err != nil {
		t.Fatal(err)
	}

	aiResp := `{"quests":[{"type":"learning","title":"Học sai loại","description":"Learning is disabled","difficulty":"normal","estimated_minutes":20,"xp_reward":10,"tags":["learning"],"reason":"x","instruction":"y","reminder_time":"14:00"}]}`
	aiGen := quest_generation.NewAIGenerator(&ai.MockClient{ResponseText: aiResp, Model: "m", FinishReason: "stop"})
	service := quest_generation.NewGenerationService(
		db,
		quest_generation.NewUserQuestContextBuilder(db),
		aiGen,
		quest_generation.NewRuleBasedGenerator(db),
	)
	preferAI := true
	date := timeutil.FormatDateVN(timeutil.TodayVN().AddDate(0, 0, 1))
	result, err := service.GenerateToday(context.Background(), userID, quest_generation.GenerateTodayRequest{
		Date: &date, PreferAI: &preferAI,
	})
	if err != nil {
		t.Fatalf("expected fallback to enabled movement rule, got: %v", err)
	}
	if !result.FallbackUsed {
		t.Fatal("expected fallback_used true")
	}
	if result.GeneratedCount == 0 {
		t.Fatal("expected fallback to generate at least one quest")
	}
	for _, q := range result.Quests {
		if q.Type == models.QuestTypeLearning {
			t.Fatalf("disabled learning quest leaked into result: %+v", q)
		}
	}
}

func TestProcessJob_DisabledAITypeAndNoFallbackRulesStillCompletes(t *testing.T) {
	// AI returns a DISABLED type (learning, dropped → zero valid). The user still
	// has an enabled quest category (review), so the deterministic fallback fills
	// the day from that ENABLED category and the job completes. The disabled type
	// must not leak in.
	db, userID, _, _, _ := setupServiceTest(t)
	defer testutils.CleanupTestDB(t, db)

	catsJSON, _ := json.Marshal([]string{"review"})
	rulesJSON, _ := json.Marshal([]dto.QuestRuleResponse{
		{ID: "rule_review", Type: "review", Enabled: true, Difficulty: "easy", ActiveTimeRange: &dto.TimeRangeResponse{Start: "21:00", End: "23:00"}, ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
		{ID: "rule_learning", Type: "learning", Enabled: false, Difficulty: "easy", ActiveWeekdays: []int{1, 2, 3, 4, 5, 6, 7}},
	})
	if err := db.Model(&models.QuestSettings{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"daily_quest_count":  1,
		"enabled_categories": datatypes.JSON(catsJSON),
		"rules":              datatypes.JSON(rulesJSON),
	}).Error; err != nil {
		t.Fatal(err)
	}

	aiResp := `{"quests":[{"type":"learning","title":"Học sai loại","description":"Learning is disabled","difficulty":"normal","estimated_minutes":20,"xp_reward":10,"tags":["learning"],"reason":"x","instruction":"y","reminder_time":"14:00"}]}`
	aiGen := quest_generation.NewAIGenerator(&ai.MockClient{ResponseText: aiResp, Model: "m", FinishReason: "stop"})
	service := quest_generation.NewGenerationService(
		db,
		quest_generation.NewUserQuestContextBuilder(db),
		aiGen,
		quest_generation.NewRuleBasedGenerator(db),
	)
	job := models.DailyQuestGenerationJob{
		ID: uuid.New(), UserID: userID, Date: timeutil.StartOfDayVN(timeutil.TodayVN().AddDate(0, 0, 1)),
		Status: models.QuestGenJobStatusPending, PreferAI: true,
	}
	db.Create(&job)

	service.ProcessJob(context.Background(), job.ID)

	var reloaded models.DailyQuestGenerationJob
	db.First(&reloaded, "id = ?", job.ID)
	if reloaded.Status != models.QuestGenJobStatusCompleted {
		t.Fatalf("expected completed via smart fallback, got %s (err=%v)", reloaded.Status, reloaded.ErrorMessage)
	}
	if reloaded.CompletedAt == nil {
		t.Fatal("expected finished_at/completed_at")
	}
	if !reloaded.FallbackUsed {
		t.Fatal("expected fallback_used true after AI validation failure")
	}
	if reloaded.GeneratedCount != 1 {
		t.Errorf("expected generated_count 1 (target), got %d", reloaded.GeneratedCount)
	}

	// The disabled learning type must not leak into the saved quests.
	var dbQuests []models.Quest
	db.Where("user_id = ?", userID).Find(&dbQuests)
	for _, q := range dbQuests {
		if q.Type == models.QuestTypeLearning {
			t.Fatalf("disabled learning quest leaked into result: %+v", q)
		}
	}
}
