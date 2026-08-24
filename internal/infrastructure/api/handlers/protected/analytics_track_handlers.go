package protected

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"thanawy-backend/internal/infrastructure/api/middleware"
)

// TrackUserJourney saves a complete user journey for analysis
// @Summary Track user journey
// @Description Save a complete user journey session for analytics
// @Tags admin,analytics
// @Accept json
// @Produce json
// @Param request body UserJourneyRequest true "User journey data"
// @Success 201 {object} map[string]string
// @Router /api/admin/analytics/journey [post]
func TrackUserJourney(c *gin.Context) {
	var req UserJourneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Save journey to database
	journey := models.UserJourney{
		UserID:         req.UserID,
		SessionID:      req.SessionID,
		StartedAt:      req.StartedAt,
		EndedAt:        req.EndedAt,
		TotalDuration:  req.TotalDuration,
		ConversionGoal: req.ConversionGoal,
		Completed:      req.Completed,
		Steps:          make([]models.UserJourneyStep, len(req.Steps)),
	}

	for i, step := range req.Steps {
		journey.Steps[i] = models.UserJourneyStep{
			ID:        step.ID,
			UserID:    step.UserID,
			SessionID: step.SessionID,
			Page:      step.Page,
			Action:    step.Action,
			Metadata:  step.Metadata,
			Timestamp: step.Timestamp,
			Duration:  step.Duration,
		}
	}

	if err := db.WriteDB().Create(&journey).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save journey")
		return
	}

	api_response.Success(c, gin.H{
		"message":   "Journey tracked successfully",
		"journeyId": journey.ID,
	})
}

// TrackConversionEvent tracks a conversion event
// @Summary Track conversion event
// @Description Track when a user completes a conversion goal
// @Tags admin,analytics
// @Accept json
// @Produce json
// @Param request body ConversionEventRequest true "Conversion event data"
// @Success 201 {object} map[string]string
// @Router /api/admin/analytics/conversion [post]
func TrackConversionEvent(c *gin.Context) {
	var req ConversionEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Save conversion event
	conversion := models.ConversionEvent{
		UserID:       req.UserID,
		SessionID:    req.SessionID,
		Goal:         req.Goal,
		Value:        req.Value,
		Timestamp:    req.Timestamp,
		JourneySteps: req.JourneySteps,
	}

	if err := db.WriteDB().Create(&conversion).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save conversion")
		return
	}

	// Log critical conversion for admin tracking
	middleware.LogCriticalOperation(c, "conversion_achieved", map[string]interface{}{
		"user_id":       req.UserID,
		"goal":          req.Goal,
		"value":         req.Value,
		"journey_steps": req.JourneySteps,
	})

	api_response.Success(c, gin.H{
		"message":      "Conversion tracked successfully",
		"conversionId": conversion.ID,
	})
}
