package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  System Logs
// ─────────────────────────────────────────────

func AdminListSystemLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	level := c.Query("level")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.SystemLog{})
	if search != "" {
		query = query.Where("message ILIKE ? OR service ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if level != "" && level != "all" {
		query = query.Where("level = ?", level)
	}

	var total int64
	query.Count(&total)

	var logs []models.SystemLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch system logs")
		return
	}

	var infoCount, warnCount, errorCount, debugCount int64
	for _, l := range logs {
		switch l.Level {
		case "INFO":
			infoCount++
		case "WARN":
			warnCount++
		case "ERROR":
			errorCount++
		case "DEBUG":
			debugCount++
		}
	}

	api_response.Success(c, gin.H{
		"logs":       logs,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "infoCount": infoCount, "warnCount": warnCount, "errorCount": errorCount, "debugCount": debugCount},
	})
}
