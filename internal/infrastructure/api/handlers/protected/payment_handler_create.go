package protected

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type CreatePaymentRequest struct {
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	Method    string  `json:"method" binding:"required"`
	Currency  string  `json:"currency"`
	SubjectID *string `json:"subjectId"`
}

var allowedPaymentMethods = map[string]bool{
	"card":            true,
	"wallet":          true,
	"fawry":           true,
	"internal_wallet": true,
	"PAYMOB":          true,
	"WALLET":          true,
}

// generateSecureReference generates a cryptographically unique payment reference
func generateSecureReference(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(b))
}

func CreatePayment(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists || userId == nil {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Currency == "" {
		req.Currency = "EGP"
	}
	if !allowedPaymentMethods[req.Method] {
		api_response.Error(c, http.StatusBadRequest, "Unsupported payment method")
		return
	}

	// Validate amount bounds
	if req.Amount > 100000 {
		api_response.Error(c, http.StatusBadRequest, "Amount exceeds maximum allowed")
		return
	}
	if req.SubjectID != nil && *req.SubjectID != "" {
		var subject models.Subject
		if err := db.DB.Select("id", "price").First(&subject, idQuery, *req.SubjectID).Error; err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid subject")
			return
		}
		price, _ := subject.Price.Float64()
		if price > 0 && req.Amount != price {
			api_response.Error(c, http.StatusBadRequest, "Invalid payment amount")
			return
		}
	}

	payment := models.Payment{
		UserID:    userId.(string),
		SubjectID: req.SubjectID,
		Amount:    decimal.NewFromFloat(req.Amount),
		Currency:  req.Currency,
		Method:    req.Method,
		Status:    models.PaymentPending,
		Reference: generateSecureReference("REF"),
	}

	if err := SafeCreate(db.DB, &payment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create payment")
		return
	}

	analyticsservice.GetAuditService().LogAsync(userId.(string), analyticsservice.AuditEventPaymentStarted, "payment", payment.ID, map[string]interface{}{"amount": req.Amount, "method": req.Method}, c.ClientIP(), c.Request.UserAgent())

	api_response.Success(c, payment)
}

func GetPaymentHistory(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists || userId == nil {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	var payments []models.Payment

	if err := db.DB.Where("user_id = ?", userId).Order("created_at desc").Limit(100).Find(&payments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payments")
		return
	}

	api_response.Success(c, payments)
}
