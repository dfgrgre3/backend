package protected

import (
	"net/http"
	"time"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func AdminUpdateEvent(c *gin.Context) {
	var input struct {
		ID           string  `json:"id" binding:"required"`
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		Type         *string `json:"type"`
		StartDate    *string `json:"startDate"`
		EndDate      *string `json:"endDate"`
		Location     *string `json:"location"`
		IsOnline     *bool   `json:"isOnline"`
		MaxAttendees *int    `json:"maxAttendees"`
		IsActive     *bool   `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var event models.Event
	if err := db.DB.First(&event, whereIDEquals, input.ID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	type eventUpdates struct {
		Title        *string    `gorm:"column:title"`
		Description  *string    `gorm:"column:description"`
		Type         *string    `gorm:"column:type"`
		StartDate    *time.Time `gorm:"column:start_date"`
		EndDate      *time.Time `gorm:"column:end_date"`
		Location     *string    `gorm:"column:location"`
		IsOnline     *bool      `gorm:"column:is_online"`
		MaxAttendees *int       `gorm:"column:max_attendees"`
		IsActive     *bool      `gorm:"column:is_active"`
	}

	updates := eventUpdates{
		Title:        input.Title,
		Description:  input.Description,
		Type:         input.Type,
		Location:     input.Location,
		IsOnline:     input.IsOnline,
		MaxAttendees: input.MaxAttendees,
		IsActive:     input.IsActive,
	}

	if input.StartDate != nil {
		if t, err := parseFlexibleDate(*input.StartDate); err == nil {
			updates.StartDate = &t
		}
	}
	if input.EndDate != nil {
		if t, err := parseFlexibleDate(*input.EndDate); err == nil {
			updates.EndDate = &t
		}
	}

	if err := db.DB.Model(&models.Event{}).Where(whereIDEquals, event.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update event")
		return
	}

	LogAudit(c, "UPDATE", "event", input.ID, updates)
	api_response.Success(c, nil)
}

// AdminDeleteEvent deletes a platform event.
func AdminDeleteEvent(c *gin.Context) {
	var input struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.DB.Delete(&models.Event{}, whereIDEquals, input.ID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}

	LogAudit(c, "DELETE", "event", input.ID, nil)
	api_response.Success(c, nil)
}
