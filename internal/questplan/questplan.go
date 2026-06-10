// Package questplan: semantic dedup cho SoloQuest daily quest generation.
//
// Dedup hai quest trùng nghĩa (vd 2 learning generic từ AI + ruleBased) bằng
// canonical key: learning generic → "learning:generic", learning gắn roadmap →
// "learning:step:<id>", còn lại → "<type>:<title-normalized>".
package questplan

import (
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type QuestType string

const (
	TypeMovement QuestType = "movement"
	TypeLearning QuestType = "learning"
	TypeSleep    QuestType = "sleep"
	TypeReview   QuestType = "review"
)

type Source string

const (
	SourceAI        Source = "ai"
	SourceRuleBased Source = "ruleBased"
)

// Candidate represents a quest before it is committed to the day plan.
type Candidate struct {
	Title         string
	Description   string
	Type          QuestType
	Difficulty    string
	EstimatedMin  int
	Reason        string
	Instruction   string
	Tags          []string
	Source        Source
	ReminderTime  time.Time
	RoadmapStepID string
}

// ---------------------------------------------------------------------------
// Role
// ---------------------------------------------------------------------------

type Role string

const (
	RoleStartDay     Role = "start_day"
	RoleMiddayReset  Role = "midday_reset"
	RoleAfterWork    Role = "after_work"
	RoleLearning     Role = "learning"
	RoleEndReview    Role = "end_review"
	RoleSleepPrepare Role = "sleep_prepare"
)

func roleCategory(r Role) QuestType {
	switch r {
	case RoleStartDay, RoleEndReview:
		return TypeReview
	case RoleMiddayReset, RoleAfterWork:
		return TypeMovement
	case RoleLearning:
		return TypeLearning
	case RoleSleepPrepare:
		return TypeSleep
	}
	return TypeLearning
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

type DayType int

const (
	DayNormal DayType = iota
	DayBusy
	DayLowEnergy
	DayRest
)

type Context struct {
	Now               time.Time
	Loc               *time.Location
	DailyQuestCount   int
	EnabledCategories map[QuestType]bool
	DayType           DayType
	HasRoadmap        bool
	WorkEnd           time.Time
	SleepTarget       time.Time
	MinIntervalMin    int
}

// ---------------------------------------------------------------------------
// Semantic dedup (exported)
// ---------------------------------------------------------------------------

var genericLearningMarkers = []string{
	"học tập",
	"học 20",
	"học khoảng 20",
	"chọn một chủ đề",
	"chọn chủ đề",
	"đọc tài liệu",
	"tìm hiểu kiến thức",
	"ghi lại 3 ý",
	"ôn lại",
}

func normalizeStr(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

func isGenericLearning(c Candidate) bool {
	if c.Type != TypeLearning {
		return false
	}
	if c.RoadmapStepID != "" {
		return false
	}
	hay := normalizeStr(c.Title) + " | " + normalizeStr(c.Description)
	for _, m := range genericLearningMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}

// canonicalKey returns the dedup key for a candidate.
func canonicalKey(c Candidate) string {
	if c.Type == TypeLearning {
		if c.RoadmapStepID != "" {
			return "learning:step:" + c.RoadmapStepID
		}
		if isGenericLearning(c) {
			return "learning:generic"
		}
	}
	return string(c.Type) + ":" + normalizeStr(c.Title)
}

// CanonicalKey returns the dedup key for a candidate.
func CanonicalKey(c Candidate) string { return canonicalKey(c) }

// dedup keeps the first occurrence of each canonical key.
func dedup(cands []Candidate) []Candidate {
	seen := make(map[string]bool, len(cands))
	out := make([]Candidate, 0, len(cands))
	for _, c := range cands {
		k := canonicalKey(c)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c)
	}
	return out
}

// Dedup removes semantic duplicates, keeping the first occurrence per canonical key.
// Pass AI candidates first to prefer keeping AI-generated quests over rule-based ones.
func Dedup(cands []Candidate) []Candidate { return dedup(cands) }

// ---------------------------------------------------------------------------
// Role detection + day plan
// ---------------------------------------------------------------------------

func (ctx Context) detectRole(c Candidate) Role {
	h := c.ReminderTime.In(ctx.Loc).Hour()
	switch c.Type {
	case TypeReview:
		if h < 12 {
			return RoleStartDay
		}
		return RoleEndReview
	case TypeMovement:
		if h >= 11 && h < 16 {
			return RoleMiddayReset
		}
		return RoleAfterWork
	case TypeLearning:
		return RoleLearning
	case TypeSleep:
		return RoleSleepPrepare
	}
	return RoleLearning
}

func (ctx Context) desiredRolePlan() []Role {
	var plan []Role
	switch ctx.DayType {
	case DayBusy:
		plan = []Role{RoleStartDay, RoleAfterWork, RoleLearning, RoleSleepPrepare}
	case DayLowEnergy:
		plan = []Role{RoleStartDay, RoleAfterWork, RoleEndReview, RoleSleepPrepare}
	case DayRest:
		plan = []Role{RoleStartDay, RoleAfterWork, RoleEndReview, RoleSleepPrepare}
	default:
		plan = []Role{RoleStartDay, RoleMiddayReset, RoleAfterWork, RoleLearning, RoleEndReview, RoleSleepPrepare}
	}
	out := make([]Role, 0, len(plan))
	for _, r := range plan {
		if ctx.EnabledCategories[roleCategory(r)] {
			out = append(out, r)
		}
	}
	return out
}

func (ctx Context) timeAt(h, m int) time.Time {
	y, mo, d := ctx.Now.In(ctx.Loc).Date()
	return time.Date(y, mo, d, h, m, 0, 0, ctx.Loc)
}

func (ctx Context) defaultReminder(r Role) time.Time {
	switch r {
	case RoleStartDay:
		return ctx.timeAt(8, 10)
	case RoleMiddayReset:
		return ctx.timeAt(12, 30)
	case RoleAfterWork:
		return ctx.WorkEnd.Add(1 * time.Hour)
	case RoleLearning:
		return ctx.WorkEnd.Add(2 * time.Hour)
	case RoleEndReview:
		return ctx.SleepTarget.Add(-60 * time.Minute)
	case RoleSleepPrepare:
		return ctx.SleepTarget.Add(-30 * time.Minute)
	}
	return ctx.WorkEnd
}

// ---------------------------------------------------------------------------
// Compose (role-based day plan)
// ---------------------------------------------------------------------------

// RuleProvider generates a fallback quest for a role when AI didn't fill it.
type RuleProvider func(role Role, ctx Context) (Candidate, bool)

// Compose deduplicates AI candidates, assigns roles, fills missing roles via
// the rule provider, and sorts the final list by reminder time.
func Compose(ctx Context, aiCands []Candidate, rule RuleProvider) []Candidate {
	deduped := dedup(aiCands)

	filled := make(map[Role]Candidate)
	for _, c := range deduped {
		r := ctx.detectRole(c)
		if _, ok := filled[r]; !ok {
			filled[r] = c
		}
	}

	plan := ctx.desiredRolePlan()
	capN := ctx.DailyQuestCount
	if capN <= 0 || capN > len(plan) {
		capN = len(plan)
	}

	final := make([]Candidate, 0, capN)
	usedGenericLearning := false

	for _, role := range plan {
		if len(final) >= capN {
			break
		}
		c, ok := filled[role]
		if !ok {
			c, ok = rule(role, ctx)
			if !ok {
				continue
			}
		}
		if roleCategory(role) == TypeLearning && isGenericLearning(c) {
			if usedGenericLearning {
				continue
			}
			usedGenericLearning = true
		}
		if c.ReminderTime.IsZero() {
			c.ReminderTime = ctx.defaultReminder(role)
		}
		final = append(final, c)
	}

	sort.Slice(final, func(i, j int) bool {
		return final[i].ReminderTime.Before(final[j].ReminderTime)
	})
	enforceMinInterval(final, ctx.MinIntervalMin)
	return final
}

func enforceMinInterval(cands []Candidate, minMin int) {
	if minMin <= 0 {
		return
	}
	gap := time.Duration(minMin) * time.Minute
	for i := 1; i < len(cands); i++ {
		earliest := cands[i-1].ReminderTime.Add(gap)
		if cands[i].ReminderTime.Before(earliest) {
			cands[i].ReminderTime = earliest
		}
	}
}
