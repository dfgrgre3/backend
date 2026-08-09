package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	marketingservice "thanawy-backend/internal/domain/marketing/service"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

var abTestingService = marketingservice.NewABTestingService()

// queryID is defined in common.go: queryID = "id = ?"

func AdminGetABTests(c *gin.Context) {
	experiments, err := abTestingService.ListExperiments(c.Request.Context())
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch AB tests")
		return
	}
	api_response.Success(c, gin.H{
		"experiments": experiments,
	})
}

func AdminCreateABTest(c *gin.Context) {
	var item models.ABExperiment
	if err := c.ShouldBindJSON(&item); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	created, err := abTestingService.CreateExperiment(c.Request.Context(), item.Name, item.Description, item.Status, item.Variants, item.TrafficPct)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create AB test")
		return
	}

	api_response.Created(c, gin.H{
		"experiment": created,
	})
}

func AdminUpdateABTest(c *gin.Context) {
	id := c.Param("id")
	experiment, err := abTestingService.GetExperiment(c.Request.Context(), id)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "AB Test not found")
		return
	}

	var input struct {
		Name         *string  `json:"name"`
		Description  *string  `json:"description"`
		Status       *string  `json:"status"`
		TrafficSplit *float64 `json:"trafficSplit"`
		TrafficPct   *int     `json:"trafficPct"`
		Winner       *string  `json:"winner"`
		Variants     *string  `json:"variants"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if input.Name != nil {
		experiment.Name = *input.Name
	}
	if input.Description != nil {
		experiment.Description = *input.Description
	}
	if input.Status != nil {
		experiment.Status = *input.Status
	}
	if input.TrafficPct != nil {
		experiment.TrafficPct = *input.TrafficPct
	} else if input.TrafficSplit != nil {
		experiment.TrafficPct = int(*input.TrafficSplit)
	}
	if input.Winner != nil {
		experiment.Winner = input.Winner
	}
	if input.Variants != nil {
		experiment.Variants = *input.Variants
	}

	if err := abTestingService.UpdateExperiment(c.Request.Context(), experiment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update AB test")
		return
	}

	api_response.Success(c, gin.H{
		"experiment": experiment,
	})
}

func AdminDeleteABTest(c *gin.Context) {
	id := c.Param("id")
	if err := abTestingService.DeleteExperiment(c.Request.Context(), id); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete AB test")
		return
	}
	api_response.Success(c, nil)
}

// AdminGetABVariant resolves a stable server-side assignment for an existing experiment.
func AdminGetABVariant(c *gin.Context) {
	// Never trust a browser-supplied userId for allocation. The authenticated
	// identity is populated by Auth() and is the only identity eligible here.
	userID := c.GetString("userId")
	if userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "authenticated user is required")
		return
	}

	variant, err := abTestingService.ResolveVariant(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	api_response.Success(c, gin.H{"variant": variant})
}

// AdminTrackABEvent tracks an event for a specific experiment variant
func AdminTrackABEvent(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		UserID string `json:"userId" binding:"required"`
		Event  string `json:"event" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	authenticatedUserID := c.GetString("userId")
	if authenticatedUserID == "" {
		api_response.Error(c, http.StatusUnauthorized, "authenticated user is required")
		return
	}
	if input.UserID != "" && input.UserID != authenticatedUserID {
		api_response.Error(c, http.StatusForbidden, "userId must match the authenticated user")
		return
	}

	if err := abTestingService.TrackEvent(c.Request.Context(), id, authenticatedUserID, input.Event); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	api_response.Success(c, gin.H{
		"tracked": true,
		"event":   input.Event,
	})
}
