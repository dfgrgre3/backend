package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AssignTicket assigns a ticket to an admin
// @Summary Assign ticket
// @Description Assign a support ticket to an admin
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Param request body map[string]string true "Admin ID to assign to"
// @Success 200 {object} map[string]string
// @Router /api/admin/tickets/{id}/assign [post]
func AssignTicket(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		AdminID string `json:"adminId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Get admin name
	var admin models.User
	if err := db.DB.First(&admin, idQuery, req.AdminID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Admin not found")
		return
	}

	if err := db.DB.Model(&models.SupportTicket{}).
		Where(idQuery, id).
		Updates(map[string]interface{}{
			"assigned_to":      req.AdminID,
			"assigned_to_name": admin.GetName(),
			"updated_at":       time.Now(),
		}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to assign ticket")
		return
	}

	api_response.Success(c, gin.H{"message": "Ticket assigned successfully"})
}

// CloseTicket closes a support ticket
// @Summary Close ticket
// @Description Close a support ticket permanently
// @Tags admin,support
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/tickets/{id}/close [post]
func CloseTicket(c *gin.Context) {
	id := c.Param("id")

	if err := db.DB.Model(&models.SupportTicket{}).
		Where(idQuery, id).
		Updates(map[string]interface{}{
			"status":     "closed",
			"closed_at":  time.Now(),
			"updated_at": time.Now(),
		}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to close ticket")
		return
	}

	api_response.Success(c, gin.H{"message": "Ticket closed successfully"})
}
