package protected

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// AdminListAffiliateCampaigns returns all affiliate marketing campaigns.
func AdminListAffiliateCampaigns(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))

	tx := database.Model(&models.AffiliateCampaign{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		tx = tx.Where("LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(COALESCE(promo_code, '')) LIKE ?", like, like, like)
	}

	var campaigns []models.AffiliateCampaign
	if err := tx.Order("created_at DESC").Find(&campaigns).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch campaigns")
		return
	}
	api_response.Success(c, campaigns)
}

// AdminCreateAffiliateCampaign creates a new affiliate campaign.
func AdminCreateAffiliateCampaign(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		Name           string     `json:"name" binding:"required"`
		Slug           string     `json:"slug"`
		Description    *string    `json:"description"`
		Status         string     `json:"status"`
		StartDate      *time.Time `json:"startDate"`
		EndDate        *time.Time `json:"endDate"`
		CommissionRate *float64   `json:"commissionRate"`
		Budget         *float64   `json:"budget"`
		BannerURL      *string    `json:"bannerUrl"`
		LandingURL     *string    `json:"landingUrl"`
		PromoCode      *string    `json:"promoCode"`
		Metadata       models.JSONMap `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	slug := strings.TrimSpace(strings.ToLower(input.Slug))
	if slug == "" {
		slug = slugify(input.Name)
	}
	if slug == "" {
		slug = fmt.Sprintf("camp-%d", time.Now().UnixNano()%100000)
	}

	var existing models.AffiliateCampaign
	if err := database.Where("slug = ?", slug).First(&existing).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "Campaign slug already in use")
		return
	}

	actorID, _ := getAuthenticatedUserID(c)
	campaign := models.AffiliateCampaign{
		Name:           strings.TrimSpace(input.Name),
		Slug:           slug,
		Description:    input.Description,
		Status:         firstNonEmpty(strings.ToUpper(input.Status), "DRAFT"),
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
		CommissionRate: input.CommissionRate,
		Budget:         input.Budget,
		BannerURL:      input.BannerURL,
		LandingURL:     input.LandingURL,
		PromoCode:      input.PromoCode,
		Metadata:       nilOrJSONMap(input.Metadata),
		CreatedBy:      strPtr(actorID),
	}

	if err := SafeCreate(database, &campaign); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "Campaign already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create campaign")
		return
	}

	LogAudit(c, "CREATE", "affiliate_campaign", campaign.ID, campaign)
	api_response.Created(c, campaign)
}

// AdminGetAffiliateCampaign returns a single campaign.
func AdminGetAffiliateCampaign(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var campaign models.AffiliateCampaign
	if err := database.Where(queryID, id).First(&campaign).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Campaign not found")
		return
	}

	// Aggregates
	var linksCount int64
	var clicksCount int64
	var conversionsCount int64
	database.Model(&models.AffiliateLink{}).Where("campaign_id = ?", id).Count(&linksCount)
	database.Model(&models.AffiliateLinkClick{}).Where("affiliate_id IN (SELECT id FROM \"Affiliate\" WHERE 1=1) AND link_id IN (SELECT id FROM \"AffiliateLink\" WHERE campaign_id = ?)", id).Count(&clicksCount)
	database.Model(&models.AffiliateLinkClick{}).Where("converted = ? AND link_id IN (SELECT id FROM \"AffiliateLink\" WHERE campaign_id = ?)", true, id).Count(&conversionsCount)

	api_response.Success(c, gin.H{
		"campaign":         campaign,
		"linksCount":       linksCount,
		"clicksCount":      clicksCount,
		"conversionsCount": conversionsCount,
	})
}

// AdminUpdateAffiliateCampaign updates an existing campaign.
func AdminUpdateAffiliateCampaign(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var campaign models.AffiliateCampaign
	if err := database.Where(queryID, id).First(&campaign).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Campaign not found")
		return
	}

	var input struct {
		Name           *string    `json:"name"`
		Slug           *string    `json:"slug"`
		Description    *string    `json:"description"`
		Status         *string    `json:"status"`
		StartDate      *time.Time `json:"startDate"`
		EndDate        *time.Time `json:"endDate"`
		CommissionRate *float64   `json:"commissionRate"`
		Budget         *float64   `json:"budget"`
		BannerURL      *string    `json:"bannerUrl"`
		LandingURL     *string    `json:"landingUrl"`
		PromoCode      *string    `json:"promoCode"`
		Spent          *float64   `json:"spent"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Slug != nil {
		slug := slugify(*input.Slug)
		if slug == "" {
			slug = slugify(*input.Name)
		}
		if slug != "" && slug != campaign.Slug {
			var count int64
			database.Model(&models.AffiliateCampaign{}).Where("slug = ? AND id != ?", slug, id).Count(&count)
			if count > 0 {
				api_response.Error(c, http.StatusConflict, "Campaign slug already in use")
				return
			}
			updates["slug"] = slug
		}
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Status != nil {
		updates["status"] = strings.ToUpper(*input.Status)
	}
	if input.StartDate != nil {
		updates["start_date"] = *input.StartDate
	}
	if input.EndDate != nil {
		updates["end_date"] = *input.EndDate
	}
	if input.CommissionRate != nil {
		updates["commission_rate"] = *input.CommissionRate
	}
	if input.Budget != nil {
		updates["budget"] = *input.Budget
	}
	if input.BannerURL != nil {
		updates["banner_url"] = *input.BannerURL
	}
	if input.LandingURL != nil {
		updates["landing_url"] = *input.LandingURL
	}
	if input.PromoCode != nil {
		updates["promo_code"] = *input.PromoCode
	}
	if input.Spent != nil {
		updates["spent"] = *input.Spent
	}

	if len(updates) > 0 {
		if err := database.Model(&models.AffiliateCampaign{}).Where(queryID, id).Updates(updates).Error; err != nil {
			if IsDuplicateKeyError(err) {
				api_response.Error(c, http.StatusConflict, "Campaign slug already in use")
				return
			}
			api_response.Error(c, http.StatusInternalServerError, "Failed to update campaign")
			return
		}
	}

	database.Where(queryID, id).First(&campaign)
	LogAudit(c, "UPDATE", "affiliate_campaign", id, updates)
	api_response.Success(c, campaign)
}

// AdminDeleteAffiliateCampaign deletes a campaign.
func AdminDeleteAffiliateCampaign(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	if err := database.Where(queryID, id).Delete(&models.AffiliateCampaign{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete campaign")
		return
	}
	LogAudit(c, "DELETE", "affiliate_campaign", id, nil)
	api_response.Success(c, nil)
}

// slugify normalises a string into a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// nilOrJSONMap returns nil if empty so gorm uses the default.
func nilOrJSONMap(m models.JSONMap) models.JSONMap {
	if len(m) == 0 {
		return models.JSONMap{}
	}
	return m
}