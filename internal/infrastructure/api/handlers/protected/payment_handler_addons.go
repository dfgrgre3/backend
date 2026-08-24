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
	"gorm.io/gorm"
)

func GetSubscriptionAddons(c *gin.Context) {
	addons := []gin.H{
		{
			"id":          "addon_ai_100",
			"name":        "100 AI Messages",
			"nameAr":      "100 رسالة ذكية إضافية",
			"description": "استمر في طرح الأسئلة على المساعد الذكي بكل حرية",
			"price":       50,
			"type":        "AI_CREDITS",
			"value":       100,
		},
		{
			"id":          "addon_exams_5",
			"name":        "5 Premium Exams",
			"nameAr":      "5 امتحانات متميزة إضافية",
			"description": "افتح الوصول إلى 5 امتحانات شاملة من اختيارك",
			"price":       75,
			"type":        "EXAM_PACK",
			"value":       5,
		},
		{
			"id":          "addon_balance_100",
			"name":        "100 EGP Wallet Balance",
			"nameAr":      "شحن 100 ج.م في المحفظة",
			"description": "أضف رصيداً لمحفظتك لاستخدامه لاحقاً في شراء الدورات",
			"price":       100,
			"type":        "WALLET_CREDIT",
			"value":       100,
		},
	}
	api_response.Success(c, gin.H{"addons": addons})
}

// addonPrices maps addon IDs to their prices for server-side validation
var addonPrices = map[string]float64{
	"addon_ai_100":      50,
	"addon_exams_5":     75,
	"addon_balance_100": 100,
}

func PurchaseAddon(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists || userId == nil {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var req struct {
		AddonID string `json:"addonId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	price, validAddon := addonPrices[req.AddonID]
	if !validAddon {
		api_response.Error(c, http.StatusBadRequest, "Invalid addon ID")
		return
	}

	paymentRef := generateSecureReference("ADDON")
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		user, err := getUserForPurchase(tx, userId.(string))
		if err != nil {
			return err
		}

		balance, _ := user.Balance.Float64()
		if balance < price {
			return paymentservice.ErrInsufficientBalance
		}

		if err := deductUserBalance(tx, user, price); err != nil {
			return err
		}

		if err := applyAddonCredits(tx, userId.(string), req.AddonID); err != nil {
			return err
		}

		return createAddonRecords(tx, userId.(string), req.AddonID, price, paymentRef)
	})

	if err != nil {
		handlePurchaseError(c, err)
		return
	}

	analyticsservice.GetAuditService().LogAsync(userId.(string), analyticsservice.AuditEventPaymentSuccess, "addon", req.AddonID, map[string]interface{}{"price": price}, c.ClientIP(), c.Request.UserAgent())
	api_response.Success(c, gin.H{"success": true})
}

func getUserForPurchase(tx *gorm.DB, userID string) (*models.User, error) {
	var user models.User
	if err := tx.Where(idQuery, userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func deductUserBalance(tx *gorm.DB, user *models.User, price float64) error {
	result := tx.Model(&models.User{}).
		Where("id = ? AND version = ?", user.ID, user.Version).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance - ?", price),
			"version": user.Version + 1,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return paymentservice.ErrOptimisticLock
	}
	return nil
}

func applyAddonCredits(tx *gorm.DB, userID string, addonID string) error {
	switch addonID {
	case "addon_ai_100":
		return tx.Model(&models.User{}).Where(idQuery, userID).
			Update("ai_credits", gorm.Expr("ai_credits + ?", 100)).Error
	case "addon_exams_5":
		return tx.Model(&models.User{}).Where(idQuery, userID).
			Update("exam_credits", gorm.Expr("exam_credits + ?", 5)).Error
	case "addon_balance_100":
		return tx.Model(&models.User{}).Where(idQuery, userID).
			Update("balance", gorm.Expr("balance + ?", 100)).Error
	default:
		return nil
	}
}

func createAddonRecords(tx *gorm.DB, userID string, addonID string, price float64, ref string) error {
	walletTx := models.WalletTransaction{
		UserID:      userID,
		Type:        models.TxTypeWithdraw,
		Amount:      decimal.NewFromFloat(-price),
		Currency:    "EGP",
		WalletType:  "BALANCE",
		Description: fmt.Sprintf("شراء إضافة: %s", addonID),
		ReferenceID: &ref,
	}
	if err := tx.Create(&walletTx).Error; err != nil {
		return err
	}

	payment := models.Payment{
		UserID:      userID,
		Amount:      decimal.NewFromFloat(price),
		Currency:    "EGP",
		Method:      "WALLET",
		Status:      models.PaymentCompleted,
		Reference:   ref,
		CompletedAt: time.Now(),
	}
	return tx.Create(&payment).Error
}

func handlePurchaseError(c *gin.Context, err error) {
	if err == paymentservice.ErrInsufficientBalance {
		api_response.Error(c, http.StatusBadRequest, "رصيدك غير كافٍ لإتمام هذه العملية")
		return
	}
	if err == paymentservice.ErrOptimisticLock {
		api_response.Error(c, http.StatusConflict, "يرجى المحاولة مرة أخرى")
		return
	}
	api_response.Error(c, http.StatusInternalServerError, "Failed to apply addon credits")
}
