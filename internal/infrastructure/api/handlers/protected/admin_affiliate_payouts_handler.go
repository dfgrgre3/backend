package protected

import (
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// AdminListAffiliatePayouts returns the history of payouts (optionally filtered).
func AdminListAffiliatePayouts(c *gin.Context) {
	database, abort := safeReadDB(c)
	if abort {
		return
	}

	affiliateID := strings.TrimSpace(c.Query("affiliateId"))
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))

	tx := database.Model(&models.AffiliatePayout{}).Preload("Affiliate.User")
	if affiliateID != "" {
		tx = tx.Where("affiliate_id = ?", affiliateID)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}

	var payouts []models.AffiliatePayout
	if err := tx.Order("created_at DESC").Find(&payouts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payouts")
		return
	}
	api_response.Success(c, payouts)
}

// AdminCreateAffiliatePayout creates a payout record manually (optional).
func AdminCreateAffiliatePayout(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	var input struct {
		AffiliateID string   `json:"affiliateId" binding:"required"`
		Amount      float64  `json:"amount" binding:"required"`
		Currency    string   `json:"currency"`
		Method      *string  `json:"method"`
		Reference   *string  `json:"reference"`
		Notes       *string  `json:"notes"`
		ReferralIDs []string `json:"referralIds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var affiliate models.Affiliate
	if err := database.Where(queryID, input.AffiliateID).First(&affiliate).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Affiliate not found")
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "EGP"
	}

	actorID, _ := getAuthenticatedUserID(c)
	payout := models.AffiliatePayout{
		AffiliateID: input.AffiliateID,
		Amount:      input.Amount,
		Currency:    currency,
		Status:      "PENDING",
		Method:      input.Method,
		Reference:   input.Reference,
		Notes:       input.Notes,
		ReferralIDs: models.JSONArray(input.ReferralIDs),
	}
	if actorID != "" {
		payout.ProcessedBy = &actorID
	}

	if err := SafeCreate(database, &payout); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create payout")
		return
	}

	database.Preload("Affiliate.User").Where(queryID, payout.ID).First(&payout)
	LogAudit(c, "CREATE", "affiliate_payout", payout.ID, payout)
	api_response.Created(c, payout)
}

// AdminMarkAffiliatePayoutPaid marks a payout as PAID (or FAILED / CANCELLED).
func AdminMarkAffiliatePayoutPaid(c *gin.Context) {
	database, abort := safeDB(c)
	if abort {
		return
	}

	id := c.Param("id")
	var payout models.AffiliatePayout
	if err := database.Preload("Affiliate").Where(queryID, id).First(&payout).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Payout not found")
		return
	}

	var input struct {
		Status    string  `json:"status" binding:"required"`
		Reference *string `json:"reference"`
		Notes     *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	status := strings.ToUpper(strings.TrimSpace(input.Status))
	switch status {
	case "PAID", "FAILED", "CANCELLED", "PROCESSING":
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid payout status")
		return
	}

	actorID, _ := getAuthenticatedUserID(c)
	now := time.Now()
	updates := map[string]interface{}{
		"status":      status,
		"processed_at": now,
	}
	if actorID != "" {
		updates["processed_by"] = actorID
	}
	if input.Reference != nil {
		updates["reference"] = *input.Reference
	}
	if input.Notes != nil {
		updates["notes"] = *input.Notes
	}

	if err := database.Model(&models.AffiliatePayout{}).Where(queryID, id).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update payout")
		return
	}

	// If marked PAID, update affiliate totals
	if status == "PAID" {
		newTotalPaid := payout.Affiliate.TotalPaid + payout.Amount
		newTotalEarned := payout.Affiliate.TotalEarned
		if newTotalEarned < newTotalPaid {
			newTotalEarned = newTotalPaid
		}
		database.Model(&models.Affiliate{}).Where(queryID, payout.AffiliateID).
			Updates(map[string]interface{}{
				"total_paid":   newTotalPaid,
				"total_earned": newTotalEarned,
				"updated_at":   now,
			})
	}

	database.Preload("Affiliate.User").Where(queryID, id).First(&payout)
	LogAudit(c, "PAYOUT_"+status, "affiliate_payout", id, updates)
	api_response.Success(c, payout)
}

// AdminProcessAffiliatePayouts runs the per-affiliate pay action from the legacy endpoint but records a payout.
func AdminProcessAffiliatePayouts(c *gin.Context) {
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

	var pending []models.AffiliateReferral
	if err := database.Where("affiliate_id = ? AND status = ?", id, "PENDING").Find(&pending).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pending referrals")
		return
	}
	if len(pending) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No pending commissions to pay")
		return
	}

	var total float64
	referralIDs := make([]string, 0, len(pending))
	for _, r := range pending {
		total += r.Commission
		referralIDs = append(referralIDs, r.ID)
	}

	now := time.Now()
	actorID, _ := getAuthenticatedUserID(c)
	payout := models.AffiliatePayout{
		AffiliateID: id,
		Amount:      total,
		Currency:    "EGP",
		Status:      "PAID",
		Method:      affiliate.PayoutMethod,
		ReferralIDs: models.JSONArray(referralIDs),
	}
	if actorID != "" {
		payout.ProcessedBy = &actorID
	}
	payout.ProcessedAt = &now

	if err := SafeCreate(database, &payout); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create payout record")
		return
	}

	database.Model(&models.AffiliateReferral{}).
		Where("affiliate_id = ? AND status = ?", id, "PENDING").
		Updates(map[string]interface{}{"status": "PAID", "updated_at": now})

	newTotalPaid := affiliate.TotalPaid + total
	newTotalEarned := affiliate.TotalEarned
	if newTotalEarned < newTotalPaid {
		newTotalEarned = newTotalPaid
	}
	database.Model(&models.Affiliate{}).Where(queryID, id).
		Updates(map[string]interface{}{
			"total_paid":   newTotalPaid,
			"total_earned": newTotalEarned,
			"updated_at":   now,
		})

	LogAudit(c, "PAYOUT_PROCESS", "affiliate_payout", payout.ID, gin.H{
		"affiliateId": id,
		"amount":      total,
		"count":       len(pending),
	})
	api_response.Success(c, gin.H{
		"success": true,
		"paid":    total,
		"count":   len(pending),
		"payout":  payout,
	})
}