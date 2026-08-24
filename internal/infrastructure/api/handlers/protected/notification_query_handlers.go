package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetBroadcasts returns all broadcasts with filtering
// @Summary Get broadcasts
// @Description Get all notification broadcasts with optional filtering
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param from query string false "Filter from date (RFC3339)"
// @Param to query string false "Filter to date (RFC3339)"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/broadcasts [get]
func GetBroadcasts(c *gin.Context) {
	status := c.Query("status")
	from := c.Query("from")
	to := c.Query("to")

	query := db.DB.Model(&models.Broadcast{}).Order("created_at DESC")

	if status != "" {
		query = query.Where(statusQuery, status)
	}

	if from != "" {
		if fromTime, err := time.Parse(time.RFC3339, from); err == nil {
			query = query.Where("created_at >= ?", fromTime)
		}
	}

	if to != "" {
		if toTime, err := time.Parse(time.RFC3339, to); err == nil {
			query = query.Where("created_at <= ?", toTime)
		}
	}

	var broadcasts []models.Broadcast
	if err := query.Find(&broadcasts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch broadcasts")
		return
	}

	api_response.Success(c, gin.H{
		"broadcasts": broadcasts,
	})
}

// GetNotificationStats returns statistics about notifications
// @Summary Get notification statistics
// @Description Get aggregated statistics about notification delivery
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/notifications/stats [get]
func GetNotificationStats(c *gin.Context) {
	var stats struct {
		TotalSent      int64 `json:"totalSent"`
		TotalDelivered int64 `json:"totalDelivered"`
		TotalFailed    int64 `json:"totalFailed"`
		TotalRead      int64 `json:"totalRead"`
		Pending        int64 `json:"pending"`
	}

	// Get counts by status
	db.DB.Model(&models.Notification{}).Count(&stats.TotalSent)
	db.DB.Model(&models.Notification{}).Where(statusQuery, "delivered").Count(&stats.TotalDelivered)
	db.DB.Model(&models.Notification{}).Where(statusQuery, "failed").Count(&stats.TotalFailed)
	db.DB.Model(&models.Notification{}).Where(statusQuery, "read").Count(&stats.TotalRead)
	db.DB.Model(&models.Notification{}).Where(statusQuery, "pending").Count(&stats.Pending)

	// Get channel breakdown
	var channelStats []struct {
		Channel string `json:"channel"`
		Count   int64  `json:"count"`
	}
	db.DB.Table(models.Notification{}.TableName()).Select("jsonb_array_elements_text(COALESCE(channels, '[]'::jsonb)) as channel, count(*) as count").Group("jsonb_array_elements_text(COALESCE(channels, '[]'::jsonb))").Scan(&channelStats)

	// Get recent broadcasts
	var recentBroadcasts []models.Broadcast
	db.DB.Order("created_at DESC").Limit(5).Find(&recentBroadcasts)

	api_response.Success(c, gin.H{
		"overview":         stats,
		"byChannel":        channelStats,
		"recentBroadcasts": recentBroadcasts,
	})
}

// SendPushNotification sends push notifications to specific users
// @Summary Send push notification
// @Description Send push notifications to specific users via FCM/APNs
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Push notification details"
// @Success 200 {object} map[string]string
// @Router /api/admin/notifications/push [post]
func SendPushNotification(c *gin.Context) {
	var req struct {
		UserIDs []string               `json:"userIds" binding:"required,min=1"`
		Title   string                 `json:"title" binding:"required"`
		Body    string                 `json:"body" binding:"required"`
		Data    map[string]interface{} `json:"data,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get push service
	pushService := notificationservice.GetPushNotificationService()

	sent := 0
	failed := 0

	for _, userID := range req.UserIDs {
		// Get user's push tokens
		var tokens []models.PushToken
		db.DB.Where("user_id = ? AND "+isActiveQuery, userID, true).Find(&tokens)

		for _, token := range tokens {
			err := pushService.Send(token.Token, req.Title, req.Body, req.Data)
			if err != nil {
				failed++
				// Mark token as potentially invalid
				if pushService.IsInvalidTokenError(err) {
					token.IsActive = false
					db.DB.Save(&token)
				}
			} else {
				sent++
			}
		}
	}

	api_response.Success(c, gin.H{
		"message": "Push notifications processed",
		"sent":    sent,
		"failed":  failed,
	})
}

func ptrString(s string) *string {
	return &s
}

func contextString(c *gin.Context, keys ...string) (string, bool) {
	for _, key := range keys {
		if value := c.GetString(key); value != "" {
			return value, true
		}
	}
	return "", false
}
