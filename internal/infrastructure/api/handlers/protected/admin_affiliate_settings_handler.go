package protected

import (
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// AdminGetAffiliateSettings returns the singleton settings row (or sensible defaults).
func AdminGetAffiliateSettings(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	var settings models.AffiliateSetting
	err := database.Where("key = ?", "default").First(&settings).Error
	if err != nil {
		// Return defaults if not seeded yet
		settings = models.AffiliateSetting{
			Key:                   "default",
			DefaultCommissionRate: 10,
			DefaultTier:           "BRONZE",
			AutoApprove:           true,
			MinimumPayout:         100,
			HoldDays:              14,
			CookieDays:            30,
			AllowSelfReferral:     false,
			NotifyOnSignup:        true,
			NotifyOnPayout:        true,
		}
	}
	api_response.Success(c, settings)
}

// AdminUpdateAffiliateSettings upserts the singleton settings row.
func AdminUpdateAffiliateSettings(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		DefaultCommissionRate *float64 `json:"defaultCommissionRate"`
		DefaultTier           *string  `json:"defaultTier"`
		AutoApprove           *bool    `json:"autoApprove"`
		MinimumPayout         *float64 `json:"minimumPayout"`
		HoldDays              *int     `json:"holdDays"`
		CookieDays            *int     `json:"cookieDays"`
		AllowSelfReferral     *bool    `json:"allowSelfReferral"`
		EmailTemplateWelcome  *string  `json:"emailTemplateWelcome"`
		EmailTemplatePayout   *string  `json:"emailTemplatePayout"`
		NotifyOnSignup        *bool    `json:"notifyOnSignup"`
		NotifyOnPayout        *bool    `json:"notifyOnPayout"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if input.DefaultCommissionRate != nil {
		updates["default_commission_rate"] = *input.DefaultCommissionRate
	}
	if input.DefaultTier != nil {
		updates["default_tier"] = strings.ToUpper(*input.DefaultTier)
	}
	if input.AutoApprove != nil {
		updates["auto_approve"] = *input.AutoApprove
	}
	if input.MinimumPayout != nil {
		updates["minimum_payout"] = *input.MinimumPayout
	}
	if input.HoldDays != nil {
		updates["hold_days"] = *input.HoldDays
	}
	if input.CookieDays != nil {
		updates["cookie_days"] = *input.CookieDays
	}
	if input.AllowSelfReferral != nil {
		updates["allow_self_referral"] = *input.AllowSelfReferral
	}
	if input.EmailTemplateWelcome != nil {
		updates["email_template_welcome"] = *input.EmailTemplateWelcome
	}
	if input.EmailTemplatePayout != nil {
		updates["email_template_payout"] = *input.EmailTemplatePayout
	}
	if input.NotifyOnSignup != nil {
		updates["notify_on_signup"] = *input.NotifyOnSignup
	}
	if input.NotifyOnPayout != nil {
		updates["notify_on_payout"] = *input.NotifyOnPayout
	}

	actorID, _ := getAuthenticatedUserID(c)
	if actorID != "" {
		updates["updated_by"] = actorID
	}
	updates["updated_at"] = time.Now()

	if err := database.Model(&models.AffiliateSetting{}).
		Where("key = ?", "default").Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	var settings models.AffiliateSetting
	database.Where("key = ?", "default").First(&settings)
	LogAudit(c, "UPDATE", "affiliate_settings", settings.ID, updates)
	api_response.Success(c, settings)
}

// ---------------------------------------------------------------------------
// Tier rules CRUD
// ---------------------------------------------------------------------------

// AdminListAffiliateTiers returns all tier rules.
func AdminListAffiliateTiers(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	var tiers []models.AffiliateTierRule
	if err := database.Order("sort_order ASC, tier ASC").Find(&tiers).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch tiers")
		return
	}
	api_response.Success(c, tiers)
}

// AdminUpsertAffiliateTier creates or updates a tier rule by tier name.
func AdminUpsertAffiliateTier(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		Tier           string  `json:"tier" binding:"required"`
		NameAr         string  `json:"nameAr" binding:"required"`
		CommissionRate float64 `json:"commissionRate"`
		MinRevenue     float64 `json:"minRevenue"`
		MinReferrals   int     `json:"minReferrals"`
		BonusRate      float64 `json:"bonusRate"`
		Color          string  `json:"color"`
		SortOrder      int     `json:"sortOrder"`
		Active         *bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tier := strings.ToUpper(strings.TrimSpace(input.Tier))
	active := true
	if input.Active != nil {
		active = *input.Active
	}

	var existing models.AffiliateTierRule
	if err := database.Where("tier = ?", tier).First(&existing).Error; err == nil {
		updates := map[string]interface{}{
			"name_ar":          input.NameAr,
			"commission_rate":  input.CommissionRate,
			"min_revenue":      input.MinRevenue,
			"min_referrals":    input.MinReferrals,
			"bonus_rate":       input.BonusRate,
			"color":            firstNonEmpty(input.Color, existing.Color),
			"sort_order":       input.SortOrder,
			"active":           active,
		}
		if err := database.Model(&models.AffiliateTierRule{}).Where("tier = ?", tier).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update tier")
			return
		}
		database.Where("tier = ?", tier).First(&existing)
		LogAudit(c, "UPDATE", "affiliate_tier", existing.ID, updates)
		api_response.Success(c, existing)
		return
	}

	t := models.AffiliateTierRule{
		Tier:           tier,
		NameAr:         input.NameAr,
		CommissionRate: input.CommissionRate,
		MinRevenue:     input.MinRevenue,
		MinReferrals:   input.MinReferrals,
		BonusRate:      input.BonusRate,
		Color:          firstNonEmpty(input.Color, "amber"),
		SortOrder:      input.SortOrder,
		Active:         active,
	}
	if err := SafeCreate(database, &t); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create tier")
		return
	}
	LogAudit(c, "CREATE", "affiliate_tier", t.ID, t)
	api_response.Created(c, t)
}

// AdminDeleteAffiliateTier removes a tier rule.
func AdminDeleteAffiliateTier(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	if err := database.Where(queryID, id).Delete(&models.AffiliateTierRule{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete tier")
		return
	}
	LogAudit(c, "DELETE", "affiliate_tier", id, nil)
	api_response.Success(c, nil)
}