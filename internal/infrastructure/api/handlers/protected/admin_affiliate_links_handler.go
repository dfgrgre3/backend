package protected

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminListAffiliateLinks returns all tracked affiliate links (optionally filtered by affiliate / campaign).
func AdminListAffiliateLinks(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	affiliateID := strings.TrimSpace(c.Query("affiliateId"))
	campaignID := strings.TrimSpace(c.Query("campaignId"))
	q := strings.TrimSpace(c.Query("q"))

	tx := database.Model(&models.AffiliateLink{}).Preload("Affiliate").Preload("Campaign")
	if affiliateID != "" {
		tx = tx.Where("affiliate_id = ?", affiliateID)
	}
	if campaignID != "" {
		tx = tx.Where("campaign_id = ?", campaignID)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		tx = tx.Where("LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(destination_url) LIKE ?", like, like, like)
	}

	var links []models.AffiliateLink
	if err := tx.Order("created_at DESC").Find(&links).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch links")
		return
	}
	api_response.Success(c, links)
}

// AdminCreateAffiliateLink creates a new tracked link for an affiliate.
func AdminCreateAffiliateLink(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		AffiliateID    string  `json:"affiliateId" binding:"required"`
		CampaignID     *string `json:"campaignId"`
		Name           string  `json:"name" binding:"required"`
		Slug           string  `json:"slug"`
		DestinationURL string  `json:"destinationUrl" binding:"required"`
		UTMSource      *string `json:"utmSource"`
		UTMMedium      *string `json:"utmMedium"`
		UTMCampaign    *string `json:"utmCampaign"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Verify affiliate exists
	var affiliate models.Affiliate
	if err := database.Where(queryID, input.AffiliateID).First(&affiliate).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Build slug
	slug := slugify(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	if slug == "" {
		slug = fmt.Sprintf("lnk-%d", time.Now().UnixNano()%1000000)
	}

	// Uniqueness check
	var existing models.AffiliateLink
	if err := database.Where("slug = ?", slug).First(&existing).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "Link slug already in use")
		return
	}

	link := models.AffiliateLink{
		AffiliateID:    input.AffiliateID,
		CampaignID:     input.CampaignID,
		Name:           strings.TrimSpace(input.Name),
		Slug:           slug,
		DestinationURL: input.DestinationURL,
		UTMSource:      input.UTMSource,
		UTMMedium:      input.UTMMedium,
		UTMCampaign:    input.UTMCampaign,
		Active:         true,
	}

	if err := SafeCreate(database, &link); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "Link slug already in use")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create link")
		return
	}

	database.Preload("Affiliate").Preload("Campaign").Where(queryID, link.ID).First(&link)
	LogAudit(c, "CREATE", "affiliate_link", link.ID, link)
	api_response.Created(c, link)
}

// AdminGetAffiliateLink returns one link with its recent clicks.
func AdminGetAffiliateLink(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var link models.AffiliateLink
	if err := database.Preload("Affiliate").Preload("Campaign").Where(queryID, id).First(&link).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Link not found")
		return
	}

	var clicks []models.AffiliateLinkClick
	if err := database.Where("link_id = ?", id).Order("created_at DESC").Limit(50).Find(&clicks).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch clicks")
		return
	}
	api_response.Success(c, gin.H{"link": link, "clicks": clicks})
}

// AdminUpdateAffiliateLink updates a link.
func AdminUpdateAffiliateLink(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var link models.AffiliateLink
	if err := database.Where(queryID, id).First(&link).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Link not found")
		return
	}

	var input struct {
		Name           *string `json:"name"`
		DestinationURL *string `json:"destinationUrl"`
		UTMSource      *string `json:"utmSource"`
		UTMMedium      *string `json:"utmMedium"`
		UTMCampaign    *string `json:"utmCampaign"`
		Active         *bool   `json:"active"`
		CampaignID     *string `json:"campaignId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = strings.TrimSpace(*input.Name)
	}
	if input.DestinationURL != nil {
		updates["destination_url"] = *input.DestinationURL
	}
	if input.UTMSource != nil {
		updates["utm_source"] = *input.UTMSource
	}
	if input.UTMMedium != nil {
		updates["utm_medium"] = *input.UTMMedium
	}
	if input.UTMCampaign != nil {
		updates["utm_campaign"] = *input.UTMCampaign
	}
	if input.Active != nil {
		updates["active"] = *input.Active
	}
	if input.CampaignID != nil {
		if *input.CampaignID == "" {
			updates["campaign_id"] = nil
		} else {
			updates["campaign_id"] = *input.CampaignID
		}
	}

	if len(updates) > 0 {
		if err := database.Model(&models.AffiliateLink{}).Where(queryID, id).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update link")
			return
		}
	}

	database.Preload("Affiliate").Preload("Campaign").Where(queryID, id).First(&link)
	LogAudit(c, "UPDATE", "affiliate_link", id, updates)
	api_response.Success(c, link)
}

// AdminDeleteAffiliateLink deletes a link.
func AdminDeleteAffiliateLink(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	if err := database.Where(queryID, id).Delete(&models.AffiliateLink{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete link")
		return
	}
	LogAudit(c, "DELETE", "affiliate_link", id, nil)
	api_response.Success(c, nil)
}

// AdminTrackAffiliateClick records an outbound click for analytics.
func AdminTrackAffiliateClick(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var link models.AffiliateLink
	if err := database.Where(queryID, id).First(&link).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Link not found")
		return
	}

	var input struct {
		IPHash    *string `json:"ipHash"`
		UserAgent *string `json:"userAgent"`
		Referer   *string `json:"referer"`
		Country   *string `json:"country"`
		Device    *string `json:"device"`
		Converted *bool   `json:"converted"`
	}
	_ = c.ShouldBindJSON(&input)

	click := models.AffiliateLinkClick{
		LinkID:      link.ID,
		AffiliateID: link.AffiliateID,
		IPHash:      input.IPHash,
		UserAgent:   input.UserAgent,
		Referer:     input.Referer,
		Country:     input.Country,
		Device:      input.Device,
		Converted:   false,
	}
	if input.Converted != nil {
		click.Converted = *input.Converted
	}
	if err := SafeCreate(database, &click); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to record click")
		return
	}

	// Update counters on the link
	updates := map[string]interface{}{"clicks_count": gorm.Expr("clicks_count + 1")}
	if click.Converted {
		updates["conversions_count"] = gorm.Expr("conversions_count + 1")
	}
	database.Model(&models.AffiliateLink{}).Where(queryID, id).Updates(updates)

	// And on the affiliate
	affUpdates := map[string]interface{}{"clicks_count": gorm.Expr("clicks_count + 1")}
	if click.Converted {
		affUpdates["conversions_count"] = gorm.Expr("conversions_count + 1")
	}
	database.Model(&models.Affiliate{}).Where(queryID, link.AffiliateID).Updates(affUpdates)

	api_response.Created(c, click)
}