package protected

import (
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetTicketStats returns ticket statistics
// @Summary Get ticket statistics
// @Description Get statistics about support tickets
// @Tags admin,support
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/tickets/stats [get]
func GetTicketStats(c *gin.Context) {
	var stats struct {
		Total      int64 `json:"total"`
		Open       int64 `json:"open"`
		InProgress int64 `json:"inProgress"`
		Resolved   int64 `json:"resolved"`
		Closed     int64 `json:"closed"`
		Unassigned int64 `json:"unassigned"`
		Urgent     int64 `json:"urgent"`
	}

	db.DB.Model(&models.SupportTicket{}).Count(&stats.Total)
	db.DB.Model(&models.SupportTicket{}).Where(statusQuery, "open").Count(&stats.Open)
	db.DB.Model(&models.SupportTicket{}).Where(statusQuery, "in_progress").Count(&stats.InProgress)
	db.DB.Model(&models.SupportTicket{}).Where(statusQuery, "resolved").Count(&stats.Resolved)
	db.DB.Model(&models.SupportTicket{}).Where(statusQuery, "closed").Count(&stats.Closed)
	db.DB.Model(&models.SupportTicket{}).Where("assigned_to IS NULL").Count(&stats.Unassigned)
	db.DB.Model(&models.SupportTicket{}).Where("priority = ?", "urgent").Count(&stats.Urgent)

	// Average resolution time (for resolved tickets in last 30 days)
	var avgResolutionTime float64
	db.DB.Raw(`
		SELECT AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)) / 3600)
		FROM support_tickets
		WHERE status = 'resolved'
		AND resolved_at >= NOW() - INTERVAL '30 days'
	`).Scan(&avgResolutionTime)

	api_response.Success(c, gin.H{
		"overview":          stats,
		"avgResolutionTime": avgResolutionTime,
	})
}
