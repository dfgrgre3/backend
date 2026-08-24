package protected

import (
	"fmt"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

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
