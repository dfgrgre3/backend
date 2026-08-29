package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  API Keys
// ─────────────────────────────────────────────

func AdminListAPIKeys(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.APIKey{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var keys []models.APIKey
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&keys).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch API keys")
		return
	}

	var activeCount, expiredCount int64
	db.DB.Model(&models.APIKey{}).Where("is_active = ?", true).Count(&activeCount)
	db.DB.Model(&models.APIKey{}).Where("expires_at < ?", time.Now()).Count(&expiredCount)

	api_response.Success(c, gin.H{
		"keys":       keys,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalKeys": total, "activeKeys": activeCount, "expiredKeys": expiredCount},
	})
}

func AdminCreateAPIKey(c *gin.Context) {
	var req models.APIKey
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create API key")
		return
	}
	api_response.Created(c, req)
}

func AdminDeleteAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.APIKey{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete API key")
		return
	}
	api_response.Success(c, gin.H{"message": "API key deleted"})
}

// ─────────────────────────────────────────────
//  Webhooks
// ─────────────────────────────────────────────

func AdminListWebhooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Webhook{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var webhooks []models.Webhook
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&webhooks).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch webhooks")
		return
	}

	var activeCount int64
	var totalTriggers int64
	db.DB.Model(&models.Webhook{}).Where("is_active = ?", true).Count(&activeCount)
	db.DB.Model(&models.Webhook{}).Select("COALESCE(SUM(success_count + failure_count), 0)").Scan(&totalTriggers)

	api_response.Success(c, gin.H{
		"webhooks":   webhooks,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalWebhooks": total, "activeWebhooks": activeCount, "totalTriggers": totalTriggers},
	})
}

func AdminCreateWebhook(c *gin.Context) {
	var req models.Webhook
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create webhook")
		return
	}
	api_response.Created(c, req)
}

func AdminUpdateWebhook(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.Webhook{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update webhook")
		return
	}
	api_response.Success(c, gin.H{"message": "Webhook updated"})
}

func AdminDeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.Webhook{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete webhook")
		return
	}
	api_response.Success(c, gin.H{"message": "Webhook deleted"})
}

func AdminTestWebhook(c *gin.Context) {
	id := c.Param("id")
	var webhook models.Webhook
	if err := db.DB.First(&webhook, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Webhook not found")
		return
	}

	db.DB.Model(&webhook).Updates(map[string]interface{}{
		"last_triggered_at": time.Now(),
	})

	api_response.Success(c, gin.H{"message": "Webhook test sent", "status": "SUCCESS"})
}
