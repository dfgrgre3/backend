package protected

import (
	"fmt"
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminBulkSendMessage delivers an announcement to a set of users through the
// selected channels (in-app notification and/or email).
func AdminBulkSendMessage(c *gin.Context) {
	var req struct {
		Message   string   `json:"message" binding:"required"`
		Title     string   `json:"title"`
		Type      string   `json:"type"`
		UserIDs   []string `json:"userIds"`
		Role      string   `json:"role"`
		ActionURL string   `json:"actionUrl"`
		Channels  []string `json:"channels"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := validateBulkMessageChannels(req.Channels); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	targetUsers, err := fetchBulkMessageTargetUsers(req.UserIDs, req.Role)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	notificationType := parseNotificationType(req.Type)
	title := req.Title
	if title == "" {
		title = "رسالة من الإدارة"
	}

	notifications := prepareBulkNotifications(targetUsers, title, req.Message, notificationType, req.ActionURL, req.Channels)

	if len(notifications) > 0 {
		if err := db.DB.CreateInBatches(&notifications, 100).Error; err != nil {
			log.Printf("ERROR: Failed to create notifications in bulk: %v", err)
			api_response.Error(c, http.StatusInternalServerError, "Failed to create notifications: "+err.Error())
			return
		}
	}

	sendBulkEmailsBackground(targetUsers, title, req.Message, req.ActionURL, req.Channels)

	LogAudit(c, "BULK_SEND_MESSAGE", "notification", "", gin.H{
		"targetCount": len(targetUsers),
		"channels":    req.Channels,
		"appCount":    len(notifications),
		"queued":      true,
	})

	api_response.Success(c, gin.H{
		"summary": gin.H{
			"success": len(notifications),
			"failure": 0,
			"total":   len(targetUsers),
		},
		"message": fmt.Sprintf("تم إرسال %d رسالة بنجاح", len(notifications)),
	})
}

func validateBulkMessageChannels(channels []string) error {
	if len(channels) == 0 {
		return nil
	}
	validChannels := map[string]bool{"app": true, "email": true, "sms": true}
	for _, channel := range channels {
		if !validChannels[channel] {
			return fmt.Errorf("invalid channel: %s", channel)
		}
	}
	return nil
}

func fetchBulkMessageTargetUsers(userIDs []string, role string) ([]models.User, error) {
	var targetUsers []models.User
	query := db.DB.Model(&models.User{})
	if len(userIDs) > 0 {
		query = query.Where(idInQuery, userIDs)
	} else if role != "" {
		query = query.Where(queryRole, role)
	}

	if err := query.Find(&targetUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch target users")
	}
	return targetUsers, nil
}

func parseNotificationType(t string) models.NotificationType {
	switch t {
	case "success":
		return models.NotificationSuccess
	case "warning":
		return models.NotificationWarning
	case "error":
		return models.NotificationError
	default:
		return models.NotificationInfo
	}
}

func prepareBulkNotifications(users []models.User, title, message string, nType models.NotificationType, actionURL string, channels []string) []models.Notification {
	notifications := make([]models.Notification, 0, len(users))
	shouldAddLink := actionURL != "" && (len(channels) == 0 || contains(channels, "app"))

	for _, u := range users {
		notification := models.Notification{
			UserID:  u.ID,
			Title:   title,
			Message: message,
			Type:    nType,
		}
		if shouldAddLink {
			link := actionURL
			notification.Link = &link
		}
		notifications = append(notifications, notification)
	}
	return notifications
}

func sendBulkEmailsBackground(users []models.User, title, message, url string, channels []string) {
	go func() {
		emailService := notificationservice.GetEmailService()
		for _, u := range users {
			if len(channels) == 0 || contains(channels, "email") {
				if emailService.ValidateEmail(u.Email) {
					emailBody := emailService.BuildNotificationEmail(title, message, url)
					_ = emailService.SendEmail(u.Email, title, emailBody, true)
				}
			}
		}
	}()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
