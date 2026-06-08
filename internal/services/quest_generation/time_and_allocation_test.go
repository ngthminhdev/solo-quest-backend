package quest_generation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
)

func TestSleepCrossingMidnight(t *testing.T) {
	questDate, _ := time.ParseInLocation("2006-01-02", "2026-06-07", timeutil.LocationVN)

	tests := []struct {
		targetSleep      string
		expectedActual   string
		expectedReminder string
	}{
		{"00:00", "2026-06-08T00:00:00", "2026-06-07T23:30:00"},
		{"01:00", "2026-06-08T01:00:00", "2026-06-08T00:30:00"},
		{"23:00", "2026-06-07T23:00:00", "2026-06-07T22:30:00"},
	}

	for _, tc := range tests {
		actual, reminder, err := quest_generation.CalculateSleepTimes(questDate, tc.targetSleep)
		if err != nil {
			t.Fatalf("unexpected error for sleep %s: %v", tc.targetSleep, err)
		}

		actualStr := actual.Format("2006-01-02T15:04:05")
		reminderStr := reminder.Format("2006-01-02T15:04:05")

		if actualStr != tc.expectedActual {
			t.Errorf("for sleep %s: expected actual %s, got %s", tc.targetSleep, tc.expectedActual, actualStr)
		}
		if reminderStr != tc.expectedReminder {
			t.Errorf("for sleep %s: expected reminder %s, got %s", tc.targetSleep, tc.expectedReminder, reminderStr)
		}
	}
}

func TestPastReminderHandlingAndValidator(t *testing.T) {
	questDate, _ := time.ParseInLocation("2006-01-02", "2026-06-07", timeutil.LocationVN)

	mockNow, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-07 20:57:00", timeutil.LocationVN)

	validatorWithNow := quest_generation.NewCandidateValidatorWithNow(mockNow)

	qctxToday := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate,
		DailyQuestCount:   5,
		EnabledCategories: []string{"learning", "sleep", "review"},
		Rules: []quest_generation.QuestRuleContext{
			{Type: "learning", Enabled: true},
			{Type: "sleep", Enabled: true},
			{Type: "review", Enabled: true},
		},
	}

	// 1. today candidate with reminder_time 18:00 (past) -> fails validator
	candidatesPast := []quest_generation.QuestCandidate{
		{
			Type:             "learning",
			Title:            "Học tập 20 phút",
			Description:      "Học bài",
			Difficulty:       "normal",
			EstimatedMinutes: 20,
			XPReward:         10,
			ReminderTime:     "18:00",
			Tags:             []string{"learning"},
		},
	}
	err := validatorWithNow.Validate(qctxToday, candidatesPast)
	if err == nil {
		t.Error("expected validation failure for today candidate with reminder_time 18:00 (in the past)")
	}

	// 2. NormalizeCandidates should shift/drop today candidate with past reminder
	normalizedToday := quest_generation.NormalizeCandidates(qctxToday, candidatesPast, mockNow)
	if len(normalizedToday) == 0 {
		t.Error("expected candidate to be shifted, not dropped")
	} else {
		if normalizedToday[0].ReminderTime != "21:15" {
			t.Errorf("expected normalized reminder_time '21:15', got '%s'", normalizedToday[0].ReminderTime)
		}
	}

	// 3. today candidate with review + learning (future) -> should pass validator
	candidatesWithReview := []quest_generation.QuestCandidate{
		{
			Type:             "review",
			Title:            "Tổng kết ngày",
			Description:      "Nhìn lại hôm nay",
			Difficulty:       "easy",
			EstimatedMinutes: 5,
			XPReward:         5,
			ReminderTime:     "21:50",
			Tags:             []string{"review"},
		},
	}
	err = validatorWithNow.Validate(qctxToday, candidatesWithReview)
	if err != nil {
		t.Errorf("unexpected validator error for future review reminder: %v", err)
	}

	// 4. future date (tomorrow) with reminder_time 18:00 should be allowed by validator
	qctxTomorrow := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate.AddDate(0, 0, 1),
		DailyQuestCount:   5,
		EnabledCategories: []string{"learning"},
		Rules: []quest_generation.QuestRuleContext{
			{Type: "learning", Enabled: true},
		},
	}
	err = validatorWithNow.Validate(qctxTomorrow, candidatesPast)
	if err != nil {
		t.Errorf("unexpected validation failure for tomorrow candidate: %v", err)
	}
}

func TestLearningCap(t *testing.T) {
	mockNow, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-08 08:00:00", timeutil.LocationVN)
	validator := quest_generation.NewCandidateValidatorWithNow(mockNow)
	questDate, _ := time.ParseInLocation("2006-01-02", "2026-06-08", timeutil.LocationVN)

	qctxNoPath := &quest_generation.UserQuestContext{
		UserID:             uuid.New(),
		LocalDate:          questDate,
		DailyQuestCount:    5,
		EnabledCategories:  []string{"learning"},
		ActiveLearningPath: nil,
		Rules: []quest_generation.QuestRuleContext{
			{Type: "learning", Enabled: true},
		},
	}

	// no Active Learning Path + 4 learning candidates -> validation fails
	candidatesFour := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Học 1", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "10:00", Tags: []string{"learning"}},
		{Type: "learning", Title: "Học 2", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "11:00", Tags: []string{"learning"}},
		{Type: "learning", Title: "Học 3", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "12:00", Tags: []string{"learning"}},
		{Type: "learning", Title: "Học 4", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "13:00", Tags: []string{"learning"}},
	}
	err := validator.Validate(qctxNoPath, candidatesFour)
	if err == nil {
		t.Error("expected validation failure for 4 learning candidates without active path")
	}

	// no Active Learning Path + 1 generic learning candidate -> validation passes
	candidatesOne := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Học tập 20 phút", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "10:00", Tags: []string{"learning"}},
	}
	err = validator.Validate(qctxNoPath, candidatesOne)
	if err != nil {
		t.Errorf("unexpected validation error for 1 learning candidate: %v", err)
	}

	// Active Learning Path exists + multiple learning candidates -> validation passes
	qctxWithPath := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate,
		DailyQuestCount:   5,
		EnabledCategories: []string{"learning"},
		ActiveLearningPath: &quest_generation.ActiveLearningPathDetail{
			RoadmapTitle:     "Go Roadmap",
			CurrentStepTitle: "Goroutines",
			Description:      "Learn concurrency",
		},
		Rules: []quest_generation.QuestRuleContext{
			{Type: "learning", Enabled: true, MaxPerDay: intPtr(5)},
		},
	}
	candidatesMultipleWithPath := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Goroutines", Description: "Học Goroutines", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "10:00", Tags: []string{"learning"}},
		{Type: "learning", Title: "Channels", Description: "Học Channels", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "11:00", Tags: []string{"learning"}},
	}
	err = validator.Validate(qctxWithPath, candidatesMultipleWithPath)
	if err != nil {
		t.Errorf("unexpected validation error with active roadmap: %v", err)
	}
}

func TestRuleBasedAliasAndDailyCount(t *testing.T) {
	// Simple validation tests for review rules and daily count
	mockNow, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-08 08:00:00", timeutil.LocationVN)
	validator := quest_generation.NewCandidateValidatorWithNow(mockNow)
	questDate, _ := time.ParseInLocation("2006-01-02", "2026-06-08", timeutil.LocationVN)

	qctxReviewAlias := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate,
		DailyQuestCount:   2,
		EnabledCategories: []string{"daily_review"}, // alias enabled
		Rules: []quest_generation.QuestRuleContext{
			{Type: "daily_review", Enabled: true}, // alias
		},
	}

	// Validator should pass review quest type when daily_review alias is enabled in categories/rules
	reviewCandidate := []quest_generation.QuestCandidate{
		{Type: "review", Title: "Daily review", Description: "Nhìn lại ngày hôm nay", Difficulty: "easy", EstimatedMinutes: 10, XPReward: 5, ReminderTime: "21:30", Tags: []string{"review"}},
	}

	err := validator.Validate(qctxReviewAlias, reviewCandidate)
	if err != nil {
		t.Errorf("unexpected validation error for review alias: %v", err)
	}

	// Review-missing detection: review enabled but no review quest generated
	qctxReviewEnabledNoReview := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate,
		DailyQuestCount:   2,
		EnabledCategories: []string{"review", "learning"},
		Rules: []quest_generation.QuestRuleContext{
			{Type: "review", Enabled: true},
			{Type: "learning", Enabled: true},
		},
	}
	candidatesNoReview := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Học tập 20 phút", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "10:00", Tags: []string{"learning"}},
	}
	err = validator.Validate(qctxReviewEnabledNoReview, candidatesNoReview)
	if err == nil {
		t.Error("expected validation error: review is enabled but no review quest was generated")
	} else {
		t.Logf("Got expected validation error: %v", err)
	}

	// Review-missing using daily_review alias
	qctxDailyReviewEnabledNoReview := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         questDate,
		DailyQuestCount:   2,
		EnabledCategories: []string{"daily_review", "learning"},
		Rules: []quest_generation.QuestRuleContext{
			{Type: "review", Enabled: true},
			{Type: "learning", Enabled: true},
		},
	}
	err = validator.Validate(qctxDailyReviewEnabledNoReview, candidatesNoReview)
	if err == nil {
		t.Error("expected validation error: daily_review is enabled but no review quest was generated")
	} else {
		t.Logf("Got expected validation error: %v", err)
	}

	// Review enabled with review present should pass
	candidatesWithReview := []quest_generation.QuestCandidate{
		{Type: "learning", Title: "Học tập 20 phút", Description: "Học bài", Difficulty: "normal", EstimatedMinutes: 20, XPReward: 10, ReminderTime: "10:00", Tags: []string{"learning"}},
		{Type: "review", Title: "Tổng kết ngày", Description: "Nhìn lại hôm nay", Difficulty: "easy", EstimatedMinutes: 5, XPReward: 5, ReminderTime: "21:30", Tags: []string{"review"}},
	}
	err = validator.Validate(qctxReviewEnabledNoReview, candidatesWithReview)
	if err != nil {
		t.Errorf("unexpected validation error with review present: %v", err)
	}
}

func TestFilterEnabledRulesNormalization(t *testing.T) {
	rules := []quest_generation.QuestRuleContext{
		{Type: "review", Enabled: true, ID: "rule_review"},
		{Type: "water", Enabled: true, ID: "rule_water"},
		{Type: "learning", Enabled: true, ID: "rule_learning"},
		{Type: "breakTime", Enabled: true, ID: "rule_break_time"},
	}

	// daily_review alias should match "review" rule type
	result := quest_generation.FilterEnabledRules(rules, []string{"daily_review"})
	foundReview := false
	for _, r := range result {
		if r.Type == "review" {
			foundReview = true
		}
	}
	if !foundReview {
		t.Error("filterEnabledRules should match review rule when enabled category is daily_review")
	}

	// review category should match review rule
	result = quest_generation.FilterEnabledRules(rules, []string{"review"})
	foundReview = false
	for _, r := range result {
		if r.Type == "review" {
			foundReview = true
		}
	}
	if !foundReview {
		t.Error("filterEnabledRules should match review rule when enabled category is review")
	}

	// water should never match review
	result = quest_generation.FilterEnabledRules(rules, []string{"water"})
	for _, r := range result {
		if r.Type == "review" {
			t.Error("filterEnabledRules should not match review when water is enabled")
		}
	}
	if len(result) == 0 {
		t.Error("filterEnabledRules should match water rule when water is enabled")
	}

	// break_time alias should match breakTime
	result = quest_generation.FilterEnabledRules(rules, []string{"break_time"})
	foundBreak := false
	for _, r := range result {
		if r.Type == "breakTime" {
			foundBreak = true
		}
	}
	if !foundBreak {
		t.Error("filterEnabledRules should match breakTime rule when enabled category is break_time")
	}
}

func TestIsPastReminderForToday(t *testing.T) {
	questDate, _ := time.ParseInLocation("2006-01-02", "2026-06-07", timeutil.LocationVN)
	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-07 20:57:00", timeutil.LocationVN)

	tests := []struct {
		name         string
		questDate    time.Time
		reminderTime string
		now          time.Time
		expected     bool
	}{
		{"today past 18:00", questDate, "18:00", now, true},
		{"today past 20:00", questDate, "20:00", now, true},
		{"today future 21:30", questDate, "21:30", now, false},
		{"tomorrow 18:00", questDate.AddDate(0, 0, 1), "18:00", now, false},
		{"sleep 00:00 overnight to next day", questDate, "00:00", now, false}, // overnight 00:00 becomes next day 00:00 which is after 20:57
		{"sleep 23:30 before midnight", questDate, "23:30", now, false},
	}

	for _, tc := range tests {
		result := quest_generation.IsPastReminderForToday(tc.questDate, tc.reminderTime, tc.now)
		if result != tc.expected {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, result)
		}
	}
}

func TestNextSafeTimeSlot(t *testing.T) {
	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-07 20:57:00", timeutil.LocationVN)

	slot := quest_generation.NextSafeTimeSlot(now)
	expected := "21:15" // 20:57 + 15 = 21:12, rounded to next 5-min = 21:15
	actual := slot.Format("15:04")
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}

	// Test with time already on 5-minute boundary
	now2, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-06-07 10:00:00", timeutil.LocationVN)
	slot2 := quest_generation.NextSafeTimeSlot(now2)
	expected2 := "10:15"
	actual2 := slot2.Format("15:04")
	if actual2 != expected2 {
		t.Errorf("expected %s, got %s", expected2, actual2)
	}
}

func TestHasReviewEnabled(t *testing.T) {
	if !quest_generation.HasReviewEnabled([]string{"review", "learning"}) {
		t.Error("expected HasReviewEnabled true for review")
	}
	if !quest_generation.HasReviewEnabled([]string{"daily_review", "learning"}) {
		t.Error("expected HasReviewEnabled true for daily_review alias")
	}
	if quest_generation.HasReviewEnabled([]string{"learning", "sleep"}) {
		t.Error("expected HasReviewEnabled false when review not in list")
	}
	if quest_generation.HasReviewEnabled([]string{}) {
		t.Error("expected HasReviewEnabled false for empty list")
	}
}

func TestHasCategoryEnabled(t *testing.T) {
	if !quest_generation.HasCategoryEnabled([]string{"daily_review", "learning"}, "review") {
		t.Error("expected HasCategoryEnabled true for review when daily_review is enabled")
	}
	if !quest_generation.HasCategoryEnabled([]string{"review", "learning"}, "daily_review") {
		t.Error("expected HasCategoryEnabled true for daily_review when review is enabled")
	}
	if !quest_generation.HasCategoryEnabled([]string{"break_time"}, "breakTime") {
		t.Error("expected HasCategoryEnabled true for breakTime when break_time is enabled")
	}
	if quest_generation.HasCategoryEnabled([]string{"learning"}, "review") {
		t.Error("expected HasCategoryEnabled false for review when only learning is enabled")
	}
}

func TestFilterEnabledRulesExport(t *testing.T) {
	// Export test so the external test package can use FilterEnabledRules
	rules := []quest_generation.QuestRuleContext{
		{Type: "movement", Enabled: true, ID: "rule_movement"},
	}
	result := quest_generation.FilterEnabledRules(rules, []string{"movement"})
	if len(result) != 1 {
		t.Errorf("expected 1 rule, got %d", len(result))
	}
}

func intPtr(val int) *int {
	return &val
}
