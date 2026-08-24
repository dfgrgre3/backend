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

// UpdateTicketStatus updates the status of a ticket
// @Summary Update ticket status
// @Description Update the status of a support ticket
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Param request body UpdateTicketStatusRequest true "Status update"
// @Success 200 {object} map[string]string
// @Router /api/admin/tickets/{id}/status [patch]
func UpdateTicketStatus(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var ticket models.SupportTicket
	if err := db.DB.First(&ticket, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errTicketNotFound)
		return
	}

	type ticketUpdates struct {
		Status     *string    `gorm:"column:status"`
		UpdatedAt  *time.Time `gorm:"column:updated_at"`
		ResolvedAt *time.Time `gorm:"column:resolved_at"`
		ClosedAt   *time.Time `gorm:"column:closed_at"`
	}

	now := time.Now()
	updates := ticketUpdates{
		Status:    &req.Status,
		UpdatedAt: &now,
	}

	// Set timestamps based on status
	if req.Status == "resolved" {
		updates.ResolvedAt = &now
	}
	if req.Status == "closed" {
		updates.ClosedAt = &now
	}

	if err := db.DB.Model(&models.SupportTicket{}).Where(idQuery, ticket.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update status")
		return
	}

	// Notify user of status change
	notificationservice.GetNotificationService().QueueNotification(models.Notification{
		UserID:   ticket.UserID,
		Title:    "Ticket Status Updated",
		Message:  "Your ticket '" + ticket.Subject + "' is now " + req.Status,
		Type:     "info",
		Channels: []string{channelInApp},
	})

	api_response.Success(c, gin.H{"message": "Status updated successfully"})
}

// UpdateTicketPriority updates the priority of a ticket
// @Summary Update ticket priority
// @Description Update the priority of a support ticket
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Param request body UpdateTicketPriorityRequest true "Priority update"
// @Success 200 {object} map[string]string
// @Router /api/admin/tickets/{id}/priority [patch]
func UpdateTicketPriority(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTicketPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.DB.Model(&models.SupportTicket{}).
		Where(idQuery, id).
		Updates(map[string]interface{}{
			"priority":   req.Priority,
			"updated_at": time.Now(),
		}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update priority")
		return
	}

	api_response.Success(c, gin.H{"message": "Priority updated successfully"})
}
