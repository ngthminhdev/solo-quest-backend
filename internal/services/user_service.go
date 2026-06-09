package services

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"solo_quest_backend/internal/models"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetDB() *gorm.DB {
	return s.db
}

func (s *UserService) GetUserByID(userID uuid.UUID) (*models.UserProfile, error) {
	var user models.UserProfile
	err := s.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// TODO: replace dev user with authenticated user after auth is implemented.
func (s *UserService) GetDevUser() (*models.UserProfile, error) {
	var user models.UserProfile
	devUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	err := s.db.Where("id = ?", devUserID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
