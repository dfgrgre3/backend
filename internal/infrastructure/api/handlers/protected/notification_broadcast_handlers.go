package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// SendNotificationBroadcast sends notifications to multiple users via various channels
// @Summary Send broadcast notification
// @Description Send notifications to multiple users via in-app, email, SMS, or push
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param request body NotificationRequest true "Notification details"
// @Success 200 {object} NotificationResponse
// @Router /api/admin/notifications/broadcast [post]
func SendNotificationBroadcast(c *gin.Context) {
	var req NotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminID, ok := contextString(c, "userId", "user_id")
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	adminRole, _ := contextString(c, "role", "user_role")

	// Log critical operation
	middleware.LogCriticalOperation(c, "notification_broadcast", map[string]interface{}{
		"target_users": len(req.UserIDs),
		"channels":     req.Channels,
		"priority":     req.Priority,
		"scheduled":    req.ScheduledFor != nil,
	})

	// Create broadcast record
	broadcast := models.Broadcast{
		Title:       req.Title,
		Message:     req.Message,
		Type:        req.Type,
		Channels:    req.Channels,
		TargetCount: len(req.UserIDs),
		Status:      "sending",
		CreatedBy:   adminID,
		CreatedAt:   time.Now(),
	}

	if req.ScheduledFor != nil {
		broadcast.Status = "scheduled"
		broadcast.ScheduledFor = req.ScheduledFor
	}

	if err := SafeCreate(db.DB, &broadcast); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to create broadcast", err)
		return
	}

	// Queue notifications for each user
	notificationService := notificationservice.GetNotificationService()
	successCount := 0
	failureCount := 0

	for _, userID := range req.UserIDs {
		err := notificationService.QueueNotification(models.Notification{
			UserID:      userID,
			BroadcastID: broadcast.ID,
			Title:       req.Title,
			Message:     req.Message,
			Type:        models.NotificationType(strings.ToUpper(req.Type)),
			Channels:    req.Channels,
			Status:      "pending",
			Priority:    req.Priority,
			Link:        &req.ActionURL,
			CreatedAt:   time.Now(),
		})

		if err != nil {
			failureCount++
		} else {
			successCount++
		}
	}

	// Update broadcast status
	if req.ScheduledFor == nil {
		broadcast.Status = "completed"
		now := time.Now()
		broadcast.SentAt = &now
	}
	broadcast.SuccessCount = successCount
	broadcast.FailureCount = failureCount
	db.DB.Save(&broadcast)

	// Notify admins via WebSocket
	GlobalHub.NotifyAdmins(map[string]interface{}{
		"type":        "broadcast-completed",
		"broadcastId": broadcast.ID,
		"success":     successCount,
		"failed":      failureCount,
		"total":       len(req.UserIDs),
		"adminId":     adminID,
		"adminRole":   adminRole,
	})

	api_response.Success(c, NotificationResponse{
		BroadcastID: broadcast.ID,
		Summary: NotificationSummary{
			Total:   len(req.UserIDs),
			Success: successCount,
			Failure: failureCount,
			Queued:  successCount,
		},
		Queued: true,
	})
}

// ScheduleNotificationBroadcast schedules a notification for future delivery
// @Summary Schedule broadcast notification
// @Description Schedule notifications to be sent at a specific time
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param request body NotificationRequest true "Notification details with scheduledFor"
// @Success 200 {object} NotificationResponse
// @Router /api/admin/notifications/schedule [post]
func ScheduleNotificationBroadcast(c *gin.Context) {
	var req NotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.ScheduledFor == nil {
		api_response.Error(c, http.StatusBadRequest, "scheduledFor is required")
		return
	}

	// Ensure scheduled time is in the future
	if req.ScheduledFor.Before(time.Now()) {
		api_response.Error(c, http.StatusBadRequest, "scheduledFor must be in the future")
		return
	}

	// Call the main broadcast function (it will handle the scheduled flag)
	SendNotificationBroadcast(c)
}

// CancelScheduledBroadcast cancels a scheduled broadcast
// @Summary Cancel scheduled broadcast
// @Description Cancel a notification that was scheduled for future delivery
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param broadcastId path string true "Broadcast ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/notifications/broadcast/{broadcastId}/cancel [post]
func CancelScheduledBroadcast(c *gin.Context) {
	broadcastID := c.Param("broadcastId")
	if broadcastID == "" {
		broadcastID = c.Param("id")
	}
	adminID, ok := contextString(c, "userId", "user_id")
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var broadcast models.Broadcast
	if err := db.DB.First(&broadcast, "id = ?", broadcastID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Broadcast not found")
		return
	}

	// Check if broadcast is scheduled
	if broadcast.Status != "scheduled" {
		api_response.Error(c, http.StatusBadRequest, "Can only cancel scheduled broadcasts")
		return
	}

	// Update status
	broadcast.Status = "cancelled"
	broadcast.CancelledBy = ptrString(adminID)
	now := time.Now()
	broadcast.CancelledAt = &now

	if err := db.DB.Save(&broadcast).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to cancel broadcast")
		return
	}

	// Cancel any pending notifications
	db.DB.Model(&models.Notification{}).
		Where("broadcast_id = ? AND status = ?", broadcastID, "pending").
		Update("status", "cancelled")

	api_response.Success(c, gin.H{"message": "Broadcast cancelled successfully"})
}

// RetryFailedNotifications retries failed notifications from a broadcast
// @Summary Retry failed notifications
// @Description Retry sending notifications that previously failed
// @Tags admin,notifications
// @Accept json
// @Produce json
// @Param broadcastId path string true "Broadcast ID"
// @Success 200 {object} NotificationResponse
// @Router /api/admin/notifications/broadcast/{broadcastId}/retry [post]
func RetryFailedNotifications(c *gin.Context) {
	broadcastID := c.Param("broadcastId")

	var notifications []models.Notification
	if err := db.DB.Where("broadcast_id = ? AND status = ?", broadcastID, "failed").Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch failed notifications")
		return
	}

	notificationService := notificationservice.GetNotificationService()
	successCount := 0
	failureCount := 0

	for _, notification := range notifications {
		notification.Status = "pending"
		db.DB.Save(&notification)

		err := notificationService.QueueNotification(notification)
		if err != nil {
			failureCount++
		} else {
			successCount++
		}
	}

	api_response.Success(c, NotificationResponse{
		BroadcastID: broadcastID,
		Summary: NotificationSummary{
			Total:   len(notifications),
			Success: successCount,
			Failure: failureCount,
		},
		Queued: true,
	})
}
