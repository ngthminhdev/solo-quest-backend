package quest_generation

import "strings"

// QuestSlot describes one planned quest slot in a QuestCompositionPlan. Slots are
// the explicit, ordered composition the AI is asked to follow and the smart
// fallback uses to fill missing quests. Source is a human/AI-facing hint
// (learning_roadmap, health_goal, daily_review, sleep_goal, screen_rest,
// fallback). MaxCount mirrors the per-type cap that applies to the slot's type.
type QuestSlot struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Required bool   `json:"required,omitempty"`
	MaxCount int    `json:"max_count,omitempty"`
	Style    string `json:"style,omitempty"`
}

type QuestCompositionPlan struct {
	TargetCount            int
	ExistingCount          int
	NeededCount            int
	AllowedTypes           []string
	RequireLearningRoadmap bool
	PreferredLearningCount int
	MaxBreakTimeCount      int
	EasyAndShort           bool
	GentleMovement         bool

	// Caps is the per-type aggregate maximum for a single day's generation.
	// water is always 0 (never generated). Keys are normalized quest types
	// (movement, learning, sleep, review, breakTime, water).
	Caps map[string]int
	// Slots is the explicit, ordered composition. len(Slots) == NeededCount.
	Slots []QuestSlot
}

func BuildQuestCompositionPlan(qctx *UserQuestContext) QuestCompositionPlan {
	if qctx == nil {
		return QuestCompositionPlan{Caps: defaultCaps(1, 1, 1)}
	}

	targetCount := qctx.TargetQuestCount
	if targetCount <= 0 {
		targetCount = qctx.DailyQuestCount
	}
	existingCount := qctx.ExistingQuestCount
	neededCount := qctx.MissingQuestCount
	if neededCount <= 0 {
		neededCount = targetCount - existingCount
	}
	if neededCount < 0 {
		neededCount = 0
	}
	if qctx.PreviewLimit > 0 && qctx.PreviewLimit < neededCount {
		neededCount = qctx.PreviewLimit
	}

	plan := QuestCompositionPlan{
		TargetCount:       targetCount,
		ExistingCount:     existingCount,
		NeededCount:       neededCount,
		AllowedTypes:      AllowedAIQuestTypesForContext(qctx),
		MaxBreakTimeCount: 1,
		GentleMovement:    isMovementGentleRequired(qctx),
	}
	if neededCount >= 8 {
		plan.MaxBreakTimeCount = 2
	}

	// Movement cap: 1 by default (especially for low-activity/back-pain users).
	// A rule may raise it, but gentle-movement users stay capped at 1.
	movementCap := 1
	for _, rule := range qctx.Rules {
		if NormalizeType(rule.Type) == "movement" && rule.MaxPerDay != nil && *rule.MaxPerDay > 0 {
			movementCap = *rule.MaxPerDay
		}
	}
	if plan.GentleMovement && movementCap > 1 {
		movementCap = 1
	}

	if qctx.ActiveLearningPath != nil && qctx.ActiveLearningPath.StepID != "" && containsType(plan.AllowedTypes, "learning") {
		plan.RequireLearningRoadmap = true
		plan.PreferredLearningCount = 1
	}
	if qctx.TodayCheckIn != nil {
		if qctx.TodayCheckIn.Priority == "learning" && plan.RequireLearningRoadmap && neededCount >= 4 {
			plan.PreferredLearningCount = 2
		}
		energy := strings.ToLower(qctx.TodayCheckIn.EnergyLevel)
		availability := strings.ToLower(qctx.TodayCheckIn.Availability)
		if energy == "low" || energy == "very_low" || availability == "low" || availability == "busy" || availability == "very_busy" {
			plan.EasyAndShort = true
		}
	}

	learningCap := 1
	if plan.RequireLearningRoadmap {
		// An active roadmap is the user's main learning focus and is the scalable
		// enabled category for filling the day: a roadmap step supports several
		// complementary quests (study / review / practice / summarize). Allow
		// learning to fill the remaining slots (bounded by the available learning
		// templates) so the daily target can be met from ENABLED categories
		// instead of padding with non-enabled / reminder-only ones. Always at
		// least 2 (study + review).
		learningCap = neededCount
		if learningCap < 2 {
			learningCap = 2
		}
	}
	plan.Caps = defaultCaps(movementCap, plan.MaxBreakTimeCount, learningCap)

	plan.Slots = buildCompositionSlots(plan)
	return plan
}

// defaultCaps builds the per-type cap map. water is always 0.
func defaultCaps(movementCap, breakTimeCap, learningCap int) map[string]int {
	return map[string]int{
		"movement":  movementCap,
		"breakTime": breakTimeCap,
		"learning":  learningCap,
		"sleep":     1,
		"review":    1,
		"water":     0,
	}
}

// buildCompositionSlots produces exactly NeededCount slots, diversifying the day
// while respecting per-type caps. Required learning-roadmap slots come first.
func buildCompositionSlots(plan QuestCompositionPlan) []QuestSlot {
	if plan.NeededCount <= 0 {
		return nil
	}

	allowed := make(map[string]bool, len(plan.AllowedTypes))
	for _, t := range plan.AllowedTypes {
		allowed[NormalizeType(t)] = true
	}

	used := make(map[string]int)
	slots := make([]QuestSlot, 0, plan.NeededCount)

	style := func(typ string) string {
		if typ == "movement" && plan.GentleMovement {
			return "gentle"
		}
		return ""
	}
	cap := func(typ string) int {
		if c, ok := plan.Caps[typ]; ok {
			return c
		}
		return 1
	}
	add := func(typ, source string, required bool) bool {
		if len(slots) >= plan.NeededCount {
			return false
		}
		if c := cap(typ); c >= 0 && used[typ] >= c {
			return false
		}
		slots = append(slots, QuestSlot{Type: typ, Source: source, Required: required, MaxCount: cap(typ), Style: style(typ)})
		used[typ]++
		return true
	}

	// Required learning-roadmap slots first.
	if plan.RequireLearningRoadmap {
		want := plan.PreferredLearningCount
		if want < 1 {
			want = 1
		}
		for i := 0; i < want; i++ {
			add("learning", "learning_roadmap", i == 0)
		}
	}

	// One pass over the diversity preference, then cycle to fill remaining slots.
	pref := []struct{ typ, source string }{
		{"movement", "health_goal"},
		{"review", "daily_review"},
		{"sleep", "sleep_goal"},
		{"breakTime", "screen_rest"},
		{"learning", "learning_roadmap"},
	}
	for _, p := range pref {
		if allowed[p.typ] {
			add(p.typ, p.source, false)
		}
	}
	for len(slots) < plan.NeededCount {
		progressed := false
		for _, p := range pref {
			if !allowed[p.typ] {
				continue
			}
			if add(p.typ, p.source, false) {
				progressed = true
			}
			if len(slots) >= plan.NeededCount {
				break
			}
		}
		if !progressed {
			break
		}
	}

	// Any still-unfilled slots become generic fallback slots (review/learning),
	// so the plan always has exactly NeededCount slots.
	for len(slots) < plan.NeededCount {
		if plan.RequireLearningRoadmap {
			slots = append(slots, QuestSlot{Type: "learning", Source: "learning_roadmap", MaxCount: cap("learning")})
		} else {
			slots = append(slots, QuestSlot{Type: "review", Source: "fallback", MaxCount: cap("review")})
		}
	}

	return slots
}

func containsType(types []string, target string) bool {
	for _, typ := range types {
		if NormalizeType(typ) == target {
			return true
		}
	}
	return false
}
