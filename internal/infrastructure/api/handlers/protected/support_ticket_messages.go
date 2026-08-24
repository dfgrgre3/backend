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

// SendTicketMessage sends a message on a ticket
// @Summary Send ticket message
// @Description Send a message on a support ticket
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Param request body SendMessageRequest true "Message details"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/tickets/{id}/messages [post]
func SendTicketMessage(c *gin.Context) {
	id := c.Param("id")
	adminIDVal, _ := c.Get("userId")
	senderID := ""
	if v, ok := adminIDVal.(string); ok {
		senderID = v
	}

	senderName := getAdminSenderName(senderID)

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var ticket models.SupportTicket
	if err := db.DB.First(&ticket, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errTicketNotFound)
		return
	}

	// Update ticket status if it's the first admin response
	updateTicketStatusOnResponse(&ticket, req.IsInternal)

	// Create message
	message := models.TicketMessage{
		TicketID:   id,
		SenderID:   senderID,
		SenderName: senderName,
		SenderRole: "admin",
		Message:    req.Message,
		IsInternal: req.IsInternal,
		CreatedAt:  time.Now(),
	}

	if err := SafeCreate(db.DB, &message); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	// Notify user if not internal
	if !req.IsInternal {
		notifyUserOfTicketResponse(ticket)
	}

	api_response.Success(c, gin.H{
		"message":       "Message sent successfully",
		"ticketMessage": message,
	})
}

func getAdminSenderName(senderID string) string {
	senderName := "Admin"
	if senderID == "" {
		return senderName
	}

	var adminUser models.User
	if err := db.DB.First(&adminUser, idQuery, senderID).Error; err != nil {
		return senderName
	}

	if adminUser.Name != nil && *adminUser.Name != "" {
		return *adminUser.Name
	}
	if adminUser.Username != nil && *adminUser.Username != "" {
		return *adminUser.Username
	}
	return adminUser.Email
}

func updateTicketStatusOnResponse(ticket *models.SupportTicket, isInternal bool) {
	if ticket.Status == "open" && !isInternal {
		ticket.Status = "in_progress"
	}
	ticket.UpdatedAt = time.Now()
	db.DB.Save(ticket)
}

func notifyUserOfTicketResponse(ticket models.SupportTicket) {
	notificationservice.GetNotificationService().QueueNotification(models.Notification{
		UserID:   ticket.UserID,
		Title:    "New Response on Your Ticket",
		Message:  "Admin has responded to your ticket: " + ticket.Subject,
		Type:     "info",
		Channels: []string{channelInApp, "email"},
	})
}
