package protected

import (
	"fmt"
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// PromoEventRequest represents a promo event tracking request
type PromoEventRequest struct {
	PromoID   string                 `json:"promoId"`
	ID        string                 `json:"id"`
	EventType string                 `json:"eventType"`
	Type      string                 `json:"type"`
	Component string                 `json:"component"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TrackPromoEvent tracks promo interaction events for analytics
// @Summary Track promo event
// @Description Track user interactions with promotional content
// @Tags analytics,promo
// @Accept json
// @Produce json
// @Param request body PromoEventRequest true "Promo event data"
// @Success 200 {object} map[string]bool
// @Router /api/analytics/promo [post]
func TrackPromoEvent(c *gin.Context) {
	var req PromoEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	promoID := req.PromoID
	if promoID == "" {
		promoID = req.ID
	}
	if promoID == "" {
		promoID = req.Component
	}
	if promoID == "" {
		promoID = "general"
	}

	eventType := req.EventType
	if eventType == "" {
		eventType = req.Type
	}
	if eventType == "" {
		eventType = "view"
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
		"promoId":   promoID,
		"eventType": eventType,
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
		EventID:    fmt.Sprintf("promo-%s-%s-%d", promoID, eventType, time.Now().UnixNano()),
		EventType:  "promo_" + eventType,
		UserID:     userID,
		Payload:    payload,
		Source:     "frontend",
		ReceivedAt: receivedAt,
	}

	// Fire-and-forget style: log but don't fail the request on DB error
	if err := db.RawWriteDB(c.Request.Context()).Create(&event).Error; err != nil {
		log.Printf("Failed to track promo event: %v", err)
	}

	api_response.Success(c, gin.H{"success": true})
}
