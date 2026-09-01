package protected

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminAnnouncements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Notification{})
	if search := c.Query("search"); search != "" {
		like := "%" + search + "%"
		query = query.Where("title ILIKE ? OR message ILIKE ?", like, like)
	}

	// Type filter: comma-separated (e.g. "INFO,SUCCESS,WARNING,ERROR").
	if typeFilter := c.Query("type"); typeFilter != "" {
		if types := splitCommaQuery(typeFilter); len(types) > 0 {
			query = query.Where("type IN ?", types)
		}
	}

	// Priority filter: comma-separated (e.g. "LOW,MEDIUM,HIGH").
	if priorityFilter := c.Query("priority"); priorityFilter != "" {
		if priorities := splitCommaQuery(priorityFilter); len(priorities) > 0 {
			query = query.Where("priority IN ?", priorities)
		}
	}

	// Status filter: "active" | "inactive".
	if status := c.Query("status"); status != "" {
		query = query.Where("is_active = ?", status == "active")
	}

	var total int64
	query.Count(&total)

	var notifications []models.Notification
	if err := query.Order(buildAnnouncementOrder(c.Query("sortBy"), c.Query("sortDir"))).Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcements"})
		return
	}

	items := make([]gin.H, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, gin.H{
			"id":        n.ID,
			"title":     n.Title,
			"content":   n.Message,
			"type":      n.Type,
			"priority":  n.Priority,
			"category":  n.Category,
			"link":      n.Link,
			"isActive":  n.IsActive != nil && *n.IsActive,
			"createdAt": n.CreatedAt,
			"author": gin.H{
				"id":     n.UserID,
				"name":   announcementAuthorName(n.UserID),
				"avatar": nil,
			},
		})
	}

	api_response.List(c, items, api_response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"announcements": items,
	})
}

// splitCommaQuery splits a comma-separated query value, trimming spaces
// and dropping empty parts.
func splitCommaQuery(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// orderDirection normalizes a sort direction to a SQL keyword.
func orderDirection(sortDir string) string {
	if strings.EqualFold(sortDir, "asc") {
		return "ASC"
	}
	return "DESC"
}

// buildAnnouncementOrder builds a safe ORDER BY clause. Priority is ranked
// semantically (HIGH > MEDIUM > LOW) instead of alphabetically, and a
// created_at secondary sort keeps paging stable for equal keys.
func buildAnnouncementOrder(sortBy, sortDir string) string {
	dir := orderDirection(sortDir)
	switch sortBy {
	case "priority":
		return "CASE priority WHEN 'HIGH' THEN 0 WHEN 'MEDIUM' THEN 1 ELSE 2 END " + dir + ", created_at DESC"
	case "title":
		return "title " + dir + ", created_at DESC"
	case "type":
		return "type " + dir + ", created_at DESC"
	case "isActive":
		return "is_active " + dir + ", created_at DESC"
	default:
		return createdAtDescSort
	}
}

// announcementAuthorName resolves the announcement author's display name,
// falling back to "النظام" for system generated entries.
func announcementAuthorName(userID string) string {
	if userID == "" || userID == "system" {
		return "النظام"
	}
	names := userNameLookup([]string{userID})
	return firstNonEmpty(names[userID], "النظام")
}

func CreateAdminAnnouncement(c *gin.Context) {
	var input struct {
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content" binding:"required"`
		Type     string `json:"type"`
		Priority string `json:"priority"`
		Category string `json:"category"`
		Link     string `json:"link"`
		IsActive *bool  `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	notification := models.Notification{
		UserID:   "system",
		Title:    input.Title,
		Message:  input.Content,
		Type:     models.NotificationType(input.Type),
		Priority: strings.ToUpper(defaultString(input.Priority, "MEDIUM")),
		Category: strings.ToUpper(defaultString(input.Category, "GENERAL")),
		IsActive: &isActive,
		IsRead:   false,
	}
	if notification.Type == "" {
		notification.Type = models.NotificationInfo
	}
	if strings.TrimSpace(input.Link) != "" {
		notification.Link = &input.Link
	}

	if err := SafeCreate(db.DB, &notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create announcement"})
		return
	}

	broadcastMsg, _ := json.Marshal(gin.H{
		"type": "notification",
		"payload": gin.H{
			"title":   notification.Title,
			"message": notification.Message,
			"type":    notification.Type,
		},
	})
	GlobalHub.broadcast <- broadcastMsg

	c.JSON(http.StatusCreated, gin.H{"success": true, "id": notification.ID})
}

func UpdateAdminAnnouncement(c *gin.Context) {
	var input struct {
		ID       string `json:"id" binding:"required"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Type     string `json:"type"`
		Priority string `json:"priority"`
		Category string `json:"category"`
		Link     string `json:"link"`
		IsActive *bool  `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = input.Title
	}
	if input.Content != "" {
		updates["message"] = input.Content
	}
	if input.Type != "" {
		updates["type"] = input.Type
	}
	if input.Priority != "" {
		updates["priority"] = strings.ToUpper(input.Priority)
	}
	if input.Category != "" {
		updates["category"] = strings.ToUpper(input.Category)
	}
	if input.Link != "" {
		updates["link"] = input.Link
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	if err := db.DB.Model(&models.Notification{}).Where(queryID, input.ID).
		Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteAdminAnnouncement(c *gin.Context) {
	var input struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.DB.Delete(&models.Notification{}, queryID, input.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func MarkActivityRead(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	activityID := c.Param("id")
	if activityID == "" {
		var req struct {
			ActivityID string `json:"activityId"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.ActivityID != "" {
			activityID = req.ActivityID
		}
	}

	if activityID == "" {
		api_response.Error(c, http.StatusBadRequest, msgIDRequired)
		return
	}

	if err := db.DB.Model(&models.Notification{}).Where("id = ? AND user_id = ?", activityID, userId).Update("is_read", true).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update activity")
		return
	}

	api_response.Success(c, nil)
}

func MarkAllActivitiesRead(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := db.DB.Model(&models.Notification{}).Where("user_id = ?", userId).Update("is_read", true).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update activities")
		return
	}

	api_response.Success(c, nil)
}
