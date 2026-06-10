package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/dto"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/pkg/timeutil"
)

type LogService struct {
	db *gorm.DB
}

func NewLogService(db *gorm.DB) *LogService {
	return &LogService{db: db}
}

func (s *LogService) GetLogsByUserID(userID uuid.UUID, limit int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	if limit <= 0 {
		limit = 50
	}
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetLogs retrieves filtered logs for a user using VN timezone for date boundaries
func (s *LogService) GetLogs(userID uuid.UUID, filter dto.LogFilter) (*dto.LogListResponse, error) {
	query := s.db.Model(&models.LogEntry{}).Where("user_id = ?", userID)

	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if filter.QuestType != "" {
		query = query.Where("quest_type = ?", filter.QuestType)
	}

	// Date filtering using VN timezone (Asia/Ho_Chi_Minh)
	if filter.Date != "" {
		dateStart, err := timeutil.ParseDateVN(filter.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format: %w", err)
		}
		start, end := timeutil.DayRangeVN(dateStart)
		query = query.Where("created_at >= ? AND created_at < ?", start, end)
	}

	if filter.From != "" {
		fromDate, err := timeutil.ParseDateVN(filter.From)
		if err != nil {
			return nil, fmt.Errorf("invalid from date format: %w", err)
		}
		query = query.Where("created_at >= ?", fromDate)
	}

	if filter.To != "" {
		toDate, err := timeutil.ParseDateVN(filter.To)
		if err != nil {
			return nil, fmt.Errorf("invalid to date format: %w", err)
		}
		toEnd := timeutil.EndExclusiveOfDayVN(toDate)
		query = query.Where("created_at < ?", toEnd)
	}

	var total int64
	query.Count(&total)

	var entries []models.LogEntry
	err := query.Order("created_at DESC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	items := make([]dto.LogItem, len(entries))
	for i, entry := range entries {
		var questType *string
		if entry.QuestType != nil {
			s := string(*entry.QuestType)
			questType = &s
		}
		items[i] = dto.LogItem{
			ID:            entry.ID,
			Type:          string(entry.Type),
			Title:         entry.Title,
			Description:   entry.Content,
			QuestID:       entry.QuestID,
			QuestType:     questType,
			ExpChanged:    entry.ExpChanged,
			PointsChanged: entry.PointsChanged,
			CreatedAt:     entry.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	return &dto.LogListResponse{
		Items:      items,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
		TotalCount: total,
	}, nil
}
