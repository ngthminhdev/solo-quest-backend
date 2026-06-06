package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"solo_quest_backend/internal/pkg/response"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/utils"
)

type OnboardingHandler struct {
	onboardingService *services.OnboardingService
}

func NewOnboardingHandler(onboardingService *services.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{onboardingService: onboardingService}
}

func (h *OnboardingHandler) SaveOnboarding(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	var req services.OnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.BadRequest("invalid request: "+err.Error()))
		return
	}

	user, onboarding, err := h.onboardingService.SaveOnboarding(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to save onboarding"))
		return
	}

	c.JSON(http.StatusOK, response.SuccessWithMessage(gin.H{
		"profile":    user,
		"onboarding": onboarding,
	}, "onboarding saved successfully"))
}

func (h *OnboardingHandler) GetOnboardingStatus(c *gin.Context) {
	userID, err := utils.GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Unauthorized())
		return
	}

	hasCompleted, profileName, err := h.onboardingService.GetOnboardingStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.InternalError("failed to get onboarding status"))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"has_completed_onboarding": hasCompleted,
		"user_id":                  userID.String(),
		"profile_name":             profileName,
	}))
}
