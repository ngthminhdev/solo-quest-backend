package quest_generation

import (
	"context"
	"encoding/json"
	"math/rand"
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
	enabledRules := filterEnabledRules(qctx.Rules, qctx.EnabledCategories)

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

	// Build quest pool from enabled rules
	questPool := buildQuestPoolFromRules(enabledRules, qctx.Difficulty, qctx.PreferredDuration)

	// Select quests respecting max_per_day limits
	selected := selectQuestsWithLimits(questPool, enabledRules, targetCount)

	// Create quest records
	today := timeutil.StartOfDayVN(qctx.LocalDate)
	var created []models.Quest

	for _, template := range selected {
		tagsJSON, _ := json.Marshal(template.Tags)
		dueDate := today
		reminderTime := time.Date(today.Year(), today.Month(), today.Day(), template.DueHour, template.DueMinute, 0, 0, timeutil.LocationVN)

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
		}

		if err := g.db.WithContext(ctx).Create(&quest).Error; err != nil {
			logger.L.Error("failed to create quest", zap.String("title", template.Title), zap.Error(err))
			return created, err
		}
		created = append(created, quest)
	}

	return created, nil
}

func filterEnabledRules(rules []QuestRuleContext, enabledCategories []string) []QuestRuleContext {
	catSet := make(map[string]bool)
	for _, cat := range enabledCategories {
		catSet[cat] = true
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
		if catSet[rule.Type] {
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

func buildQuestPoolFromRules(rules []QuestRuleContext, globalDifficulty string, preferredDuration string) []questTemplate {
	var pool []questTemplate

	for _, rule := range rules {
		templates := getTemplatesForType(rule.Type)
		difficulty := mapRuleDifficultyToQuestDifficulty(rule.Difficulty)
		estimatedMinutes := adjustDurationByPreference(rule.Type, preferredDuration)

		for _, tmpl := range templates {
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
				DueHour:          tmpl.DueHour,
				DueMinute:        tmpl.DueMinute,
				RuleID:           rule.ID,
			})
		}
	}

	return pool
}

func selectQuestsWithLimits(pool []questTemplate, rules []QuestRuleContext, targetCount int) []questTemplate {
	maxPerDay := make(map[string]int)
	for _, rule := range rules {
		if rule.MaxPerDay != nil {
			maxPerDay[rule.ID] = *rule.MaxPerDay
		} else {
			maxPerDay[rule.ID] = 999
		}
	}

	countPerRule := make(map[string]int)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	var selected []questTemplate
	usedTitles := make(map[string]bool)

	for _, template := range pool {
		if len(selected) >= targetCount {
			break
		}
		if countPerRule[template.RuleID] >= maxPerDay[template.RuleID] {
			continue
		}
		if usedTitles[template.Title] {
			continue
		}

		selected = append(selected, template)
		countPerRule[template.RuleID]++
		usedTitles[template.Title] = true
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
	var result []templateDef
	for _, t := range allTemplates {
		if string(t.Type) == questType {
			result = append(result, t)
		}
	}
	return result
}

func buildAllTemplates() []templateDef {
	return []templateDef{
		// Water
		{Title: "Uống nước buổi sáng", Description: "Uống một cốc nước đầy sau khi ngủ dậy", Type: models.QuestTypeWater, Reason: "Bắt đầu ngày mới đủ nước giúp tinh thần tỉnh táo", Instruction: "Uống ít nhất 250ml nước ngay sau khi thức dậy", Tags: []string{"sức khỏe", "hydration"}, DueHour: 7, DueMinute: 30},
		{Title: "Uống nước giữa giờ", Description: "Uống một cốc nước trong lúc làm việc", Type: models.QuestTypeWater, Reason: "Cơ thể cần nước liên tục để giữ năng lượng", Instruction: "Uống ít nhất 250ml nước giữa buổi làm việc", Tags: []string{"sức khỏe", "hydration"}, DueHour: 11, DueMinute: 0},
		{Title: "Uống nước buổi chiều", Description: "Bổ sung nước cho buổi chiều làm việc", Type: models.QuestTypeWater, Reason: "Tránh mệt mỏi do thiếu nước buổi chiều", Instruction: "Uống ít nhất 250ml nước vào đầu giờ chiều", Tags: []string{"sức khỏe", "hydration"}, DueHour: 15, DueMinute: 0},

		// Break Time
		{Title: "Nghỉ mắt 5 phút", Description: "Rời mắt khỏi màn hình và thư giãn thị giác", Type: models.QuestTypeBreak, Reason: "Mắt cần nghỉ ngơi sau thời gian tập trung màn hình", Instruction: "Nhắm mắt hoặc nhìn ra xa ít nhất 5 phút", Tags: []string{"sức khỏe", "tập trung"}, DueHour: 10, DueMinute: 30},
		{Title: "Vươn vai thư giãn", Description: "Đứng dậy và vươn vai trong vài phút", Type: models.QuestTypeBreak, Reason: "Ngồi lâu khiến cơ bắp căng cứng", Instruction: "Đứng dậy, vươn vai và xoay cổ nhẹ nhàng 3 phút", Tags: []string{"sức khỏe", "vận động"}, DueHour: 14, DueMinute: 30},

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
