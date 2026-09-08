package admin

import (
	"math"
	"net/http"
	"strconv"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------
// Banners
// ---------------------------------------------------------------------

func AdminListBanners(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Banner{})
	if position := c.Query("position"); position != "" {
		query = query.Where("\"position\" = ?", position)
	}

	var total int64
	query.Count(&total)

	var banners []models.Banner
	if err := query.Order("display_order ASC, created_at DESC").Offset(offset).Limit(limit).Find(&banners).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch banners")
		return
	}

	api_response.Success(c, gin.H{
		"items": banners,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func AdminCreateBanner(c *gin.Context) {
	var banner models.Banner
	if err := c.ShouldBindJSON(&banner); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid banner payload")
		return
	}
	if err := db.DB.Create(&banner).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create banner")
		return
	}
	api_response.Created(c, banner)
}

func AdminUpdateBanner(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid banner payload")
		return
	}
	result := db.DB.Model(&models.Banner{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update banner")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Banner not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Banner updated"})
}

func AdminDeleteBanner(c *gin.Context) {
	id := c.Param("id")
	result := db.DB.Where("id = ?", id).Delete(&models.Banner{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete banner")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Banner not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Banner deleted"})
}

// ---------------------------------------------------------------------
// FAQ
// ---------------------------------------------------------------------

func AdminListFAQs(c *gin.Context) {
	query := db.DB.Model(&models.FAQ{})
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	var faqs []models.FAQ
	if err := query.Order("display_order ASC, created_at DESC").Find(&faqs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch FAQs")
		return
	}

	api_response.Success(c, gin.H{"items": faqs, "total": len(faqs)})
}

func AdminCreateFAQ(c *gin.Context) {
	var faq models.FAQ
	if err := c.ShouldBindJSON(&faq); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid FAQ payload")
		return
	}
	if err := db.DB.Create(&faq).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create FAQ")
		return
	}
	api_response.Created(c, faq)
}

func AdminUpdateFAQ(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid FAQ payload")
		return
	}
	result := db.DB.Model(&models.FAQ{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update FAQ")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "FAQ not found")
		return
	}
	api_response.Success(c, gin.H{"message": "FAQ updated"})
}

func AdminDeleteFAQ(c *gin.Context) {
	id := c.Param("id")
	result := db.DB.Where("id = ?", id).Delete(&models.FAQ{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete FAQ")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "FAQ not found")
		return
	}
	api_response.Success(c, gin.H{"message": "FAQ deleted"})
}

// ---------------------------------------------------------------------
// Homepage sections (a.k.a. "landing" sections)
// ---------------------------------------------------------------------

func AdminListLandingSections(c *gin.Context) {
	var sections []models.HomepageSection
	if err := db.DB.Order("display_order ASC").Find(&sections).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch homepage sections")
		return
	}
	api_response.Success(c, gin.H{"items": sections, "total": len(sections)})
}

// AdminUpsertLandingSection creates or updates a section by its unique key.
func AdminUpsertLandingSection(c *gin.Context) {
	var payload models.HomepageSection
	if err := c.ShouldBindJSON(&payload); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid section payload")
		return
	}
	if payload.Key == "" {
		api_response.Error(c, http.StatusBadRequest, "key is required")
		return
	}

	var existing models.HomepageSection
	err := db.DB.Where("key = ?", payload.Key).First(&existing).Error
	if err == nil {
		payload.ID = existing.ID
		if updErr := db.DB.Model(&existing).Updates(map[string]interface{}{
			"type":          payload.Type,
			"title":         payload.Title,
			"content":       payload.Content,
			"is_active":     payload.IsActive,
			"display_order": payload.DisplayOrder,
		}).Error; updErr != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update section")
			return
		}
		api_response.Success(c, gin.H{"message": "Section updated"})
		return
	}

	if createErr := db.DB.Create(&payload).Error; createErr != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create section")
		return
	}
	api_response.Created(c, payload)
}

// ---------------------------------------------------------------------
// CMS pages
// ---------------------------------------------------------------------

func AdminListCMSPages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.CMSPage{})
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var pages []models.CMSPage
	if err := query.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&pages).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pages")
		return
	}

	api_response.Success(c, gin.H{
		"items": pages,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func AdminGetCMSPage(c *gin.Context) {
	id := c.Param("id")
	var page models.CMSPage
	if err := db.DB.Where("id = ? OR slug = ?", id, id).First(&page).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "CMS page not found")
		return
	}
	api_response.Success(c, page)
}

func AdminCreateCMSPage(c *gin.Context) {
	var page models.CMSPage
	if err := c.ShouldBindJSON(&page); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid page payload")
		return
	}
	if err := db.DB.Create(&page).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create page (slug may already exist)")
		return
	}
	api_response.Created(c, page)
}

func AdminUpdateCMSPage(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid page payload")
		return
	}
	result := db.DB.Model(&models.CMSPage{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update page")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "CMS page not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Page updated"})
}

func AdminDeleteCMSPage(c *gin.Context) {
	id := c.Param("id")
	result := db.DB.Where("id = ?", id).Delete(&models.CMSPage{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete page")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "CMS page not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Page deleted"})
}
