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
)

const errInvalidRequest = "Invalid request"
const errPlanNotFound = "Plan not found"

func GetSubscriptionPlans(c *gin.Context) {
	var plans []models.SubscriptionPlan
	if err := db.DB.Where(isActiveQuery, true).Order("price asc").Find(&plans).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch plans")
		return
	}
	api_response.Success(c, plans)
}

func GetUserSubscription(c *gin.Context) {
	userId, _ := c.Get("userId")

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	if user.ActiveSubscriptionID == nil {
		api_response.Success(c, gin.H{"active": false})
		return
	}

	var sub models.UserSubscription
	if err := db.DB.Preload("Plan").First(&sub, idQuery, *user.ActiveSubscriptionID).Error; err != nil {
		api_response.Success(c, gin.H{"active": false})
		return
	}

	api_response.Success(c, gin.H{
		"active":       true,
		"subscription": sub,
	})
}

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

	var coupon models.Coupon
	if err := tx.Where("code = ? AND "+isActiveQuery, couponCode, true).First(&coupon).Error; err != nil {
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

func createSubscriptionRecords(tx *gorm.DB, userID string, plan models.SubscriptionPlan, amount float64, ref string) error {
	paymentID, err := createSubscriptionAndPayment(tx, userID, plan, amount, ref)
	if err != nil {
		return err
	}

	invoice := models.Invoice{
		PaymentID:     paymentID,
		UserID:        userID,
		InvoiceNumber: "INV-" + time.Now().Format("20060102") + "-" + paymentID[:8],
	}
	return tx.Create(&invoice).Error
}

func createSubscriptionAndPayment(tx *gorm.DB, userID string, plan models.SubscriptionPlan, amount float64, ref string) (string, error) {
	endDate := calculateEndDate(plan.Interval)
	startDate := time.Now()

	sub := models.UserSubscription{
		UserID:    userID,
		PlanID:    plan.ID,
		Status:    models.SubscriptionActive,
		StartDate: startDate,
		EndDate:   endDate,
	}
	if err := tx.Create(&sub).Error; err != nil {
		return "", err
	}

	if err := tx.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"activeSubscriptionId":  sub.ID,
		"subscriptionExpiresAt": endDate,
	}).Error; err != nil {
		return "", err
	}

	payment := models.Payment{
		UserID:      userID,
		PlanID:      plan.ID,
		Amount:      decimal.NewFromFloat(amount),
		Currency:    plan.Currency,
		Method:      "WALLET",
		Status:      models.PaymentCompleted,
		Reference:   ref,
		CompletedAt: time.Now(),
	}
	if err := tx.Create(&payment).Error; err != nil {
		return "", err
	}
	return payment.ID, nil
}

func calculateEndDate(interval models.SubscriptionInterval) time.Time {
	duration := 30 * 24 * time.Hour
	switch interval {
	case models.IntervalYearly:
		duration = 365 * 24 * time.Hour
	case models.IntervalForever:
		duration = 100 * 365 * 24 * time.Hour
	}
	return time.Now().Add(duration)
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

// GetInvoice returns invoice data for a specific payment
func GetInvoice(c *gin.Context) {
	userId, _ := c.Get("userId")

	invoiceID := c.Param("id")
	if invoiceID == "" {
		invoiceID = c.Query("id")
	}

	var invoice models.Invoice
	if err := db.DB.Preload("Payment").Preload("Payment.Plan").Where("user_id = ?", userId).First(&invoice, idQuery, invoiceID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Invoice not found")
		return
	}

	api_response.Success(c, invoice)
}

// InitiatePlanPayment initiates a Paymob payment for a subscription plan
func InitiatePlanPayment(c *gin.Context) {
	userId, _ := c.Get("userId")

	var req struct {
		PlanID        string `json:"planId" binding:"required"`
		PaymentMethod string `json:"paymentMethod" binding:"required"` // "card", "wallet", "fawry"
		PhoneNumber   string `json:"phoneNumber"`                      // Required for wallet payments
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

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Get Paymob service
	paymob := paymentservice.NewPaymobService()

	// Authenticate with Paymob
	authToken, err := paymob.Authenticate()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to authenticate with payment provider")
		return
	}

	// Calculate amount in cents (Paymob uses cents)
	price, _ := plan.Price.Float64()
	amountCents := int64(price * 100)

	// Register order with Paymob
	orderID, err := paymob.RegisterOrder(authToken, amountCents, nil)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to register order")
		return
	}

	// Determine integration ID based on payment method
	var integrationID string
	switch req.PaymentMethod {
	case "card":
		integrationID = paymob.CardIntegrationID
	case "wallet":
		integrationID = paymob.WalletIntegrationID
	case "fawry":
		integrationID = paymob.FawryIntegrationID
	default:
		api_response.Error(c, http.StatusBadRequest, "Invalid payment method")
		return
	}

	// Prepare billing data
	firstName := "User"
	if user.Name != nil && *user.Name != "" {
		firstName = *user.Name
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}
	billingData := map[string]string{
		"first_name":   firstName,
		"last_name":    "User",
		"email":        user.Email,
		"phone_number": phone,
	}

	// Get payment key
	paymentKey, err := paymob.GetPaymentKey(authToken, orderID, amountCents, integrationID, billingData)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to generate payment key")
		return
	}

	// Create payment record in pending state
	payment := models.Payment{
		UserID:        user.ID,
		PlanID:        plan.ID,
		Amount:        plan.Price,
		Currency:      plan.Currency,
		Method:        "PAYMOB_" + req.PaymentMethod,
		Status:        models.PaymentPending,
		Reference:     generateSecureReference("PLAN"),
		PaymobOrderID: orderID,
	}
	if err := SafeCreate(db.DB, &payment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create payment record")
		return
	}

	// For wallet payments, create wallet request
	if req.PaymentMethod == "wallet" && req.PhoneNumber != "" {
		redirectURL, err := paymob.CreateWalletRequest(paymentKey, req.PhoneNumber)
		if err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to create wallet request")
			return
		}
		api_response.Success(c, gin.H{
			"success":     true,
			"paymentKey":  paymentKey,
			"redirectUrl": redirectURL,
			"paymentId":   payment.ID,
			"orderId":     orderID,
		})
		return
	}

	// For card payments, return iframe URL
	iframeURL := fmt.Sprintf("https://accept.paymob.com/api/acceptance/iframes/%s?payment_token=%s", paymob.IframeID, paymentKey)
	api_response.Success(c, gin.H{
		"success":    true,
		"paymentKey": paymentKey,
		"iframeUrl":  iframeURL,
		"paymentId":  payment.ID,
		"orderId":    orderID,
	})
}

// CancelSubscription cancels the user's active subscription
func CancelSubscription(c *gin.Context) {
	userId, _ := c.Get("userId")

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	if user.ActiveSubscriptionID == nil {
		api_response.Error(c, http.StatusBadRequest, "No active subscription to cancel")
		return
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Update subscription status to cancelled
		if err := tx.Model(&models.UserSubscription{}).
			Where(idQuery, *user.ActiveSubscriptionID).
			Update("status", models.SubscriptionCancelled).Error; err != nil {
			return err
		}

		// Clear user's active subscription
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"activeSubscriptionId":  nil,
			"subscriptionExpiresAt": nil,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to cancel subscription")
		return
	}

	api_response.Success(c, gin.H{"success": true, "message": "Subscription cancelled successfully"})
}

func RenewSubscription(c *gin.Context) {
	userId, _ := c.Get("userId")

	var req struct {
		PlanID string `json:"planId" binding:"required"`
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

	paymentRef := generateSecureReference("RENEW")
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where(idQuery, userId).First(&user).Error; err != nil {
			return err
		}

		price, _ := plan.Price.Float64()
		if err := subDeductUserBalance(tx, userId.(string), &user, price, fmt.Sprintf("تجديد اشتراك: %s", plan.Name), paymentRef); err != nil {
			return err
		}

		_, err := createSubscriptionAndPayment(tx, userId.(string), plan, price, paymentRef)
		return err
	})

	if err != nil {
		subHandlePurchaseError(c, err)
		return
	}

	api_response.Success(c, gin.H{"success": true, "message": "Subscription renewed successfully"})
}
