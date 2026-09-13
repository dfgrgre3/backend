package protected

import (
	"context"
	"log"
	"net/http"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const megaMenuEventQueueSize = 256

var (
	megaMenuQueueOnce sync.Once
	megaMenuEvents    chan models.AnalyticsEvent
)

// startMegaMenuEventWriter serializes best-effort telemetry writes. A
// goroutine per click can exhaust the write pool while the endpoint itself is
// intentionally returning immediately.
func startMegaMenuEventWriter() {
	megaMenuQueueOnce.Do(func() {
		megaMenuEvents = make(chan models.AnalyticsEvent, megaMenuEventQueueSize)
		go func() {
			for event := range megaMenuEvents {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if writer := db.RawWriteDB(ctx); writer != nil {
					if err := writer.Create(&event).Error; err != nil {
						log.Printf("Failed to track mega menu event: %v", err)
					}
				}
				cancel()
			}
		}()
	})
}

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

	// Respond immediately; this endpoint is best-effort telemetry and must
	// never block the caller (navigator.sendBeacon/fetch keepalive) on DB latency.
	api_response.Success(c, gin.H{"success": true})

	// Telemetry is best effort. Queue it after responding so a slow database
	// never delays navigation and a burst of clicks cannot create one writer
	// goroutine per request.
	startMegaMenuEventWriter()
	select {
	case megaMenuEvents <- event:
	default:
		// Dropping telemetry is preferable to allowing analytics to affect the
		// application's database capacity.
	}
}
