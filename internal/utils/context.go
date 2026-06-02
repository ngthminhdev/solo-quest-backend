package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CurrentUserIDKey = "currentUserID"

func GetCurrentUserID(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get(CurrentUserIDKey)
	if !exists {
		return uuid.Nil, errors.New("user not found in context")
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("invalid user ID in context")
	}

	return uid, nil
}

func SetCurrentUserID(c *gin.Context, userID uuid.UUID) {
	c.Set(CurrentUserIDKey, userID)
}
