package protected

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	paymentservice "thanawy-backend/internal/domain/payment/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Paymob Callback Handler
func PaymobWebhook(c *gin.Context) {
	var payload map[string]interface{}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	paymobSvc := paymentservice.NewPaymobService()
	if !paymobSvc.VerifyHMAC(payload) {
		log.Printf("[PaymobWebhook] HMAC verification failed (possible spoofed callback), ip=%s", c.ClientIP())
		api_response.Error(c, http.StatusForbidden, "Invalid signature")
		return
	}

	data := extractPaymobTransactionData(payload)
	if data.Pending {
		return
	}

	var payment models.Payment
	if err := db.DB.Where("\"paymobOrderId\" = ?", data.OrderID).First(&payment).Error; err != nil {
		log.Printf("[PaymobWebhook] payment record not found for order=%d", data.OrderID)
		api_response.Success(c, gin.H{"status": "ignored"})
		return
	}

	// Idempotency guard: Paymob may retry webhook delivery, and a captured
	// valid-signature payload could be replayed. Without this check, a
	// duplicate "success" callback would re-run processPaymentItems and
	// double-credit the wallet / re-enroll / re-create a subscription for a
	// payment that was already completed.
	if payment.Status == models.PaymentCompleted || payment.Status == models.PaymentFailed {
		log.Printf("[PaymobWebhook] ignoring duplicate callback for already-%s payment id=%s order=%d", payment.Status, payment.ID, data.OrderID)
		api_response.Success(c, gin.H{"status": "already_processed"})
		return
	}

	if data.Success {
		handleSuccessfulPayment(c, &payment, data)
	} else {
		handleFailedPayment(&payment, data.OrderID)
	}

	api_response.Success(c, gin.H{"status": "received"})
}

type paymobTransactionData struct {
	Success bool
	Pending bool
	OrderID int64
	TxnID   int64
}

func extractPaymobTransactionData(payload map[string]interface{}) paymobTransactionData {
	obj, ok := payload["obj"].(map[string]interface{})
	if !ok {
		obj = payload
	}
	success, _ := obj["success"].(bool)
	pending, _ := obj["pending"].(bool)

	var orderID, txnID int64

	if orderVal := obj["order"]; orderVal != nil {
		if num, ok := orderVal.(json.Number); ok {
			orderID, _ = num.Int64()
		} else if f, ok := orderVal.(float64); ok {
			orderID = int64(f)
		}
	}

	if idVal := obj["id"]; idVal != nil {
		if num, ok := idVal.(json.Number); ok {
			txnID, _ = num.Int64()
		} else if f, ok := idVal.(float64); ok {
			txnID = int64(f)
		}
	}

	return paymobTransactionData{
		Success: success,
		Pending: pending,
		OrderID: orderID,
		TxnID:   txnID,
	}
}

func handleSuccessfulPayment(c *gin.Context, payment *models.Payment, data paymobTransactionData) {
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(payment).Updates(map[string]interface{}{
			"status":        models.PaymentCompleted,
			"externalTxnId": fmt.Sprintf("%d", data.TxnID),
			"completedAt":   time.Now(),
		}).Error; err != nil {
			return err
		}

		return processPaymentItems(tx, payment)
	})

	if err != nil {
		analyticsservice.GetAuditService().LogAsync(payment.UserID, analyticsservice.AuditEventPaymentFailed, "payment", payment.ID, map[string]interface{}{"error": err.Error(), "orderId": data.OrderID}, c.ClientIP(), c.Request.UserAgent())
		api_response.Error(c, http.StatusInternalServerError, "Failed to update record")
		return
	}

	analyticsservice.GetAuditService().LogAsync(payment.UserID, analyticsservice.AuditEventPaymentSuccess, "payment", payment.ID, map[string]interface{}{"amount": payment.Amount, "orderId": data.OrderID}, c.ClientIP(), c.Request.UserAgent())
}

func handleFailedPayment(payment *models.Payment, orderID int64) {
	db.DB.Model(payment).Update("status", models.PaymentFailed)
	analyticsservice.GetAuditService().LogAsync(payment.UserID, analyticsservice.AuditEventPaymentFailed, "payment", payment.ID, map[string]interface{}{"reason": "provider_failed", "orderId": orderID}, "", "")
}
