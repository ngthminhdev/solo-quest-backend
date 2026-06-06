package services

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
	"solo_quest_backend/pkg/logger"
)

type devQuestTemplate struct {
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
}

func buildDevQuestTemplates() []devQuestTemplate {
	return []devQuestTemplate{
		// --- Wellness (water, breakTime, movement) ---
		{
			Title:            "Uống nước buổi sáng",
			Description:      "Uống một cốc nước đầy sau khi ngủ dậy",
			Type:             models.QuestTypeWater,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         5,
			EstimatedMinutes: 2,
			Reason:           "Bắt đầu ngày mới đủ nước giúp tinh thần tỉnh táo",
			Instruction:      "Uống ít nhất 250ml nước ngay sau khi thức dậy",
			Tags:             []string{"sức khỏe", "hydration"},
			DueHour:          7,
			DueMinute:        30,
		},
		{
			Title:            "Uống nước giữa giờ",
			Description:      "Uống một cốc nước trong lúc làm việc",
			Type:             models.QuestTypeWater,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         5,
			EstimatedMinutes: 2,
			Reason:           "Cơ thể cần nước liên tục để giữ năng lượng",
			Instruction:      "Uống ít nhất 250ml nước giữa buổi làm việc",
			Tags:             []string{"sức khỏe", "hydration"},
			DueHour:          11,
			DueMinute:        0,
		},
		{
			Title:            "Uống nước buổi chiều",
			Description:      "Bổ sung nước cho buổi chiều làm việc",
			Type:             models.QuestTypeWater,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         5,
			EstimatedMinutes: 2,
			Reason:           "Tránh mệt mỏi do thiếu nước buổi chiều",
			Instruction:      "Uống ít nhất 250ml nước vào đầu giờ chiều",
			Tags:             []string{"sức khỏe", "hydration"},
			DueHour:          15,
			DueMinute:        0,
		},
		{
			Title:            "Nghỉ mắt 5 phút",
			Description:      "Rời mắt khỏi màn hình và thư giãn thị giác",
			Type:             models.QuestTypeBreak,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         5,
			EstimatedMinutes: 5,
			Reason:           "Mắt cần nghỉ ngơi sau thời gian tập trung màn hình",
			Instruction:      "Nhắm mắt hoặc nhìn ra xa ít nhất 5 phút",
			Tags:             []string{"sức khỏe", "tập trung"},
			DueHour:          10,
			DueMinute:        30,
		},
		{
			Title:            "Vươn vai thư giãn",
			Description:      "Đứng dậy và vươn vai trong vài phút",
			Type:             models.QuestTypeBreak,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         5,
			EstimatedMinutes: 3,
			Reason:           "Ngồi lâu khiến cơ bắp căng cứng",
			Instruction:      "Đứng dậy, vươn vai và xoay cổ nhẹ nhàng 3 phút",
			Tags:             []string{"sức khỏe", "vận động"},
			DueHour:          14,
			DueMinute:        30,
		},
		{
			Title:            "Đi bộ ngắn",
			Description:      "Đi bộ nhẹ nhàng trong vài phút",
			Type:             models.QuestTypeMovement,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Vận động nhẹ giúp tuần hoàn máu tốt hơn",
			Instruction:      "Đi bộ quanh phòng hoặc ngoài trời 10 phút",
			Tags:             []string{"vận động", "sức khỏe"},
			DueHour:          12,
			DueMinute:        0,
		},
		{
			Title:            "Bài tập giãn cơ",
			Description:      "Thực hiện vài động tác giãn cơ đơn giản",
			Type:             models.QuestTypeMovement,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Phòng tránh đau lưng và mỏi cổ khi ngồi lâu",
			Instruction:      "Thực hiện 5 động tác giãn cơ cơ bản trong 10 phút",
			Tags:             []string{"vận động", "sức khỏe"},
			DueHour:          16,
			DueMinute:        0,
		},
		{
			Title:            "Đứng làm việc 15 phút",
			Description:      "Chuyển sang đứng làm việc thay vì ngồi",
			Type:             models.QuestTypeMovement,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 15,
			Reason:           "Giảm áp lực lên cột sống khi ngồi quá lâu",
			Instruction:      "Đứng làm việc ít nhất 15 phút",
			Tags:             []string{"vận động", "sức khỏe"},
			DueHour:          10,
			DueMinute:        0,
		},
		// --- Learning ---
		{
			Title:            "Đọc 10 trang sách",
			Description:      "Đọc sách chuyên ngành hoặc phát triển bản thân",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyMedium,
			XPReward:         15,
			EstimatedMinutes: 20,
			Reason:           "Đọc sách đều đặn giúp mở rộng kiến thức và tư duy",
			Instruction:      "Chọn một cuốn sách và đọc ít nhất 10 trang",
			Tags:             []string{"học tập", "đọc sách"},
			DueHour:          20,
			DueMinute:        0,
		},
		{
			Title:            "Học từ vựng mới",
			Description:      "Học và ghi nhớ từ vựng ngoại ngữ",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyMedium,
			XPReward:         15,
			EstimatedMinutes: 15,
			Reason:           "Học ngoại ngữ mỗi ngày giúp tiến bộ nhanh chóng",
			Instruction:      "Học 10 từ vựng mới và đặt câu với mỗi từ",
			Tags:             []string{"học tập", "ngoại ngữ"},
			DueHour:          9,
			DueMinute:        0,
		},
		{
			Title:            "Xem video học tập",
			Description:      "Xem một video giáo dục ngắn về chủ đề quan tâm",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 15,
			Reason:           "Học qua video giúp tiếp thu nhanh và sinh động",
			Instruction:      "Xem một video giáo dục và ghi lại 3 ý chính",
			Tags:             []string{"học tập", "video"},
			DueHour:          13,
			DueMinute:        0,
		},
		{
			Title:            "Thực hành coding 30 phút",
			Description:      "Luyện tập lập trình với một bài toán nhỏ",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyHard,
			XPReward:         20,
			EstimatedMinutes: 30,
			Reason:           "Luyện tập đều đặn giúp nâng cao kỹ năng lập trình",
			Instruction:      "Giải một bài toán coding hoặc làm một tính năng nhỏ",
			Tags:             []string{"học tập", "coding"},
			DueHour:          17,
			DueMinute:        0,
		},
		{
			Title:            "Viết ghi chú kiến thức",
			Description:      "Tổng hợp và viết lại kiến thức đã học hôm nay",
			Type:             models.QuestTypeLearning,
			Difficulty:       models.QuestDifficultyMedium,
			XPReward:         10,
			EstimatedMinutes: 15,
			Reason:           "Viết lại giúp ghi nhớ sâu hơn và hệ thống kiến thức",
			Instruction:      "Viết ít nhất 200 từ tổng hợp kiến thức hôm nay",
			Tags:             []string{"học tập", "ghi chú"},
			DueHour:          21,
			DueMinute:        0,
		},
		// --- Review / Reflection ---
		{
			Title:            "Daily review",
			Description:      "Nhìn lại những việc đã làm hôm nay",
			Type:             models.QuestTypeReview,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Review giúp nhận diện điểm mạnh và điểm cần cải thiện",
			Instruction:      "Ghi lại 3 điều tốt, 1 điều khó, và 1 điều muốn làm tốt hơn",
			Tags:             []string{"review", "phản ánh"},
			DueHour:          21,
			DueMinute:        30,
		},
		{
			Title:            "Viết nhật ký biết ơn",
			Description:      "Viết ra những điều bạn biết ơn hôm nay",
			Type:             models.QuestTypeReflection,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Thực hành lòng biết ơn giúp tăng hạnh phúc và giảm stress",
			Instruction:      "Viết ít nhất 3 điều bạn biết ơn trong ngày hôm nay",
			Tags:             []string{"phản ánh", "biết ơn"},
			DueHour:          21,
			DueMinute:        0,
		},
		{
			Title:            "Thiền 5 phút",
			Description:      "Ngồi thiền và tập trung vào hơi thở",
			Type:             models.QuestTypeReflection,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 5,
			Reason:           "Thiền giúp giảm căng thẳng và cải thiện tập trung",
			Instruction:      "Ngồi yên, nhắm mắt, tập trung vào hơi thở trong 5 phút",
			Tags:             []string{"phản ánh", "thiền"},
			DueHour:          8,
			DueMinute:        0,
		},
		{
			Title:            "Lên kế hoạch ngày mai",
			Description:      "Viết ra 3 việc quan trọng nhất cho ngày mai",
			Type:             models.QuestTypeReflection,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 10,
			Reason:           "Lên kế hoạch trước giúp ngày mai bắt đầu hiệu quả hơn",
			Instruction:      "Viết ra 3 nhiệm vụ quan trọng nhất cho ngày mai và ước lượng thời gian",
			Tags:             []string{"phản ánh", "kế hoạch"},
			DueHour:          22,
			DueMinute:        0,
		},
		// --- Sleep ---
		{
			Title:            "Chuẩn bị đi ngủ đúng giờ",
			Description:      "Tắt màn hình và chuẩn bị cho giấc ngủ",
			Type:             models.QuestTypeSleep,
			Difficulty:       models.QuestDifficultyEasy,
			XPReward:         10,
			EstimatedMinutes: 15,
			Reason:           "Ngủ đủ giấc là nền tảng cho sức khỏe và hiệu suất",
			Instruction:      "Tắt điện thoại, máy tính trước khi ngủ 15 phút, thư giãn nhẹ nhàng",
			Tags:             []string{"giấc ngủ", "sức khỏe"},
			DueHour:          22,
			DueMinute:        30,
		},
		{
			Title:            "Không màn hình trước ngủ",
			Description:      "Tránh ánh sáng xanh 30 phút trước khi ngủ",
			Type:             models.QuestTypeSleep,
			Difficulty:       models.QuestDifficultyMedium,
			XPReward:         10,
			EstimatedMinutes: 30,
			Reason:           "Ánh sáng xanh làm giảm melatonin và ảnh hưởng chất lượng giấc ngủ",
			Instruction:      "Không dùng điện thoại hoặc máy tính 30 phút trước khi ngủ",
			Tags:             []string{"giấc ngủ", "kỷ luật"},
			DueHour:          22,
			DueMinute:        0,
		},
	}
}

type DevQuestGenerator struct {
	db        *gorm.DB
	devUserID uuid.UUID
}

func NewDevQuestGenerator(db *gorm.DB) *DevQuestGenerator {
	return &DevQuestGenerator{
		db:        db,
		devUserID: devUserUUID,
	}
}

func (g *DevQuestGenerator) GetDevUserID() uuid.UUID {
	return g.devUserID
}

func (g *DevQuestGenerator) IsDevUser(userID uuid.UUID) bool {
	return userID == g.devUserID
}

func (g *DevQuestGenerator) CountQuestsForDate(userID uuid.UUID, date time.Time) (int64, error) {
	start, end := timeutil.DayRangeVN(date)
	var count int64
	err := g.db.Model(&models.Quest{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, start, end).
		Count(&count).Error
	return count, err
}

// GenerateDevDailyQuests generates exactly 10 random dev quests for the given user and date.
// Returns the created quests. Idempotent: does nothing if quests already exist for that date.
func (g *DevQuestGenerator) GenerateDevDailyQuests(userID uuid.UUID, date time.Time) ([]models.Quest, error) {
	count, err := g.CountQuestsForDate(userID, date)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}

	templates := buildDevQuestTemplates()

	// Ensure quest type diversity:
	//   - At least 2 wellness (water/break/movement)
	//   - At least 1 learning
	//   - At least 1 review/reflection
	//   - Total exactly 10
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	selected := selectDiverseQuests(rng, templates, 10)

	today := timeutil.StartOfDayVN(date)
	var created []models.Quest

	for _, t := range selected {
		tagsJSON, _ := json.Marshal(t.Tags)
		dueDate := today
		reminderTime := time.Date(today.Year(), today.Month(), today.Day(), t.DueHour, t.DueMinute, 0, 0, timeutil.LocationVN)

		quest := models.Quest{
			UserID:           userID,
			Title:            t.Title,
			Description:      t.Description,
			Type:             t.Type,
			Status:           models.QuestStatusPending,
			Difficulty:       t.Difficulty,
			Source:           models.QuestSourceDevRandomDailyPlan,
			XPReward:         t.XPReward,
			EstimatedMinutes: t.EstimatedMinutes,
			Reason:           t.Reason,
			Instruction:      t.Instruction,
			Tags:             datatypes.JSON(tagsJSON),
			Date:             today,
			DueDate:          &dueDate,
			ReminderTime:     &reminderTime,
		}

		if err := g.db.Create(&quest).Error; err != nil {
			logger.L.Error("failed to create dev quest", zap.String("title", t.Title), zap.Error(err))
			return created, err
		}
		created = append(created, quest)
	}

	logger.L.Info("dev daily quests generated",
		zap.Int("count", len(created)),
		zap.String("user_id", userID.String()),
	)
	return created, nil
}

func selectDiverseQuests(rng *rand.Rand, templates []devQuestTemplate, total int) []devQuestTemplate {
	wellness := filterByTypes(templates, models.QuestTypeWater, models.QuestTypeBreak, models.QuestTypeMovement)
	learning := filterByTypes(templates, models.QuestTypeLearning)
	reflection := filterByTypes(templates, models.QuestTypeReview, models.QuestTypeReflection)
	sleep := filterByTypes(templates, models.QuestTypeSleep)

	rng.Shuffle(len(wellness), func(i, j int) { wellness[i], wellness[j] = wellness[j], wellness[i] })
	rng.Shuffle(len(learning), func(i, j int) { learning[i], learning[j] = learning[j], learning[i] })
	rng.Shuffle(len(reflection), func(i, j int) { reflection[i], reflection[j] = reflection[j], reflection[i] })
	rng.Shuffle(len(sleep), func(i, j int) { sleep[i], sleep[j] = sleep[j], sleep[i] })

	var selected []devQuestTemplate
	// Use maps to avoid duplicate titles
	usedTitle := make(map[string]bool)

	addUnique := func(source []devQuestTemplate, need int) {
		for i := 0; i < len(source) && need > 0; i++ {
			if usedTitle[source[i].Title] {
				continue
			}
			selected = append(selected, source[i])
			usedTitle[source[i].Title] = true
			need--
		}
	}

	if len(wellness) >= 2 {
		addUnique(wellness, 2)
	}
	if len(learning) >= 1 {
		addUnique(learning, 1)
	}
	if len(reflection) >= 1 {
		addUnique(reflection, 1)
	}

	// Fill remaining slots with shuffled remainder
	allRemaining := shuffleRemaining(rng, templates, usedTitle)
	remaining := total - len(selected)
	for i := 0; i < len(allRemaining) && remaining > 0; i++ {
		selected = append(selected, allRemaining[i])
		usedTitle[allRemaining[i].Title] = true
		remaining--
	}

	// If still not enough (pool exhausted), pad with anything
	if len(selected) < total {
		for i := 0; i < len(templates) && len(selected) < total; i++ {
			if !usedTitle[templates[i].Title] {
				selected = append(selected, templates[i])
				usedTitle[templates[i].Title] = true
			}
		}
	}

	rng.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })
	return selected
}

func filterByTypes(templates []devQuestTemplate, types ...models.QuestType) []devQuestTemplate {
	typeSet := make(map[models.QuestType]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	var result []devQuestTemplate
	for _, t := range templates {
		if typeSet[t.Type] {
			result = append(result, t)
		}
	}
	return result
}

func shuffleRemaining(rng *rand.Rand, templates []devQuestTemplate, used map[string]bool) []devQuestTemplate {
	var result []devQuestTemplate
	for _, t := range templates {
		if !used[t.Title] {
			result = append(result, t)
		}
	}
	rng.Shuffle(len(result), func(i, j int) { result[i], result[j] = result[j], result[i] })
	return result
}
