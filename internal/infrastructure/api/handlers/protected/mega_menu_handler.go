package protected

import (
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MegaMenuTrackRequest represents the payload sent by the MegaMenu component
// via navigator.sendBeacon or fetch keepalive.
type MegaMenuTrackRequest struct {
	Type      string                 `json:"type" binding:"required"` // "open" | "close"
	Component string                 `json:"component"`               // "mega_menu"
	Label     string                 `json:"label"`                   // menu label
	Timestamp int64                  `json:"timestamp"`               // client epoch ms
	Metadata  map[string]interface{} `json:"metadata,omitempty"`      // e.g. { trigger: "toggle" }
}

// TrackMegaMenuEvent handles tracking for MegaMenu open/close events.
// @Summary Track mega menu event
// @Description Track open/close interactions on the mega menu component
// @Tags analytics,mega-menu
// @Accept json
// @Produce json
// @Param request body MegaMenuTrackRequest true "Mega menu event data"
// @Success 200 {object} map[string]bool
// @Router /api/analytics/mega-menu [post]
func TrackMegaMenuEvent(c *gin.Context) {
	var req MegaMenuTrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate event type
	if req.Type != "open" && req.Type != "close" {
		api_response.Error(c, http.StatusBadRequest, "type must be 'open' or 'close'")
		return
	}

	// Get user ID if authenticated (optional — endpoint is public)
	var userID *string
	if uid, exists := c.Get("userId"); exists && uid != nil {
		if uidStr, ok := uid.(string); ok {
			userID = &uidStr
		}
	}

	// Build payload
	payload := models.JSONMap{
		"type":      req.Type,
		"component": req.Component,
		"label":     req.Label,
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

	// Use client timestamp if provided, otherwise server time
	receivedAt := time.Now()
	if req.Timestamp > 0 {
		// Convert client epoch ms to time
		clientTime := time.UnixMilli(req.Timestamp)
		// Sanity check: reject timestamps more than 1 hour in the future
		if clientTime.Before(receivedAt.Add(time.Hour)) {
			receivedAt = clientTime
		}
	}

	event := models.AnalyticsEvent{
		EventID:    "mega-menu-" + uuid.NewString(),
		EventType:  "mega_menu_" + req.Type,
		UserID:     userID,
		Payload:    payload,
		Source:     "frontend",
		ReceivedAt: receivedAt,
	}

	// Fire-and-forget style: log but don't fail the request on DB error
	if err := db.RawWriteDB(c.Request.Context()).Create(&event).Error; err != nil {
		log.Printf("Failed to track mega menu event: %v", err)
		// Still return success to avoid breaking frontend UX
	}

	api_response.Success(c, gin.H{"success": true})
}
