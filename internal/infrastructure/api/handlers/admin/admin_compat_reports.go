package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func AdminReportsContent(c *gin.Context) {
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

	switch c.Request.Method {
	case http.MethodGet:
		var reports []models.ContentReport
		var total int64
		var pending int64
		var resolved int64

		query := db.DB.Model(&models.ContentReport{})
		if status := c.Query("status"); status != "" && status != "all" {
			query = query.Where(statusQuery, status)
		}
		query.Count(&total)
		db.DB.Model(&models.ContentReport{}).Where(statusQuery, "PENDING").Count(&pending)
		db.DB.Model(&models.ContentReport{}).Where(statusQuery, "RESOLVED").Count(&resolved)

		query.Preload("Reporter").Order(createdAtDescSort).Limit(limit).Offset((page - 1) * limit).Find(&reports)

		api_response.Success(c, gin.H{
			"reports": reports,
			"items":   reports,
			"stats": gin.H{
				"pending":  pending,
				"resolved": resolved,
				"total":    total,
			},
			"pagination": gin.H{
				"page": page, "limit": limit, "total": total,
				"totalPages": (total + int64(limit) - 1) / int64(limit),
			},
		})

	case http.MethodPatch:
		var input struct {
			ID     string `json:"id" binding:"required"`
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			api_response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		validReportStatuses := map[string]bool{"PENDING": true, "RESOLVED": true, "DISMISSED": true}
		if input.Status != "" && !validReportStatuses[input.Status] {
			api_response.Error(c, http.StatusBadRequest, "Invalid status: must be PENDING, RESOLVED, or DISMISSED")
			return
		}

		type reportUpdates struct {
			Status     *string    `gorm:"column:status"`
			ResolvedAt *time.Time `gorm:"column:resolved_at"`
			ResolvedBy *string    `gorm:"column:resolved_by"`
		}
		updates := reportUpdates{
			Status: &input.Status,
		}
		if input.Status == "RESOLVED" || input.Status == "DISMISSED" {
			now := time.Now()
			updates.ResolvedAt = &now
			if userId, exists := c.Get("userId"); exists {
				if uid, ok := userId.(string); ok {
					updates.ResolvedBy = &uid
				}
			}
		}
		if err := db.DB.Model(&models.ContentReport{}).Where(idQuery, input.ID).
			Updates(&updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update report")
			return
		}
		LogAudit(c, "UPDATE", "content_report", input.ID, updates)
		api_response.Success(c, nil)

	default:
		api_response.Success(c, gin.H{"reports": []interface{}{}, "stats": gin.H{"pending": 0, "resolved": 0, "total": 0}})
	}
}
