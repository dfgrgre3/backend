package handlers

import (
	"net/http"
	"strings"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

func GetPublicAnnouncements(c *gin.Context) {
	var notifications []models.Notification
	if err := db.DB.Order("created_at DESC").Limit(50).Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch announcements")
		return
	}

	items := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, gin.H{
			"id":          n.ID,
			"title":       n.Title,
			"content":     n.Message,
			"publishedAt": n.CreatedAt,
			"priority":    strings.ToLower(defaultString(n.Priority, "medium")),
			"category":    strings.ToLower(defaultString(n.Category, "general")),
			"authorName":  "System",
			"tags":        []string{},
			"views":       0,
		})
	}

	api_response.Success(c, items)
}

func CreatePublicAnnouncement(c *gin.Context) {
	userID := c.GetString("userId")
	var input struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Priority string   `json:"priority"`
		Category string   `json:"category"`
		Tags     []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" || input.Content == "" {
		api_response.Error(c, http.StatusBadRequest, "title and content are required")
		return
	}

	notification := models.Notification{
		UserID:   userID,
		Title:    input.Title,
		Message:  input.Content,
		Type:     models.NotificationInfo,
		Category: strings.ToUpper(defaultString(input.Category, "general")),
		Priority: strings.ToUpper(defaultString(input.Priority, "medium")),
		IsRead:   false,
	}
	if err := SafeCreate(db.DB, &notification); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create announcement")
		return
	}

	api_response.Success(c, gin.H{"id": notification.ID})
}

func GetChatConversations(c *gin.Context) {
	api_response.Success(c, []gin.H{})
}

func GetChatMessages(c *gin.Context) {
	api_response.Success(c, []gin.H{})
}

func SendChatMessage(c *gin.Context) {
	api_response.Success(c, gin.H{"success": true})
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
