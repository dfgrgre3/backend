package protected

import (
	"fmt"
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func HandleWalletDeposit(c *gin.Context) {
	adminUserId, exists := c.Get("userId")
	if !exists || adminUserId == nil {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
		// TargetUserID lets an admin credit a specific user's wallet (refunds,
		// support cases, manual top-ups). If omitted, the deposit applies to
		// the calling admin's own wallet, preserving the previous behavior.
		TargetUserID string `json:"targetUserId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid amount")
		return
	}

	targetUserID := req.TargetUserID
	if targetUserID == "" {
		targetUserID = adminUserId.(string)
	} else {
		var targetExists models.User
		if err := db.DB.Select("id").First(&targetExists, idQuery, targetUserID).Error; err != nil {
			api_response.Error(c, http.StatusBadRequest, "Target user not found")
			return
		}
	}

	// Use centralized wallet service with optimistic locking
	_, err := paymentservice.ProcessWalletTransaction(
		targetUserID,
		req.Amount,
		models.TxTypeDeposit,
		"BALANCE",
		"إيداع رصيد في المحفظة",
		nil,
	)

	if err != nil {
		if err == paymentservice.ErrOptimisticLock {
			api_response.Error(c, http.StatusConflict, "يرجى المحاولة مرة أخرى")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to update wallet")
		return
	}

	// Create payment record for audit trail
	payment := models.Payment{
		UserID:      targetUserID,
		Amount:      decimal.NewFromFloat(req.Amount),
		Currency:    "EGP",
		Method:      "WALLET_TOPUP",
		Status:      models.PaymentCompleted,
		Reference:   generateSecureReference("TOPUP"),
		CompletedAt: time.Now(),
	}
	if err := SafeCreate(db.DB, &payment); err != nil {
		// Log but don't fail — the deposit itself succeeded
		fmt.Printf("Warning: failed to create payment audit record for user %s: %v\n", targetUserID, err)
	} else {
		analyticsservice.GetAuditService().LogAsync(targetUserID, analyticsservice.AuditEventPaymentSuccess, "wallet_topup", payment.ID, map[string]interface{}{"amount": req.Amount, "creditedBy": adminUserId}, c.ClientIP(), c.Request.UserAgent())
	}
	LogAudit(c, "CREATE", "wallet_topup", payment.ID, gin.H{"targetUserId": targetUserID, "amount": req.Amount})

	// Reload user for fresh balance
	var user models.User
	db.DB.First(&user, idQuery, targetUserID)

	api_response.Success(c, gin.H{
		"success": true,
		"balance": user.Balance,
		"message": "تم شحن الرصيد بنجاح",
	})
}
