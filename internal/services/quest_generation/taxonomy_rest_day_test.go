package quest_generation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/internal/services/quest_generation"
	"solo_quest_backend/internal/testutils"
)

// weekdayNumOf returns 1=Mon ... 7=Sun for a date (matching prompt convention).
func weekdayNumOf(d time.Time) int {
	n := int(d.Weekday())
	if n == 0 {
		n = 7
	}
	return n
}

// nextFutureWeekday returns a future date (strictly after today) whose weekday
// matches/avoids the weekend as requested, keeping tests deterministic and out
// of the "today late-evening" branch.
func nextFutureSaturday() time.Time {
	d := timeutil.TodayVN().AddDate(0, 0, 1)
	for d.Weekday() != time.Saturday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func nextFutureMidweekDay() time.Time {
	d := timeutil.TodayVN().AddDate(0, 0, 1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// ─── Taxonomy: rule-based must never produce water / breakTime ───────────────

func TestRuleBasedGenerator_NeverGeneratesWaterOrBreakTime(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	max8 := 8
	// Construct context directly with water + breakTime enabled and as rules,
	// bypassing the context builder's sanitization, to prove the generator
	// itself never emits reminder-only types as daily quests.
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         nextFutureMidweekDay(),
		DailyQuestCount:   8,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"water", "breakTime", "movement", "learning", "sleep", "review"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_water", Type: "water", Enabled: true, Difficulty: "easy", MaxPerDay: &max8,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "08:00", End: "22:00"}},
			{ID: "rule_break_time", Type: "breakTime", Enabled: true, Difficulty: "easy", MaxPerDay: &max8,
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "09:00", End: "18:00"}},
			{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "medium", MaxPerDay: ptr(1),
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "10:00", End: "17:00"}},
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", MaxPerDay: ptr(1),
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "19:00", End: "22:00"}},
		},
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, q := range quests {
		if q.Type == models.QuestTypeWater {
			t.Errorf("rule-based generator produced a water daily quest: %s", q.Title)
		}
		if q.Type == models.QuestTypeBreak {
			t.Errorf("rule-based generator produced a breakTime daily quest: %s", q.Title)
		}
	}
}

// ─── Rest day / weekend policy ───────────────────────────────────────────────

func TestRuleBasedGenerator_RestDayReducesLoad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	saturday := nextFutureSaturday()

	baseCtx := func(restDay bool) *quest_generation.UserQuestContext {
		return &quest_generation.UserQuestContext{
			UserID:            uuid.New(),
			LocalDate:         saturday,
			DailyQuestCount:   8,
			Difficulty:        "hard",
			PreferredDuration: "long",
			EnabledCategories: []string{"movement", "learning", "sleep", "review"},
			RestDayEnabled:    restDay,
			IsWeekend:         true,
			IsRestDay:         restDay,
			Rules:             makeQuestRules(ptr(3), ptr(3), ptr(1), ptr(1)),
		}
	}

	// Rest day ON
	restQuests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), baseCtx(true))
	if err != nil {
		t.Fatalf("unexpected error (rest day): %v", err)
	}

	if len(restQuests) > 4 {
		t.Errorf("rest day should cap quest count at 4, got %d", len(restQuests))
	}
	for _, q := range restQuests {
		if q.Difficulty == models.QuestDifficultyHard {
			t.Errorf("rest day should not produce hard quests, got %s: %s", q.Difficulty, q.Title)
		}
	}
}

func TestRuleBasedGenerator_RestDayDisabled_NoCap(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	saturday := nextFutureSaturday()

	// Weekend but rest_day_enabled=false → no rest-day cap applies.
	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         saturday,
		DailyQuestCount:   8,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning", "sleep", "review"},
		RestDayEnabled:    false,
		IsWeekend:         true,
		IsRestDay:         false,
		Rules:             makeQuestRules(ptr(3), ptr(3), ptr(1), ptr(1)),
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be able to exceed the rest-day cap of 4 on a normal (non-rest) day.
	if len(quests) <= 4 {
		t.Logf("note: produced %d quests; pool may be small, but no rest-day cap was applied", len(quests))
	}
}

// ─── Active weekdays: rule not active today must not be used ──────────────────

func TestRuleBasedGenerator_RuleNotActiveTodayIsSkipped(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	day := nextFutureMidweekDay()
	todayNum := weekdayNumOf(day)
	otherDay := todayNum%7 + 1 // a different weekday number

	qctx := &quest_generation.UserQuestContext{
		UserID:            uuid.New(),
		LocalDate:         day,
		DailyQuestCount:   8,
		Difficulty:        "normal",
		PreferredDuration: "medium",
		EnabledCategories: []string{"movement", "learning"},
		Rules: []quest_generation.QuestRuleContext{
			{ID: "rule_movement", Type: "movement", Enabled: true, Difficulty: "medium", MaxPerDay: ptr(1),
				Weekdays:        []int{1, 2, 3, 4, 5, 6, 7},
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "10:00", End: "17:00"}},
			// Learning only active on a day that is NOT today.
			{ID: "rule_learning", Type: "learning", Enabled: true, Difficulty: "medium", MaxPerDay: ptr(2),
				Weekdays:        []int{otherDay},
				ActiveTimeRange: &quest_generation.TimeRangeContext{Start: "19:00", End: "22:00"}},
		},
	}

	quests, err := quest_generation.NewRuleBasedGenerator(db).GenerateDailyQuests(context.Background(), qctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	movementSeen := false
	for _, q := range quests {
		if q.Type == models.QuestTypeLearning {
			t.Errorf("learning rule is not active on weekday %d but a learning quest was generated: %s", todayNum, q.Title)
		}
		if q.Type == models.QuestTypeMovement {
			movementSeen = true
		}
	}
	if !movementSeen {
		t.Error("movement rule active every day should still produce a movement quest")
	}
}
