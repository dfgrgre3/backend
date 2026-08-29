package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminGetEvents returns a paginated list of platform events.
func AdminGetEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	query := db.DB.Model(&models.Event{})

	if search := c.Query("search"); search != "" {
		query = query.Where("title ILIKE ?", "%"+search+"%")
	}
	query.Count(&total)

	var events []models.Event
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	items := make([]gin.H, 0, len(events))
	for _, e := range events {
		items = append(items, gin.H{
			"id":           e.ID,
			"title":        e.Title,
			"description":  e.Description,
			"type":         e.Type,
			"startDate":    e.StartDate,
			"endDate":      e.EndDate,
			"location":     e.Location,
			"isOnline":     e.IsOnline,
			"maxAttendees": e.MaxAttendees,
			"isActive":     e.IsActive,
			"createdAt":    e.CreatedAt,
			"_count": gin.H{
				"attendees": e.AttendeesCount,
			},
		})
	}

	pagination := api_response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}
	api_response.List(c, items, pagination, gin.H{"events": items})
}

// AdminCreateEvent creates a new platform event.
func AdminCreateEvent(c *gin.Context) {
	var input struct {
		Title        string  `json:"title" binding:"required"`
		Description  *string `json:"description"`
		Type         string  `json:"type"`
		StartDate    string  `json:"startDate" binding:"required"`
		EndDate      string  `json:"endDate" binding:"required"`
		Location     *string `json:"location"`
		IsOnline     bool    `json:"isOnline"`
		MaxAttendees *int    `json:"maxAttendees"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	startDate, err := parseFlexibleDate(input.StartDate)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid start date format")
		return
	}
	endDate, err := parseFlexibleDate(input.EndDate)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid end date format")
		return
	}

	eventType := input.Type
	if eventType == "" {
		eventType = "workshop"
	}

	event := models.Event{
		Title:        input.Title,
		Description:  input.Description,
		Type:         eventType,
		StartDate:    startDate,
		EndDate:      endDate,
		Location:     input.Location,
		IsOnline:     input.IsOnline,
		MaxAttendees: input.MaxAttendees,
		IsActive:     true,
	}

	if err := SafeCreate(db.DB, &event); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	LogAudit(c, "CREATE", "event", event.ID, event)
	api_response.Created(c, event)
}
