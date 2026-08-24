package protected

import (
	"encoding/json"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Cache Management
// ─────────────────────────────────────────────

func AdminGetCacheStats(c *gin.Context) {
	var entries []models.CacheEntry
	if err := db.DB.Find(&entries).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch cache stats")
		return
	}

	var totalSize, totalHits, totalMisses int64
	items := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		totalSize += e.Size
		totalHits += e.Hits
		totalMisses += e.Misses
		items = append(items, gin.H{
			"key":          e.Key,
			"type":         e.Type,
			"size":         e.Size,
			"hits":         e.Hits,
			"misses":       e.Misses,
			"lastAccessed": e.LastAccessed,
			"expiresAt":    e.ExpiresAt,
		})
	}

	total := totalHits + totalMisses
	hitRate := 0.0
	missRate := 0.0
	if total > 0 {
		hitRate = float64(totalHits) / float64(total) * 100
		missRate = float64(totalMisses) / float64(total) * 100
	}

	api_response.Success(c, gin.H{
		"entries": items,
		"summary": gin.H{
			"totalEntries": len(entries),
			"totalSize":    totalSize,
			"hitRate":      hitRate,
			"missRate":     missRate,
		},
	})
}

func AdminClearCache(c *gin.Context) {
	key := c.Param("key")
	if key != "" {
		if err := db.DB.Delete(&models.CacheEntry{}, "key = ?", key).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to clear cache")
			return
		}
	} else {
		if err := db.DB.Delete(&models.CacheEntry{}).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to clear all cache")
			return
		}
	}
	api_response.Success(c, gin.H{"message": "Cache cleared"})
}

// ─────────────────────────────────────────────
//  Email Templates
// ─────────────────────────────────────────────

func AdminListEmailTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.EmailTemplate{})
	if search != "" {
		query = query.Where("name ILIKE ? OR subject ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var templates []models.EmailTemplate
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&templates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch email templates")
		return
	}

	var activeCount int64
	db.DB.Model(&models.EmailTemplate{}).Where("is_active = ?", true).Count(&activeCount)
	var categories int64
	db.DB.Model(&models.EmailTemplate{}).Select("COUNT(DISTINCT category)").Scan(&categories)

	api_response.Success(c, gin.H{
		"templates":  templates,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalTemplates": total, "activeTemplates": activeCount, "categories": categories},
	})
}

func AdminCreateEmailTemplate(c *gin.Context) {
	var req models.EmailTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create email template")
		return
	}
	api_response.Created(c, req)
}

func AdminUpdateEmailTemplate(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.EmailTemplate{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update email template")
		return
	}
	api_response.Success(c, gin.H{"message": "Email template updated"})
}

func AdminDeleteEmailTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.EmailTemplate{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete email template")
		return
	}
	api_response.Success(c, gin.H{"message": "Email template deleted"})
}

// ─────────────────────────────────────────────
//  Feature Flags
// ─────────────────────────────────────────────

func AdminListFeatureFlags(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.FeatureFlag{})
	if search != "" {
		query = query.Where("name ILIKE ? OR key ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var flags []models.FeatureFlag
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&flags).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch feature flags")
		return
	}

	var enabledCount, disabledCount int64
	db.DB.Model(&models.FeatureFlag{}).Where("is_enabled = ?", true).Count(&enabledCount)
	db.DB.Model(&models.FeatureFlag{}).Where("is_enabled = ?", false).Count(&disabledCount)

	api_response.Success(c, gin.H{
		"flags":      flags,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalFlags": total, "enabledFlags": enabledCount, "disabledFlags": disabledCount},
	})
}

func AdminUpdateFeatureFlag(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.FeatureFlag{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update feature flag")
		return
	}
	api_response.Success(c, gin.H{"message": "Feature flag updated"})
}

func AdminDeleteFeatureFlag(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.FeatureFlag{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete feature flag")
		return
	}
	api_response.Success(c, gin.H{"message": "Feature flag deleted"})
}

// ─────────────────────────────────────────────
//  Maintenance Mode
// ─────────────────────────────────────────────

func AdminGetMaintenanceMode(c *gin.Context) {
	var settings models.SystemSetting
	if err := db.DB.First(&settings, "key = ?", "maintenance_mode").Error; err != nil {
		api_response.Success(c, gin.H{
			"isEnabled":  false,
			"message":    "",
			"allowedIPs": []string{},
			"startTime":  nil,
			"endTime":    nil,
			"updatedAt":  time.Now(),
		})
		return
	}
	api_response.Success(c, gin.H{"value": settings.Value})
}

func AdminUpdateMaintenanceMode(c *gin.Context) {
	var req struct {
		IsEnabled bool   `json:"isEnabled"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	valueBytes, err := json.Marshal(gin.H{
		"isEnabled": req.IsEnabled,
		"message":   req.Message,
		"updatedAt": time.Now(),
	})
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to encode maintenance mode settings")
		return
	}
	value := string(valueBytes)

	var setting models.SystemSetting
	if err := db.DB.First(&setting, "key = ?", "maintenance_mode").Error; err != nil {
		setting = models.SystemSetting{
			Key:   "maintenance_mode",
			Value: value,
		}
		SafeCreate(db.DB, &setting)
	} else {
		setting.Value = value
		db.DB.Save(&setting)
	}

	api_response.Success(c, gin.H{"message": "Maintenance mode updated"})
}
