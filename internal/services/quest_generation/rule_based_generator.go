package quest_generation

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

type RuleBasedGenerator struct {
	db *gorm.DB
}

func NewRuleBasedGenerator(db *gorm.DB) *RuleBasedGenerator {
	return &RuleBasedGenerator{db: db}
}

func (g *RuleBasedGenerator) GenerateDailyQuests(ctx context.Context, qctx *UserQuestContext) ([]models.Quest, error) {
	// Filter enabled rules that match enabled categories
	enabledRules := FilterEnabledRules(qctx.Rules, qctx.EnabledCategories)

	if len(enabledRules) == 0 {
		logger.L.Warn("no enabled rules found for user",
			zap.String("user_id", qctx.UserID.String()),
		)
		return []models.Quest{}, nil
	}

	// Generate quest count based on daily_quest_count
	targetCount := qctx.DailyQuestCount
	if targetCount <= 0 {
		targetCount = 8 // fallback
	}
	if targetCount > 20 {
		targetCount = 20
	}

	// Adjust count based on Check-In & Previous Review
	if qctx.TodayCheckIn != nil {
		if qctx.TodayCheckIn.Availability == "busy" {
			targetCount = int(float64(targetCount) * 0.7)
			if targetCount < 2 {
				targetCount = 2
			}
		}
	}
	if qctx.PreviousDailyReview != nil {
		if qctx.PreviousDailyReview.CompletionRate < 0.5 || qctx.PreviousDailyReview.SkippedQuestCount >= 3 {
			targetCount = int(float64(targetCount) * 0.7)
			if targetCount < 2 {
				targetCount = 2
			}
		}
	}

	// In late evening, reduce generated_count (targetCount) for today's generation
	isToday := timeutil.StartOfDayVN(qctx.LocalDate).Equal(timeutil.TodayVN())
	now := timeutil.NowVN()
	if isToday && now.Hour() >= 20 {
		if targetCount > 4 {
			targetCount = 4
		}
	}

	// Rest day (weekend + rest_day_enabled): lighter load. Difficulty/duration
	// are eased inside buildQuestPoolFromRules; here we cap the count.
	if qctx.IsRestDay && targetCount > 4 {
		targetCount = 4
	}

	// Build quest pool from enabled rules
	questPool := buildQuestPoolFromRules(qctx, enabledRules, qctx.Difficulty, qctx.PreferredDuration)

	// Determine priority category
	priorityType := ""
	if qctx.TodayCheckIn != nil {
		switch qctx.TodayCheckIn.Priority {
		case "health", "movement", "wellness":
			priorityType = "movement"
		case "learning", "study":
			priorityType = "learning"
		case "sleep", "rest":
			priorityType = "sleep"
		}
	}

	// Select quests respecting max_per_day limits
	selected := selectQuestsWithLimits(questPool, enabledRules, targetCount, priorityType, qctx)

	// Create quest records
	today := timeutil.StartOfDayVN(qctx.LocalDate)
	var created []models.Quest

	for _, template := range selected {
		normalizedTags := NormalizeTags(template.Tags)
		tagsJSON, _ := json.Marshal(normalizedTags)
		dueDate := today
		reminderTime := time.Date(today.Year(), today.Month(), today.Day(), template.DueHour, template.DueMinute, 0, 0, timeutil.LocationVN)

		if template.Type == models.QuestTypeSleep {
			_, calculatedReminder, err := CalculateSleepTimes(today, qctx.TargetSleepTime)
			if err == nil {
				reminderTime = calculatedReminder
			}
		}

		quest := models.Quest{
			UserID:           qctx.UserID,
			Title:            template.Title,
			Description:      template.Description,
			Type:             template.Type,
			Status:           models.QuestStatusPending,
			Difficulty:       template.Difficulty,
			Source:           models.QuestSourceConfigBased,
			XPReward:         template.XPReward,
			EstimatedMinutes: template.EstimatedMinutes,
			Reason:           template.Reason,
			Instruction:      template.Instruction,
			Tags:             datatypes.JSON(tagsJSON),
			Date:             today,
			DueDate:          &dueDate,
			ReminderTime:     &reminderTime,
			LearningMetadata: buildTemplateLearningMetadata(qctx, template),
		}
		created = append(created, quest)
	}

	// Normalize and filter quests (past reminder handling, sleep overnight, tags)
	created = NormalizeQuests(qctx, created, timeutil.NowVN())

	// Iterate by index so GORM's BeforeCreate hook and DB defaults
	// (ID, created_at, updated_at) are written back onto the returned
	// slice elements. Ranging by value would populate only a copy and
	// leave the returned records with a zero UUID / zero timestamps.
	for i := range created {
		if err := g.db.WithContext(ctx).Create(&created[i]).Error; err != nil {
			logger.L.Error("failed to create quest", zap.String("title", created[i].Title), zap.Error(err))
			return created, err
		}
	}

	return created, nil
}

// isRuleActiveOnWeekday reports whether a rule applies on the given weekday
// (1=Mon ... 7=Sun, matching the prompt/context convention). An empty Weekdays
// list means the rule applies every day (backward compatible with old data).
func isRuleActiveOnWeekday(rule QuestRuleContext, weekdayNum int) bool {
	if len(rule.Weekdays) == 0 {
		return true
	}
	for _, wd := range rule.Weekdays {
		if wd == weekdayNum {
			return true
		}
	}
	return false
}

// FilterEnabledRules returns rules that are enabled and match the enabled categories.
func FilterEnabledRules(rules []QuestRuleContext, enabledCategories []string) []QuestRuleContext {
	catSet := make(map[string]bool)
	for _, cat := range enabledCategories {
		catSet[cat] = true
		catSet[NormalizeType(cat)] = true
	}

	var result []QuestRuleContext
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if len(enabledCategories) == 0 {
			result = append(result, rule)
			continue
		}
		normType := NormalizeType(rule.Type)
		if catSet[rule.Type] || catSet[normType] {
			result = append(result, rule)
		}
	}
	return result
}

type questTemplate struct {
	Title            string
	Description      string
	Type             models.QuestType
	Difficulty       models.QuestDifficulty
	XPReward         int
	EstimatedMinutes int
	Reason           string
	Instruction      string
	Tags             []string
	DueHour          int
	DueMinute        int
	RuleID           string
}

func getHourMinuteFromPreference(pref string) (int, int) {
	switch pref {
	case "early_morning":
		return 6, 0
	case "morning":
		return 9, 0
	case "lunch":
		return 12, 0
	case "afternoon":
		return 15, 0
	case "after_work":
		return 18, 0
	case "evening":
		return 20, 0
	case "night":
		return 22, 0
	default:
		return 10, 0
	}
}

func parseHHMM(hhmm string, defaultHour, defaultMinute int) (int, int) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err == nil {
		return h, m
	}
	return defaultHour, defaultMinute
}

func isMovementGentleRequired(qctx *UserQuestContext) bool {
	if qctx.ActivityLevel == "sedentary" || qctx.LastWorkout == "long_ago" || qctx.LastWorkout == "never" {
		return true
	}
	if qctx.TodayCheckIn != nil && (qctx.TodayCheckIn.EnergyLevel == "low" || qctx.TodayCheckIn.EnergyLevel == "very_low") {
		return true
	}
	for _, lim := range qctx.HealthLimitations {
		limLower := strings.ToLower(lim)
		if limLower == "back_pain" || limLower == "knee_pain" || limLower == "low_energy" || limLower == "limited_mobility" || limLower == "injury_recovery" {
			return true
		}
		if strings.Contains(limLower, "lưng") || strings.Contains(limLower, "gối") || strings.Contains(limLower, "khớp") || strings.Contains(limLower, "mệt") {
			return true
		}
	}
	return false
}

func buildQuestPoolFromRules(qctx *UserQuestContext, rules []QuestRuleContext, globalDifficulty string, preferredDuration string) []questTemplate {
	var pool []questTemplate

	// Workday check
	weekday := qctx.LocalDate.Weekday()
	weekdayNum := int(weekday)
	if weekdayNum == 0 {
		weekdayNum = 7
	}
	isWorkday := false
	for _, wd := range qctx.WorkWeekdays {
		if wd == weekdayNum {
			isWorkday = true
			break
		}
	}

	// Cap difficulty and duration if low energy today or low completion yesterday
	reduceDifficulty := false
	if qctx.TodayCheckIn != nil && (qctx.TodayCheckIn.EnergyLevel == "low" || qctx.TodayCheckIn.EnergyLevel == "very_low") {
		reduceDifficulty = true
	}
	if qctx.PreviousDailyReview != nil && qctx.PreviousDailyReview.CompletionRate < 0.5 {
		reduceDifficulty = true
	}
	// Rest day: keep everything gentle (easy difficulty + short duration).
	if qctx.IsRestDay {
		reduceDifficulty = true
	}

	// When low energy, shorten duration to keep quests manageable
	effectiveDuration := preferredDuration
	if reduceDifficulty && effectiveDuration != "short" {
		effectiveDuration = "short"
	}

	for _, rule := range rules {
		// Skip rules that are not active on today's weekday (active_weekdays).
		// An empty Weekdays list means the rule applies every day.
		if !isRuleActiveOnWeekday(rule, weekdayNum) {
			continue
		}

		difficulty := mapRuleDifficultyToQuestDifficulty(rule.Difficulty)
		if reduceDifficulty {
			if difficulty == models.QuestDifficultyHard {
				difficulty = models.QuestDifficultyMedium
			} else if difficulty == models.QuestDifficultyMedium {
				difficulty = models.QuestDifficultyEasy
			}
		}
		estimatedMinutes := adjustDurationByPreference(rule.Type, effectiveDuration)

		if rule.Type == "water" || rule.Type == "breakTime" {
			// Reminder-only: generate 0 quests
			continue
		}

		if rule.Type == "movement" {
			dueHour, dueMinute := 10, 0
			if rule.ActiveTimeRange != nil && rule.ActiveTimeRange.Start != "" {
				_, _ = fmt.Sscanf(rule.ActiveTimeRange.Start, "%d:%d", &dueHour, &dueMinute)
			} else if len(qctx.MovementTimePreferences) > 0 {
				pref := qctx.MovementTimePreferences[0]
				if pref == "after_work" && !isWorkday {
					dueHour, dueMinute = 16, 0 // fallback from after_work to 16:00
				} else {
					dueHour, dueMinute = getHourMinuteFromPreference(pref)
				}
			}

			maxPerDay := 1
			if rule.MaxPerDay != nil {
				maxPerDay = *rule.MaxPerDay
			}

			gentle := isMovementGentleRequired(qctx)

			if maxPerDay <= 1 {
				if gentle {
					difficulty = models.QuestDifficultyEasy
					pool = append(pool, questTemplate{
						Title:            "Vận động nhẹ nhàng 10 phút",
						Description:      "Đi bộ nhẹ nhàng, giãn cơ cơ bản hoặc vận động linh hoạt.",
						Type:             models.QuestTypeMovement,
						Difficulty:       difficulty,
						XPReward:         calculateXPByDifficulty(difficulty),
						EstimatedMinutes: 10,
						Reason:           "Vận động nhẹ giúp tăng tuần hoàn máu và giảm mỏi mệt khi ngồi lâu",
						Instruction:      "Thực hiện các động tác giãn cơ nhẹ hoặc đi bộ chậm trong 10 phút, dừng lại ngay nếu thấy khó chịu.",
						Tags:             []string{"vận động", "sức khỏe"},
						DueHour:          dueHour,
						DueMinute:        dueMinute,
						RuleID:           rule.ID,
					})
				} else {
					pool = append(pool, questTemplate{
						Title:            "Vận động nhẹ 10 phút",
						Description:      "Đứng dậy đi lại hoặc giãn cơ nhẹ.",
						Type:             models.QuestTypeMovement,
						Difficulty:       difficulty,
						XPReward:         calculateXPByDifficulty(difficulty),
						EstimatedMinutes: 10,
						Reason:           "Vận động nhẹ giúp tăng tuần hoàn máu và giảm mỏi mệt khi ngồi lâu",
						Instruction:      "Đứng dậy đi lại nhẹ nhàng, vươn vai hoặc giãn cơ cơ bản trong 10 phút.",
						Tags:             []string{"vận động", "sức khỏe"},
						DueHour:          dueHour,
						DueMinute:        dueMinute,
						RuleID:           rule.ID,
					})
				}
			} else {
				templates := getTemplatesForType("movement")
				for _, tmpl := range templates {
					if !isWorkday && (strings.Contains(tmpl.Title, "làm việc") || strings.Contains(tmpl.Description, "làm việc")) {
						continue
					}
					diff := difficulty
					title := tmpl.Title
					desc := tmpl.Description
					inst := tmpl.Instruction
					if gentle {
						diff = models.QuestDifficultyEasy
						if title == "Bài tập giãn cơ" {
							title = "Giãn cơ nhẹ nhàng"
							desc = "Giãn cơ cơ bản nhẹ nhàng tại chỗ."
							inst = "Thực hiện giãn cơ nhẹ, tránh kéo căng quá mức, dừng lại nếu đau."
						}
					}
					pool = append(pool, questTemplate{
						Title:            title,
						Description:      desc,
						Type:             tmpl.Type,
						Difficulty:       diff,
						XPReward:         calculateXPByDifficulty(diff),
						EstimatedMinutes: estimatedMinutes,
						Reason:           tmpl.Reason,
						Instruction:      inst,
						Tags:             tmpl.Tags,
						DueHour:          dueHour,
						DueMinute:        dueMinute,
						RuleID:           rule.ID,
					})
				}
			}
			continue
		}

		if rule.Type == "learning" {
			dueHour, dueMinute := 20, 0
			if rule.ActiveTimeRange != nil && rule.ActiveTimeRange.Start != "" {
				_, _ = fmt.Sscanf(rule.ActiveTimeRange.Start, "%d:%d", &dueHour, &dueMinute)
			} else if len(qctx.LearningTimePreferences) > 0 {
				pref := qctx.LearningTimePreferences[0]
				if pref == "after_work" && !isWorkday {
					dueHour, dueMinute = 20, 0 // fallback from after_work to 20:00
				} else {
					dueHour, dueMinute = getHourMinuteFromPreference(pref)
				}
			}

			if qctx.ActiveLearningPath != nil && qctx.ActiveLearningPath.CurrentStepTitle != "" {
				desc := qctx.ActiveLearningPath.Description
				if desc == "" {
					desc = fmt.Sprintf("Học chủ đề %s thuộc lộ trình %s.", qctx.ActiveLearningPath.CurrentStepTitle, qctx.ActiveLearningPath.RoadmapTitle)
				}
				pool = append(pool, questTemplate{
					Title:            fmt.Sprintf("Học tập: %s", qctx.ActiveLearningPath.CurrentStepTitle),
					Description:      desc,
					Type:             models.QuestTypeLearning,
					Difficulty:       difficulty,
					XPReward:         calculateXPByDifficulty(difficulty),
					EstimatedMinutes: estimatedMinutes,
					Reason:           fmt.Sprintf("Tiếp tục lộ trình học tập %s", qctx.ActiveLearningPath.RoadmapTitle),
					Instruction:      fmt.Sprintf("Dành thời gian hoàn thành bước học tập: %s", qctx.ActiveLearningPath.CurrentStepTitle),
					Tags:             []string{"học tập", "roadmap"},
					DueHour:          dueHour,
					DueMinute:        dueMinute,
					RuleID:           rule.ID,
				})
			} else {
				// No active learning path: keep the fallback concrete enough to
				// be actionable (pick a topic + capture key points) instead of a
				// vague "Học tập 20 phút".
				pool = append(pool, questTemplate{
					Title:            "Chọn một chủ đề và ghi lại 3 ý chính",
					Description:      "Chọn một chủ đề bạn muốn học hôm nay và tìm hiểu khoảng 20 phút. Ghi lại 3 ý chính bạn học được.",
					Type:             models.QuestTypeLearning,
					Difficulty:       difficulty,
					XPReward:         calculateXPByDifficulty(difficulty),
					EstimatedMinutes: 20,
					Reason:           "Học tập đều đặn mỗi ngày giúp tích lũy kiến thức lâu dài",
					Instruction:      "Tập trung khoảng 20 phút không bị phân tâm. Sau khi xong, ghi lại 3 ý chính bạn vừa học được.",
					Tags:             []string{"học tập"},
					DueHour:          dueHour,
					DueMinute:        dueMinute,
					RuleID:           rule.ID,
				})
			}
			continue
		}

		if rule.Type == "sleep" {
			dueHour, dueMinute := 22, 30
			if qctx.TargetSleepTime != "" {
				dueHour, dueMinute = parseHHMM(qctx.TargetSleepTime, 22, 30)
			}

			title := "Chuẩn bị đi ngủ đúng giờ"
			desc := "Tắt màn hình và chuẩn bị cho giấc ngủ."
			if qctx.TargetSleepTime != "" {
				title = fmt.Sprintf("Chuẩn bị đi ngủ đúng giờ (%s)", qctx.TargetSleepTime)
				desc = fmt.Sprintf("Tắt màn hình và chuẩn bị đi ngủ trước %s.", qctx.TargetSleepTime)
			}

			pool = append(pool, questTemplate{
				Title:            title,
				Description:      desc,
				Type:             models.QuestTypeSleep,
				Difficulty:       difficulty,
				XPReward:         calculateXPByDifficulty(difficulty),
				EstimatedMinutes: estimatedMinutes,
				Reason:           "Ngủ đủ giấc là nền tảng cho sức khỏe và hiệu suất làm việc",
				Instruction:      "Tắt các thiết bị điện tử trước khi ngủ 15-30 phút và đi ngủ đúng giờ.",
				Tags:             []string{"giấc ngủ", "sức khỏe"},
				DueHour:          dueHour,
				DueMinute:        dueMinute,
				RuleID:           rule.ID,
			})
			continue
		}

		templates := getTemplatesForType(rule.Type)
		for _, tmpl := range templates {
			if !isWorkday && (strings.Contains(tmpl.Title, "làm việc") || strings.Contains(tmpl.Description, "làm việc") || strings.Contains(tmpl.Title, "coding")) {
				continue
			}
			dueHour, dueMinute := tmpl.DueHour, tmpl.DueMinute
			if rule.ActiveTimeRange != nil && rule.ActiveTimeRange.Start != "" {
				_, _ = fmt.Sscanf(rule.ActiveTimeRange.Start, "%d:%d", &dueHour, &dueMinute)
			}
			pool = append(pool, questTemplate{
				Title:            tmpl.Title,
				Description:      tmpl.Description,
				Type:             tmpl.Type,
				Difficulty:       difficulty,
				XPReward:         calculateXPByDifficulty(difficulty),
				EstimatedMinutes: estimatedMinutes,
				Reason:           tmpl.Reason,
				Instruction:      tmpl.Instruction,
				Tags:             tmpl.Tags,
				DueHour:          dueHour,
				DueMinute:        dueMinute,
				RuleID:           rule.ID,
			})
		}
	}

	return pool
}

func selectQuestsWithLimits(pool []questTemplate, rules []QuestRuleContext, targetCount int, priorityType string, qctx *UserQuestContext) []questTemplate {
	maxPerDay := make(map[string]int)
	for _, rule := range rules {
		if rule.MaxPerDay != nil {
			maxPerDay[rule.ID] = *rule.MaxPerDay
		} else {
			maxPerDay[rule.ID] = 999
		}
	}

	countPerRule := make(map[string]int)

	// Pre-populate countPerRule from preserved (non-pending) quests to respect max_per_day globally
	if qctx != nil && len(qctx.ExistingQuestTypeCount) > 0 {
		for existingType, count := range qctx.ExistingQuestTypeCount {
			normExisting := NormalizeType(existingType)
			for _, rule := range rules {
				if NormalizeType(rule.Type) == normExisting {
					countPerRule[rule.ID] += count
					break
				}
			}
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	// Sort prioritized templates to the front: check-in priority first, then review, then sleep, then others
	var priorityCandidates []questTemplate
	var reviewCandidates []questTemplate
	var sleepCandidates []questTemplate
	var others []questTemplate

	for _, q := range pool {
		if priorityType != "" && string(q.Type) == priorityType {
			priorityCandidates = append(priorityCandidates, q)
		} else if string(q.Type) == "review" {
			reviewCandidates = append(reviewCandidates, q)
		} else if string(q.Type) == "sleep" {
			sleepCandidates = append(sleepCandidates, q)
		} else {
			others = append(others, q)
		}
	}

	pool = append(append(append(priorityCandidates, reviewCandidates...), sleepCandidates...), others...)

	// Pre-populate usedTitles with titles of preserved quests to avoid duplicate titles
	usedTitles := make(map[string]bool)
	if qctx != nil {
		for _, title := range qctx.ExistingQuestTitles {
			usedTitles[strings.ToLower(strings.TrimSpace(title))] = true
		}
	}

	var selected []questTemplate

	for _, template := range pool {
		if len(selected) >= targetCount {
			break
		}
		if countPerRule[template.RuleID] >= maxPerDay[template.RuleID] {
			continue
		}
		lowerTitle := strings.ToLower(strings.TrimSpace(template.Title))
		if usedTitles[lowerTitle] {
			continue
		}

		selected = append(selected, template)
		countPerRule[template.RuleID]++
		usedTitles[lowerTitle] = true
	}

	return selected
}

func mapRuleDifficultyToQuestDifficulty(ruleDifficulty string) models.QuestDifficulty {
	switch ruleDifficulty {
	case "easy":
		return models.QuestDifficultyEasy
	case "medium":
		return models.QuestDifficultyMedium
	case "hard":
		return models.QuestDifficultyHard
	default:
		return models.QuestDifficultyMedium
	}
}

func adjustDurationByPreference(questType string, preference string) int {
	base := getBaseDuration(questType)
	switch preference {
	case "short":
		return int(float64(base) * 0.7)
	case "long":
		return int(float64(base) * 1.5)
	default:
		return base
	}
}

func getBaseDuration(questType string) int {
	switch questType {
	case "water":
		return 2
	case "breakTime":
		return 5
	case "movement":
		return 10
	case "learning":
		return 20
	case "sleep":
		return 15
	case "review":
		return 10
	default:
		return 10
	}
}

func buildTemplateLearningMetadata(qctx *UserQuestContext, template questTemplate) datatypes.JSON {
	if template.Type != models.QuestTypeLearning {
		return nil
	}
	return BuildLearningMetadata(qctx.ActiveLearningPath)
}

func calculateXPByDifficulty(difficulty models.QuestDifficulty) int {
	switch difficulty {
	case models.QuestDifficultyEasy:
		return 5
	case models.QuestDifficultyMedium:
		return 10
	case models.QuestDifficultyHard:
		return 20
	default:
		return 10
	}
}

type templateDef struct {
	Title       string
	Description string
	Type        models.QuestType
	Reason      string
	Instruction string
	Tags        []string
	DueHour     int
	DueMinute   int
}

func getTemplatesForType(questType string) []templateDef {
	allTemplates := buildAllTemplates()
	normType := NormalizeType(questType)
	var result []templateDef
	for _, t := range allTemplates {
		if string(t.Type) == normType || NormalizeType(string(t.Type)) == normType {
			result = append(result, t)
		}
	}
	return result
}

func buildAllTemplates() []templateDef {
	// NOTE: water and breakTime/eyeBreak are reminder habits handled by the
	// reminder module, NOT daily quests, so their templates are intentionally
	// NOT included here. See AllowedDailyQuestTypes / IsReminderOnlyDailyType.
	return []templateDef{
		// Movement
		{Title: "Đi bộ ngắn", Description: "Đi bộ nhẹ nhàng trong vài phút", Type: models.QuestTypeMovement, Reason: "Vận động nhẹ giúp tuần hoàn máu tốt hơn", Instruction: "Đi bộ quanh phòng hoặc ngoài trời 10 phút", Tags: []string{"vận động", "sức khỏe"}, DueHour: 12, DueMinute: 0},
		{Title: "Bài tập giãn cơ", Description: "Thực hiện vài động tác giãn cơ đơn giản", Type: models.QuestTypeMovement, Reason: "Phòng tránh đau lưng và mỏi cổ khi ngồi lâu", Instruction: "Thực hiện 5 động tác giãn cơ cơ bản trong 10 phút", Tags: []string{"vận động", "sức khỏe"}, DueHour: 16, DueMinute: 0},
		{Title: "Đứng làm việc 15 phút", Description: "Chuyển sang đứng làm việc thay vì ngồi", Type: models.QuestTypeMovement, Reason: "Giảm áp lực lên cột sống khi ngồi quá lâu", Instruction: "Đứng làm việc ít nhất 15 phút", Tags: []string{"vận động", "sức khỏe"}, DueHour: 10, DueMinute: 0},

		// Learning
		{Title: "Đọc 10 trang sách", Description: "Đọc sách chuyên ngành hoặc phát triển bản thân", Type: models.QuestTypeLearning, Reason: "Đọc sách đều đặn giúp mở rộng kiến thức và tư duy", Instruction: "Chọn một cuốn sách và đọc ít nhất 10 trang", Tags: []string{"học tập", "đọc sách"}, DueHour: 20, DueMinute: 0},
		{Title: "Học từ vựng mới", Description: "Học và ghi nhớ từ vựng ngoại ngữ", Type: models.QuestTypeLearning, Reason: "Học ngoại ngữ mỗi ngày giúp tiến bộ nhanh chóng", Instruction: "Học 10 từ vựng mới và đặt câu với mỗi từ", Tags: []string{"học tập", "ngoại ngữ"}, DueHour: 9, DueMinute: 0},
		{Title: "Xem video học tập", Description: "Xem một video giáo dục ngắn về chủ đề quan tâm", Type: models.QuestTypeLearning, Reason: "Học qua video giúp tiếp thu nhanh và sinh động", Instruction: "Xem một video giáo dục và ghi lại 3 ý chính", Tags: []string{"học tập", "video"}, DueHour: 13, DueMinute: 0},
		{Title: "Thực hành coding 30 phút", Description: "Luyện tập lập trình với một bài toán nhỏ", Type: models.QuestTypeLearning, Reason: "Luyện tập đều đặn giúp nâng cao kỹ năng lập trình", Instruction: "Giải một bài toán coding hoặc làm một tính năng nhỏ", Tags: []string{"học tập", "coding"}, DueHour: 17, DueMinute: 0},
		{Title: "Viết ghi chú kiến thức", Description: "Tổng hợp và viết lại kiến thức đã học hôm nay", Type: models.QuestTypeLearning, Reason: "Viết lại giúp ghi nhớ sâu hơn và hệ thống kiến thức", Instruction: "Viết ít nhất 200 từ tổng hợp kiến thức hôm nay", Tags: []string{"học tập", "ghi chú"}, DueHour: 21, DueMinute: 0},

		// Review
		{Title: "Daily review", Description: "Nhìn lại những việc đã làm hôm nay", Type: models.QuestTypeReview, Reason: "Review giúp nhận diện điểm mạnh và điểm cần cải thiện", Instruction: "Ghi lại 3 điều tốt, 1 điều khó, và 1 điều muốn làm tốt hơn", Tags: []string{"review", "phản ánh"}, DueHour: 21, DueMinute: 30},

		// Sleep
		{Title: "Chuẩn bị đi ngủ đúng giờ", Description: "Tắt màn hình và chuẩn bị cho giấc ngủ", Type: models.QuestTypeSleep, Reason: "Ngủ đủ giấc là nền tảng cho sức khỏe và hiệu suất", Instruction: "Tắt điện thoại, máy tính trước khi ngủ 15 phút, thư giãn nhẹ nhàng", Tags: []string{"giấc ngủ", "sức khỏe"}, DueHour: 22, DueMinute: 30},
		{Title: "Không màn hình trước ngủ", Description: "Tránh ánh sáng xanh 30 phút trước khi ngủ", Type: models.QuestTypeSleep, Reason: "Ánh sáng xanh làm giảm melatonin và ảnh hưởng chất lượng giấc ngủ", Instruction: "Không dùng điện thoại hoặc máy tính 30 phút trước khi ngủ", Tags: []string{"giấc ngủ", "kỷ luật"}, DueHour: 22, DueMinute: 0},
	}
}
