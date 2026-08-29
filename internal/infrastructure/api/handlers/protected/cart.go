package protected

import (
	"fmt"
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetCart returns the authenticated user's cart items with subject data and
// the running total.
func GetCart(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var items []models.CartItem
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch cart")
		return
	}

	total := decimal.Zero
	for _, item := range items {
		total = total.Add(item.Subject.Price)
	}

	api_response.Success(c, gin.H{"items": items, "total": total})
}

// AddToCart adds a course to the authenticated user's cart. Duplicate adds
// are a no-op thanks to the unique (user_id, subject_id) index.
func AddToCart(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var body struct {
		SubjectID string `json:"subjectId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, body.SubjectID).First(&subject).Error; err != nil {
		handleSubjectError(c, body.SubjectID, err, "resolving subject for cart")
		return
	}

	if isAlreadyEnrolled(userId, subject.ID) {
		api_response.Error(c, http.StatusConflict, "أنت مسجّل بالفعل في هذه الدورة")
		return
	}

	item := models.CartItem{UserID: userId, SubjectID: subject.ID}
	if err := db.DB.WithContext(c.Request.Context()).
		Where("user_id = ? AND subject_id = ?", userId, subject.ID).
		FirstOrCreate(&item).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add to cart")
		return
	}

	api_response.Success(c, gin.H{"item": item})
}

// RemoveFromCart removes one course from the authenticated user's cart.
func RemoveFromCart(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	subjectId := c.Param("subjectId")
	if err := db.DB.WithContext(c.Request.Context()).
		Where("user_id = ? AND subject_id = ?", userId, subjectId).
		Delete(&models.CartItem{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove from cart")
		return
	}

	api_response.Success(c, gin.H{"removed": true})
}

// generateOrderNumber builds a human-readable order number, e.g.
// "ORD-20260827-A1B2C3".
func generateOrderNumber() string {
	ref := generateSecureReference("ORD")
	return ref
}

// CheckoutCart charges the authenticated user's full cart in one Order,
// either via the internal wallet (fully synchronous) or Paymob (redirect
// flow — items are enrolled once the payment webhook confirms it, mirroring
// the single-course checkout in subject_checkout.go).
func CheckoutCart(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var input struct {
		PaymentMethod string `json:"paymentMethod" binding:"required"`
		CouponCode    string `json:"couponCode"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	var allCartItems []models.CartItem
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", userId).
		Find(&allCartItems).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load cart")
		return
	}
	if len(allCartItems) == 0 {
		api_response.Error(c, http.StatusBadRequest, "السلة فارغة")
		return
	}

	// Defensive filter: a course could have been enrolled from elsewhere
	// (direct checkout, admin action) after it was added to the cart. Drop
	// those before charging so the student is never billed for a course
	// they already have — see AddToCart's own check for the common case.
	cartItems := make([]models.CartItem, 0, len(allCartItems))
	for _, item := range allCartItems {
		if !isAlreadyEnrolled(userId, item.SubjectID) {
			cartItems = append(cartItems, item)
		}
	}
	if len(cartItems) == 0 {
		api_response.Error(c, http.StatusConflict, "أنت مسجّل بالفعل في كل الدورات الموجودة بالسلة")
		return
	}

	rawTotal := decimal.Zero
	for _, item := range cartItems {
		rawTotal = rawTotal.Add(item.Subject.Price)
	}
	rawTotalFloat, _ := rawTotal.Float64()

	coupon, discountedTotal := applyCoupon(input.CouponCode, rawTotalFloat)
	finalTotal := decimal.NewFromFloat(discountedTotal)
	discountAmount := rawTotal.Sub(finalTotal)

	order := models.Order{
		OrderNumber:    generateOrderNumber(),
		UserID:         userId,
		Status:         models.OrderPending,
		Total:          finalTotal,
		PaymentMethod:  &input.PaymentMethod,
		DiscountAmount: discountAmount,
	}
	if coupon != nil {
		order.CouponCode = &coupon.Code
	}

	if input.PaymentMethod == "internal_wallet" {
		processCartWalletPayment(c, userId, cartItems, &order)
		return
	}

	if input.PaymentMethod == "card" || input.PaymentMethod == "wallet" || input.PaymentMethod == "fawry" {
		processCartPaymobPayment(c, userId, cartItems, &order, input.PaymentMethod)
		return
	}

	api_response.Error(c, http.StatusBadRequest, "Unsupported payment method")
}

// processCartWalletPayment charges the whole cart from the internal wallet
// balance, enrolls the user in every item, records the Order, and clears the
// cart — all inside one transaction (mirrors processInternalWalletPayment's
// row-locking pattern in subject_checkout.go, extended to multiple items).
func processCartWalletPayment(c *gin.Context, userId string, cartItems []models.CartItem, order *models.Order) {
	txErr := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, idQuery, userId).Error; err != nil {
			return err
		}

		balance, _ := user.Balance.Float64()
		total, _ := order.Total.Float64()
		if balance < total {
			return gorm.ErrInvalidData
		}

		if err := tx.Model(&user).Update("balance", gorm.Expr("balance - ?", order.Total)).Error; err != nil {
			return err
		}

		order.Status = models.OrderCompleted
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		for _, item := range cartItems {
			orderItem := models.OrderItem{
				OrderID:   order.ID,
				SubjectID: item.Subject.ID,
				Title:     item.Subject.Name,
				Type:      "COURSE",
				Price:     item.Subject.Price,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return err
			}

			payment := models.Payment{
				UserID:    userId,
				SubjectID: &item.SubjectID,
				Amount:    item.Subject.Price,
				Method:    "internal_wallet",
				Status:    models.PaymentCompleted,
				Reference: generateSecureReference("COURSE"),
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}

			if err := enrollUserInTransaction(tx, userId, item.SubjectID); err != nil {
				return err
			}
		}

		return tx.Where("user_id = ?", userId).Delete(&models.CartItem{}).Error
	})

	if txErr != nil {
		api_response.Error(c, http.StatusBadRequest, "رصيدك غير كافٍ")
		return
	}

	api_response.Success(c, gin.H{
		"success": true,
		"orderId": order.ID,
		"message": "Payment successful and enrolled",
	})
}

// processCartPaymobPayment registers one combined Paymob order for the
// whole cart. Enrollment for these items happens once the payment webhook
// confirms success (same as the single-course Paymob flow) — this handler
// only creates the pending Order/OrderItems and returns the payment key.
func processCartPaymobPayment(c *gin.Context, userId string, cartItems []models.CartItem, order *models.Order, method string) {
	paymobSvc := paymentservice.NewPaymobService()

	token, err := paymobSvc.Authenticate()
	if err != nil {
		log.Printf("Paymob Auth Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل الاتصال ببوابة الدفع")
		return
	}

	total, _ := order.Total.Float64()
	amountCents := int64(total * 100)

	items := make([]interface{}, 0, len(cartItems))
	for _, item := range cartItems {
		price, _ := item.Subject.Price.Float64()
		items = append(items, map[string]interface{}{
			"name":         item.Subject.Name,
			"amount_cents": int64(price * 100),
			"description":  fmt.Sprintf("Course: %s", item.Subject.Name),
			"quantity":     1,
		})
	}

	orderID, err := paymobSvc.RegisterOrder(token, amountCents, items)
	if err != nil {
		log.Printf("Paymob Order Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل إنشاء طلب الدفع")
		return
	}

	var user models.User
	db.DB.First(&user, idQuery, userId)
	billingData := getBillingData(user)
	integrationID := getIntegrationID(paymobSvc, method)

	paymentKey, err := paymobSvc.GetPaymentKey(token, orderID, amountCents, integrationID, billingData)
	if err != nil {
		log.Printf("Paymob Key Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل استخراج مفتاح الدفع")
		return
	}

	txErr := db.DB.Transaction(func(tx *gorm.DB) error {
		order.Status = models.OrderProcessing
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for _, item := range cartItems {
			orderItem := models.OrderItem{
				OrderID:   order.ID,
				SubjectID: item.Subject.ID,
				Title:     item.Subject.Name,
				Type:      "COURSE",
				Price:     item.Subject.Price,
			}
			if err := tx.Create(&orderItem).Error; err != nil {
				return err
			}
			price, _ := item.Subject.Price.Float64()
			payment := models.Payment{
				UserID:        userId,
				SubjectID:     &item.SubjectID,
				OrderID:       &order.ID,
				Amount:        decimal.NewFromFloat(price),
				Method:        method,
				Status:        models.PaymentPending,
				Reference:     generateSecureReference("COURSE"),
				PaymobOrderID: orderID,
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save order")
		return
	}

	if method == "wallet" {
		handleWalletRedirect(c, paymobSvc, paymentKey, billingData["phone_number"], orderID)
		return
	}

	api_response.Success(c, gin.H{
		"paymentKey": paymentKey,
		"iframeId":   paymobSvc.IframeID,
		"orderId":    orderID,
	})
}
