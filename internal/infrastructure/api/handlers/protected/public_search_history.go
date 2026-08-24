package protected

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// SearchHistoryRequest represents a search history tracking request
type SearchHistoryRequest struct {
	Query     string                 `json:"query" binding:"required"`
	Results   int                    `json:"results"`
	Type      string                 `json:"type"` // course, resource, teacher, video, all
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TrackSearchHistory tracks user search queries for analytics
// @Summary Track search history
// @Description Track user search queries for analytics and personalization
// @Tags search,analytics
// @Accept json
// @Produce json
// @Param request body SearchHistoryRequest true "Search history data"
// @Success 200 {object} map[string]bool
// @Router /api/ai/search/track [post]
func TrackSearchHistory(c *gin.Context) {
	var req SearchHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get user ID if authenticated (optional)
	var userID *string
	if uid, exists := c.Get("userId"); exists && uid != nil {
		if uidStr, ok := uid.(string); ok {
			userID = &uidStr
		}
	}

	// Build payload
	payload := models.JSONMap{
		"query":   req.Query,
		"results": req.Results,
		"type":    req.Type,
	}
	if req.Metadata != nil {
		for k, v := range req.Metadata {
			payload[k] = v
		}
	}

	// Capture request metadata for analytics
	if ip := c.ClientIP(); ip != "" {
		payload["ipAddress"] = ip
	}
	if ua := c.GetHeader("User-Agent"); ua != "" {
		payload["userAgent"] = ua
	}

	// Use client timestamp if provided and plausible, otherwise server time.
	// Bounded to [now-24h, now+1h] so a bad/malicious client can't backdate
	// or far-future-date analytics rows arbitrarily.
	receivedAt := time.Now()
	if req.Timestamp > 0 {
		clientTime := time.UnixMilli(req.Timestamp)
		if clientTime.After(receivedAt.Add(-24*time.Hour)) && clientTime.Before(receivedAt.Add(time.Hour)) {
			receivedAt = clientTime
		}
	}

	event := models.AnalyticsEvent{
		EventID:    fmt.Sprintf("search-%s-%d", req.Query, time.Now().UnixNano()),
		EventType:  "search_query",
		UserID:     userID,
		Payload:    payload,
		Source:     "frontend",
		ReceivedAt: receivedAt,
	}

	// Fire-and-forget style: log but don't fail the request on DB error
	if err := db.RawWriteDB(c.Request.Context()).Create(&event).Error; err != nil {
		log.Printf("Failed to track search history: %v", err)
	}

	api_response.Success(c, gin.H{"success": true})
}

// GetUserSearchHistory returns the authenticated user's search history
// @Summary Get user search history
// @Description Get the authenticated user's recent search queries
// @Tags search,analytics
// @Produce json
// @Param limit query int false "Max results (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /api/ai/search/history [get]
func GetUserSearchHistory(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	var events []models.AnalyticsEvent
	query := db.RawReadDB(c.Request.Context()).Where("event_type = ? AND user_id = ?", "search_query", userID).
		Order("received_at DESC").
		Limit(limit)

	if err := query.Find(&events).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch search history")
		return
	}

	// Extract relevant data from events
	history := make([]gin.H, 0, len(events))
	for _, event := range events {
		history = append(history, gin.H{
			"query":     event.Payload["query"],
			"results":   event.Payload["results"],
			"type":      event.Payload["type"],
			"timestamp": event.ReceivedAt,
		})
	}

	api_response.Success(c, gin.H{
		"history": history,
		"count":   len(history),
	})
}
