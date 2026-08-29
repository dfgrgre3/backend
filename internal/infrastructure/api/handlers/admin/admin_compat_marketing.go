package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func Marketing(c *gin.Context) {
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
		handleMarketingGet(c, page, limit)
	case http.MethodPost:
		handleMarketingPost(c)
	case http.MethodPatch, http.MethodPut:
		handleMarketingUpdate(c)
	case http.MethodDelete:
		handleMarketingDelete(c)
	default:
		api_response.Error(c, http.StatusMethodNotAllowed, shared.MsgMethodNotAllowed)
	}
}

func handleMarketingGet(c *gin.Context, page, limit int) {
	var campaigns []models.Campaign
	var total int64
	db.DB.Model(&models.Campaign{}).Count(&total)
	db.DB.Order(createdAtDescSort).Limit(limit).Offset((page - 1) * limit).Find(&campaigns)

	pagination := gin.H{
		"page": page, "limit": limit, "total": total,
		"totalPages": (total + int64(limit) - 1) / int64(limit),
	}
	api_response.Success(c, gin.H{
		"campaigns":  campaigns,
		"items":      campaigns,
		"pagination": pagination,
	})
}

func handleMarketingPost(c *gin.Context) {
	var item models.Campaign
	if err := c.ShouldBindJSON(&item); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &item); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create campaign")
		return
	}
	api_response.Created(c, item)
}

func handleMarketingUpdate(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	id, _ := input["id"].(string)
	if id == "" {
		api_response.Error(c, http.StatusBadRequest, shared.MsgIDRequired)
		return
	}
	var item models.Campaign
	if err := db.DB.Where(idQuery, id).First(&item).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Campaign not found")
		return
	}

	type campaignUpdates struct {
		Name        *string `gorm:"column:name"`
		Description *string `gorm:"column:description"`
		Type        *string `gorm:"column:type"`
		Status      *string `gorm:"column:status"`
		TargetRole  *string `gorm:"column:target_role"`
		Content     *string `gorm:"column:content"`
		StartDate   *string `gorm:"column:start_date"`
		EndDate     *string `gorm:"column:end_date"`
	}

	var updates campaignUpdates
	if v, ok := input["name"].(string); ok {
		updates.Name = &v
	}
	if v, ok := input["description"].(string); ok {
		updates.Description = &v
	}
	if v, ok := input["type"].(string); ok {
		updates.Type = &v
	}
	if v, ok := input["status"].(string); ok {
		updates.Status = &v
	}
	if v, ok := input["targetRole"].(string); ok {
		updates.TargetRole = &v
	}
	if v, ok := input["content"].(string); ok {
		updates.Content = &v
	}
	if v, ok := input["startDate"].(string); ok {
		updates.StartDate = &v
	}
	if v, ok := input["endDate"].(string); ok {
		updates.EndDate = &v
	}

	if err := db.DB.Model(&models.Campaign{}).Where(idQuery, id).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update campaign")
		return
	}
	LogAudit(c, "UPDATE", "campaign", id, updates)
	api_response.Success(c, item)
}

func handleMarketingDelete(c *gin.Context) {
	var input struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.ID == "" {
		api_response.Error(c, http.StatusBadRequest, shared.MsgIDRequired)
		return
	}
	if err := db.DB.Where(idQuery, input.ID).Delete(&models.Campaign{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete campaign")
		return
	}
	LogAudit(c, "DELETE", "campaign", input.ID, nil)
	api_response.Success(c, nil)
}
