package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetSupportTickets returns all support tickets with filtering
// @Summary Get support tickets
// @Description Get all support tickets with optional filtering
// @Tags admin,support
// @Accept json
// @Produce json
// @Param status query string false "Filter by status"
// @Param priority query string false "Filter by priority"
// @Param category query string false "Filter by category"
// @Param assignedTo query string false "Filter by assignee"
// @Param search query string false "Search in subject/description"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/tickets [get]
func GetSupportTickets(c *gin.Context) {
	status := c.Query("status")
	priority := c.Query("priority")
	category := c.Query("category")
	assignedTo := c.Query("assignedTo")
	search := c.Query("search")
	from := c.Query("from")
	to := c.Query("to")

	query := db.DB.Model(&models.SupportTicket{}).Preload("Messages").Order("updated_at DESC")

	if status != "" {
		query = query.Where(statusQuery, status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if assignedTo != "" {
		query = query.Where("assigned_to = ?", assignedTo)
	}
	if search != "" {
		query = query.Where("subject ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
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

	var tickets []models.SupportTicket
	if err := query.Find(&tickets).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch tickets")
		return
	}

	api_response.Success(c, gin.H{
		"tickets": tickets,
		"count":   len(tickets),
	})
}

// GetSupportTicket returns a single ticket with messages
// @Summary Get support ticket
// @Description Get a specific support ticket with all messages
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/tickets/{id} [get]
func GetSupportTicket(c *gin.Context) {
	id := c.Param("id")

	var ticket models.SupportTicket
	if err := db.DB.Preload("Messages").First(&ticket, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errTicketNotFound)
		return
	}

	api_response.Success(c, gin.H{
		"ticket": ticket,
	})
}
