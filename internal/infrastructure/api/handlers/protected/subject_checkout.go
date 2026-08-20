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

func CourseCheckout(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	courseId := c.Param("id")

	var input struct {
		PaymentMethod string `json:"paymentMethod" binding:"required"`
		CouponCode    string `json:"couponCode"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, courseId).First(&subject).Error; err != nil {
		handleSubjectError(c, courseId, err, "fetching course for checkout")
		return
	}

	if input.PaymentMethod == "internal_wallet" {
		processInternalWalletPayment(c, userId, courseId, subject)
		return
	}

	if input.PaymentMethod == "card" || input.PaymentMethod == "wallet" || input.PaymentMethod == "fawry" {
		processPaymobPayment(c, userId, courseId, subject, input.PaymentMethod)
		return
	}

	api_response.Error(c, http.StatusBadRequest, "Unsupported payment method")
}

func processInternalWalletPayment(c *gin.Context, userId string, courseId string, subject models.Subject) {
	payment := models.Payment{
		UserID:    userId,
		SubjectID: &courseId,
		Amount:    subject.Price,
		Method:    "internal_wallet",
		Status:    models.PaymentPending,
		Reference: generateSecureReference("COURSE"),
	}

	txErr := db.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, idQuery, userId).Error; err != nil {
			return err
		}

		balance, _ := user.Balance.Float64()
		price, _ := subject.Price.Float64()
		if balance < price {
			return gorm.ErrInvalidData // Insufficient balance
		}

		if err := tx.Model(&user).Update("balance", gorm.Expr("balance - ?", subject.Price)).Error; err != nil {
			return err
		}

		payment.Status = models.PaymentCompleted
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		return enrollUserInTransaction(tx, userId, courseId)
	})

	if txErr != nil {
		api_response.Error(c, http.StatusBadRequest, "رصيدك غير كافٍ")
		return
	}

	api_response.Success(c, gin.H{
		"success": true,
		"message": "Payment successful and enrolled",
	})
}

func processPaymobPayment(c *gin.Context, userId string, courseId string, subject models.Subject, method string) {
	paymobSvc := paymentservice.NewPaymobService()

	token, err := paymobSvc.Authenticate()
	if err != nil {
		log.Printf("Paymob Auth Error: %v", err)
		api_response.Error(c, http.StatusInternalServerError, "فشل الاتصال ببوابة الدفع")
		return
	}

	price, _ := subject.Price.Float64()
	amountCents := int64(price * 100)
	orderID, err := paymobSvc.RegisterOrder(token, amountCents, []interface{}{
		map[string]interface{}{
			"name":         subject.Name,
			"amount_cents": amountCents,
			"description":  fmt.Sprintf("Course: %s", subject.Name),
			"quantity":     1,
		},
	})
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

	if err := createPendingPayment(userId, courseId, price, method, orderID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save payment record")
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

func getBillingData(user models.User) map[string]string {
	billingData := map[string]string{
		"first_name":   "Student",
		"last_name":    "User",
		"email":        user.Email,
		"phone_number": "01000000000",
	}
	if user.Name != nil && *user.Name != "" {
		billingData["first_name"] = *user.Name
	}
	if user.Phone != nil && *user.Phone != "" {
		billingData["phone_number"] = *user.Phone
	}
	return billingData
}

func getIntegrationID(svc *paymentservice.PaymobService, method string) string {
	switch method {
	case "wallet":
		return svc.WalletIntegrationID
	case "fawry":
		return svc.FawryIntegrationID
	default:
		return svc.CardIntegrationID
	}
}

func createPendingPayment(userId, courseId string, amount float64, method string, orderID int64) error {
	payment := models.Payment{
		UserID:        userId,
		SubjectID:     &courseId,
		Amount:        decimal.NewFromFloat(amount),
		Method:        method,
		Status:        models.PaymentPending,
		Reference:     generateSecureReference("COURSE"),
		PaymobOrderID: orderID,
	}
	return SafeCreate(db.DB, &payment)
}

func handleWalletRedirect(c *gin.Context, svc *paymentservice.PaymobService, paymentKey, phone string, orderID int64) {
	walletUrl, err := svc.CreateWalletRequest(paymentKey, phone)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "فشل معالجة طلب المحفظة")
		return
	}
	api_response.Success(c, gin.H{
		"redirectUrl": walletUrl,
		"orderId":     orderID,
	})
}
