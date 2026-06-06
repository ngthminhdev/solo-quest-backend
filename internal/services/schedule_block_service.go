package services

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
)

var (
	ErrInvalidScheduleBlockType   = errors.New("invalid schedule block type")
	ErrInvalidDaysOfWeek          = errors.New("days_of_week must not be empty")
	ErrInvalidDayValue            = errors.New("days_of_week must contain values 1-7")
	ErrInvalidTimeRange           = errors.New("start_time must be before end_time")
	ErrScheduleBlockNotFound      = errors.New("schedule block not found")
	ErrScheduleBlockNotOwned      = errors.New("schedule block does not belong to user")
)

type ScheduleBlockService struct {
	db *gorm.DB
}

func NewScheduleBlockService(db *gorm.DB) *ScheduleBlockService {
	return &ScheduleBlockService{db: db}
}

func (s *ScheduleBlockService) ListBlocks(userID uuid.UUID) ([]dto.ScheduleBlockResponse, error) {
	var blocks []models.ScheduleBlock
	if err := s.db.Where("user_id = ? AND enabled = ?", userID, true).Order("start_time ASC").Find(&blocks).Error; err != nil {
		return nil, err
	}

	responses := make([]dto.ScheduleBlockResponse, len(blocks))
	for i, block := range blocks {
		responses[i] = toScheduleBlockResponse(block)
	}
	return responses, nil
}

func (s *ScheduleBlockService) CreateBlock(userID uuid.UUID, req *dto.CreateScheduleBlockRequest) (*dto.ScheduleBlockResponse, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	daysJSON, err := daysOfWeekToJSON(req.DaysOfWeek)
	if err != nil {
		return nil, err
	}

	block := models.ScheduleBlock{
		UserID:     userID,
		Title:      req.Title,
		Type:       models.ScheduleBlockType(req.Type),
		DaysOfWeek: daysJSON,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		IsBusy:     req.IsBusy,
		IsFlexible: req.IsFlexible,
		Enabled:    true,
		Location:   strToPtr(req.Location),
		Note:       strToPtr(req.Note),
	}

	if err := s.db.Create(&block).Error; err != nil {
		return nil, err
	}

	resp := toScheduleBlockResponse(block)
	return &resp, nil
}

func (s *ScheduleBlockService) GetBlock(userID, blockID uuid.UUID) (*models.ScheduleBlock, error) {
	var block models.ScheduleBlock
	if err := s.db.Where("id = ? AND user_id = ?", blockID, userID).First(&block).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScheduleBlockNotFound
		}
		return nil, err
	}
	return &block, nil
}

func (s *ScheduleBlockService) UpdateBlock(userID, blockID uuid.UUID, req *dto.CreateScheduleBlockRequest) (*dto.ScheduleBlockResponse, error) {
	block, err := s.GetBlock(userID, blockID)
	if err != nil {
		return nil, err
	}

	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	daysJSON, err := daysOfWeekToJSON(req.DaysOfWeek)
	if err != nil {
		return nil, err
	}

	block.Title = req.Title
	block.Type = models.ScheduleBlockType(req.Type)
	block.DaysOfWeek = daysJSON
	block.StartTime = req.StartTime
	block.EndTime = req.EndTime
	block.IsBusy = req.IsBusy
	block.IsFlexible = req.IsFlexible
	block.Location = strToPtr(req.Location)
	block.Note = strToPtr(req.Note)

	if err := s.db.Save(block).Error; err != nil {
		return nil, err
	}

	resp := toScheduleBlockResponse(*block)
	return &resp, nil
}

func (s *ScheduleBlockService) PatchBlock(userID, blockID uuid.UUID, req *dto.UpdateScheduleBlockRequest) (*dto.ScheduleBlockResponse, error) {
	block, err := s.GetBlock(userID, blockID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		block.Title = *req.Title
	}
	if req.Type != nil {
		if !models.IsValidScheduleBlockType(*req.Type) {
			return nil, ErrInvalidScheduleBlockType
		}
		block.Type = models.ScheduleBlockType(*req.Type)
	}
	if req.DaysOfWeek != nil {
		if err := validateDaysOfWeek(*req.DaysOfWeek); err != nil {
			return nil, err
		}
		daysJSON, err := daysOfWeekToJSON(*req.DaysOfWeek)
		if err != nil {
			return nil, err
		}
		block.DaysOfWeek = daysJSON
	}
	if req.StartTime != nil {
		block.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		block.EndTime = *req.EndTime
	}
	if block.StartTime != "" && block.EndTime != "" && block.StartTime == block.EndTime {
		return nil, ErrInvalidTimeRange
	}
	if req.IsBusy != nil {
		block.IsBusy = *req.IsBusy
	}
	if req.IsFlexible != nil {
		block.IsFlexible = *req.IsFlexible
	}
	if req.Enabled != nil {
		block.Enabled = *req.Enabled
	}
	if req.Location != nil {
		block.Location = req.Location
	}
	if req.Note != nil {
		block.Note = req.Note
	}

	if err := s.db.Save(block).Error; err != nil {
		return nil, err
	}

	resp := toScheduleBlockResponse(*block)
	return &resp, nil
}

func (s *ScheduleBlockService) DeleteBlock(userID, blockID uuid.UUID) error {
	block, err := s.GetBlock(userID, blockID)
	if err != nil {
		return err
	}

	block.Enabled = false
	return s.db.Save(block).Error
}

func validateCreateRequest(req *dto.CreateScheduleBlockRequest) error {
	if !models.IsValidScheduleBlockType(req.Type) {
		return ErrInvalidScheduleBlockType
	}
	if err := validateDaysOfWeek(req.DaysOfWeek); err != nil {
		return err
	}
	if req.StartTime == req.EndTime {
		return ErrInvalidTimeRange
	}
	return nil
}

func validateDaysOfWeek(days []int) error {
	if len(days) == 0 {
		return ErrInvalidDaysOfWeek
	}
	for _, d := range days {
		if d < 1 || d > 7 {
			return ErrInvalidDayValue
		}
	}
	return nil
}

func daysOfWeekToJSON(days []int) (datatypes.JSON, error) {
	bytes, err := json.Marshal(days)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(bytes), nil
}

func toScheduleBlockResponse(b models.ScheduleBlock) dto.ScheduleBlockResponse {
	var days []int
	if len(b.DaysOfWeek) > 0 {
		json.Unmarshal(b.DaysOfWeek, &days)
	}

	response := dto.ScheduleBlockResponse{
		ID:         b.ID,
		Title:      b.Title,
		Type:       string(b.Type),
		DaysOfWeek: days,
		StartTime:  b.StartTime,
		EndTime:    b.EndTime,
		IsBusy:     b.IsBusy,
		IsFlexible: b.IsFlexible,
		Enabled:    b.Enabled,
		Location:   b.Location,
		Note:       b.Note,
		CreatedAt:  b.CreatedAt,
		UpdatedAt:  b.UpdatedAt,
	}
	if response.DaysOfWeek == nil {
		response.DaysOfWeek = []int{}
	}
	return response
}

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
