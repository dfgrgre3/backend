package protected

import (
	"fmt"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PurchasePlan handles purchasing a subscription plan using wallet balance
func PurchasePlan(c *gin.Context) {
	userId, _ := c.Get("userId")

	var req struct {
		PlanID     string `json:"planId" binding:"required"`
		CouponCode string `json:"couponCode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, errInvalidRequest)
		return
	}
	var plan models.SubscriptionPlan
	if err := db.DB.First(&plan, idQuery, req.PlanID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errPlanNotFound)
		return
	}

	paymentRef := generateSecureReference("PLAN")

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where(idQuery, userId).First(&user).Error; err != nil {
			return err
		}

		price, _ := plan.Price.Float64()
		finalPrice := calculateFinalPrice(tx, price, req.CouponCode)

		if err := subDeductUserBalance(tx, userId.(string), &user, finalPrice, fmt.Sprintf("شراء خطة اشتراك: %s", plan.Name), paymentRef); err != nil {
			return err
		}

		return createSubscriptionRecords(tx, userId.(string), plan, finalPrice, paymentRef)
	})

	if err != nil {
		subHandlePurchaseError(c, err)
		return
	}

	api_response.Success(c, gin.H{"success": true})
}

func calculateFinalPrice(tx *gorm.DB, originalPrice float64, couponCode string) float64 {
	if couponCode == "" {
		return originalPrice
	}

	// Lock the coupon row for the duration of this transaction. Without this,
	// two concurrent purchases using the same coupon could both read the same
	// UsedCount before either commits its increment, both pass the MaxUses
	// check, and both redeem it — letting a coupon be used more times than
	// MaxUses allows.
	var coupon models.Coupon
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ? AND "+isActiveQuery, couponCode, true).First(&coupon).Error; err != nil {
		return originalPrice
	}

	if !isCouponValid(coupon, originalPrice) {
		return originalPrice
	}

	finalPrice := originalPrice
	if coupon.DiscountType == "PERCENTAGE" {
		discountValue, _ := coupon.DiscountValue.Float64()
		finalPrice = originalPrice - (originalPrice * (discountValue / 100))
	} else {
		discountValue, _ := coupon.DiscountValue.Float64()
		finalPrice = originalPrice - discountValue
	}

	if finalPrice < 0 {
		finalPrice = 0
	}

	tx.Model(&coupon).Update("usedCount", gorm.Expr("\"usedCount\" + 1"))
	return finalPrice
}

func isCouponValid(coupon models.Coupon, amount float64) bool {
	if coupon.ExpiryDate != nil && coupon.ExpiryDate.Before(time.Now()) {
		return false
	}
	if coupon.MaxUses != nil && coupon.UsedCount >= *coupon.MaxUses {
		return false
	}
	minOrderAmount, _ := coupon.MinOrderAmount.Float64()
	if minOrderAmount > 0 && amount < minOrderAmount {
		return false
	}
	return true
}

func subDeductUserBalance(tx *gorm.DB, userID string, user *models.User, amount float64, description string, ref string) error {
	balance, _ := user.Balance.Float64()
	if balance < amount {
		return paymentservice.ErrInsufficientBalance
	}

	result := tx.Model(&models.User{}).
		Where("id = ? AND version = ?", userID, user.Version).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance - ?", amount),
			"version": user.Version + 1,
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return paymentservice.ErrOptimisticLock
	}

	walletTx := models.WalletTransaction{
		UserID:      userID,
		Type:        models.TxTypeWithdraw,
		Amount:      decimal.NewFromFloat(-amount),
		Currency:    "EGP",
		WalletType:  "BALANCE",
		Description: description,
		ReferenceID: &ref,
	}
	return tx.Create(&walletTx).Error
}

func subHandlePurchaseError(c *gin.Context, err error) {
	if err == paymentservice.ErrInsufficientBalance {
		api_response.Error(c, http.StatusBadRequest, "رصيدك غير كافٍ لإتمام هذه العملية")
		return
	}
	if err == paymentservice.ErrOptimisticLock {
		api_response.Error(c, http.StatusConflict, "يرجى المحاولة مرة أخرى")
		return
	}
	api_response.Error(c, http.StatusInternalServerError, "Failed to complete purchase")
}
