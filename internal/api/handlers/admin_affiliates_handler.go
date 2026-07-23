package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminGetAffiliates returns the full list of affiliates with user info.
func AdminGetAffiliates(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	var affiliates []models.Affiliate
	if err := database.Preload("User").Order("created_at DESC").Find(&affiliates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch affiliates")
		return
	}
	api_response.Success(c, affiliates)
}

// AdminCreateAffiliate creates a new affiliate record.
func AdminCreateAffiliate(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		UserID         string  `json:"userId" binding:"required"`
		Code           string  `json:"code"`
		Status         string  `json:"status"`
		CommissionRate float64 `json:"commissionRate"`
		Tier           string  `json:"tier"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Verify the user exists
	var user models.User
	if err := database.Where(queryID, input.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, errUserNotFound)
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify user")
		return
	}

	// Generate a referral code if not provided
	code := strings.TrimSpace(input.Code)
	if code == "" {
		code = generateAffiliateCode(user.Email)
	}

	// Check code uniqueness
	var existing models.Affiliate
	if err := database.Where("code = ?", code).First(&existing).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "Affiliate code already in use")
		return
	}

	affiliate := models.Affiliate{
		UserID:         input.UserID,
		Code:           code,
		Status:         firstNonEmpty(input.Status, "ACTIVE"),
		CommissionRate: input.CommissionRate,
		Tier:           firstNonEmpty(input.Tier, "BRONZE"),
	}

	if affiliate.CommissionRate == 0 {
		affiliate.CommissionRate = 10
	}

	if err := SafeCreate(database, &affiliate); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "Affiliate already exists for this user")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create affiliate")
		return
	}

	database.Preload("User").Where(queryID, affiliate.ID).First(&affiliate)
	LogAudit(c, "CREATE", "affiliate", affiliate.ID, affiliate)
	api_response.Created(c, affiliate)
}

// AdminGetAffiliate returns a single affiliate by id (with referrals stats).
func AdminGetAffiliate(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var affiliate models.Affiliate
	if err := database.Preload("User").Where(queryID, id).First(&affiliate).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Affiliate not found")
		return
	}

	var pendingCount int64
	database.Model(&models.AffiliateReferral{}).
		Where("affiliate_id = ? AND status = ?", id, "PENDING").
		Count(&pendingCount)

	api_response.Success(c, gin.H{
		"affiliate":     affiliate,
		"pendingCount": pendingCount,
	})
}

// AdminUpdateAffiliate updates an existing affiliate.
func AdminUpdateAffiliate(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var affiliate models.Affiliate
	if err := database.Where(queryID, id).First(&affiliate).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Affiliate not found")
		return
	}

	var input struct {
		Code           *string  `json:"code"`
		Status         *string  `json:"status"`
		CommissionRate *float64 `json:"commissionRate"`
		Tier           *string  `json:"tier"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if input.Code != nil {
		code := strings.TrimSpace(*input.Code)
		if code != "" {
			// check uniqueness (excluding current record)
			var count int64
			database.Model(&models.Affiliate{}).
				Where("code = ? AND id != ?", code, id).Count(&count)
			if count > 0 {
				api_response.Error(c, http.StatusConflict, "Affiliate code already in use")
				return
			}
			updates["code"] = code
		}
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.CommissionRate != nil {
		updates["commission_rate"] = *input.CommissionRate
	}
	if input.Tier != nil {
		updates["tier"] = *input.Tier
	}

	if len(updates) > 0 {
		if err := database.Model(&models.Affiliate{}).Where(queryID, id).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update affiliate")
			return
		}
	}

	database.Preload("User").Where(queryID, id).First(&affiliate)
	LogAudit(c, "UPDATE", "affiliate", id, updates)
	api_response.Success(c, affiliate)
}

// AdminDeleteAffiliate deletes an affiliate by id.
func AdminDeleteAffiliate(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	if err := database.Where(queryID, id).Delete(&models.Affiliate{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete affiliate")
		return
	}
	LogAudit(c, "DELETE", "affiliate", id, nil)
	api_response.Success(c, nil)
}

// AdminGetAffiliateReferrals lists all referral/commission records for an affiliate.
func AdminGetAffiliateReferrals(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var referrals []models.AffiliateReferral
	if err := database.Preload("User").Where("affiliate_id = ?", id).Order("created_at DESC").Find(&referrals).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch referrals")
		return
	}
	api_response.Success(c, referrals)
}

// AdminPayAffiliate pays out all pending commissions for an affiliate.
func AdminPayAffiliate(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var affiliate models.Affiliate
	if err := database.Where(queryID, id).First(&affiliate).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Affiliate not found")
		return
	}

	// Calculate total pending commissions
	var pending []models.AffiliateReferral
	if err := database.Where("affiliate_id = ? AND status = ?", id, "PENDING").Find(&pending).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pending referrals")
		return
	}

	if len(pending) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No pending commissions to pay")
		return
	}

	var totalPaid float64
	for _, r := range pending {
		totalPaid += r.Commission
	}

	// Mark all pending referrals as PAID
	now := time.Now()
	if err := database.Model(&models.AffiliateReferral{}).
		Where("affiliate_id = ? AND status = ?", id, "PENDING").
		Updates(map[string]interface{}{"status": "PAID", "updated_at": now}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update referrals")
		return
	}

	// Update affiliate totals
	newTotalPaid := affiliate.TotalPaid + totalPaid
	newTotalEarned := affiliate.TotalEarned
	if newTotalEarned < newTotalPaid {
		newTotalEarned = newTotalPaid
	}
	if err := database.Model(&models.Affiliate{}).Where(queryID, id).
		Updates(map[string]interface{}{"total_paid": newTotalPaid, "total_earned": newTotalEarned, "updated_at": now}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update affiliate totals")
		return
	}

	LogAudit(c, "PAY", "affiliate", id, gin.H{"paid": totalPaid, "count": len(pending)})
	api_response.Success(c, gin.H{
		"success": true,
		"paid":    totalPaid,
		"count":   len(pending),
	})
}

// generateAffiliateCode creates a unique referral code from a user's email/username.
func generateAffiliateCode(seed string) string {
	// Strip non-alphanumeric and uppercase
	prefix := strings.ToUpper(strings.ReplaceAll(strings.Split(seed, "@")[0], ".", ""))
	prefix = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, prefix)
	if len(prefix) > 6 {
		prefix = prefix[:6]
	}
	if len(prefix) < 3 {
		prefix = "AFF"
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%10000)
	return prefix + suffix
}
