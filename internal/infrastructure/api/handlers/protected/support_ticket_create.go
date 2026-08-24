package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	systemservice "thanawy-backend/internal/domain/system/service"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// CreateSupportTicket creates a new support ticket (admin-initiated)
// @Summary Create support ticket
// @Description Create a new support ticket on behalf of a user
// @Tags admin,support
// @Accept json
// @Produce json
// @Param request body CreateTicketRequest true "Ticket details"
// @Success 201 {object} map[string]interface{}
// @Router /api/admin/tickets [post]
func CreateSupportTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminIDVal, _ := c.Get("userId")
	senderID := "00000000-0000-0000-0000-000000000000"
	if v, ok := adminIDVal.(string); ok && v != "" {
		senderID = v
	}

	// Set default priority
	if req.Priority == "" {
		req.Priority = "medium"
	}

	// Generate ticket number
	ticketNumber := systemservice.GenerateTicketNumber()

	// Get user info
	var user models.User
	if err := db.DB.First(&user, idQuery, req.UserID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	ticket := models.SupportTicket{
		TicketNumber:      ticketNumber,
		UserID:            req.UserID,
		UserName:          user.GetName(),
		UserEmail:         user.Email,
		Subject:           req.Subject,
		Description:       req.Description,
		Category:          req.Category,
		Status:            "open",
		Priority:          req.Priority,
		RelatedEntityType: req.RelatedEntityType,
		RelatedEntityID:   req.RelatedEntityID,
		Tags:              []string{},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := SafeCreate(db.DB, &ticket); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create ticket")
		return
	}

	// Log operation
	middleware.LogCriticalOperation(c, "ticket_created", map[string]interface{}{
		"ticket_number": ticketNumber,
		"user_id":       req.UserID,
		"category":      req.Category,
	})

	// Create initial system message
	systemMessage := models.TicketMessage{
		TicketID:   ticket.ID,
		SenderID:   senderID,
		SenderName: "System",
		SenderRole: "system",
		Message:    "Ticket created by admin",
		IsInternal: true,
		CreatedAt:  time.Now(),
	}
	SafeCreate(db.DB, &systemMessage)

	// Notify user
	notificationservice.GetNotificationService().QueueNotification(models.Notification{
		UserID:   req.UserID,
		Title:    "New Support Ticket",
		Message:  "A support ticket has been created for you: " + req.Subject,
		Type:     "info",
		Channels: []string{channelInApp, "email"},
	})

	api_response.Success(c, gin.H{
		"message": "Ticket created successfully",
		"ticket":  ticket,
	})
}
