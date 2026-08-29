package protected

import (
	"fmt"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

		// Clear user's active subscription. Map keys must be the real DB
		// column names, not the struct's JSON tags — see
		// payment_handler_webhook.go for the identical bug and full
		// explanation. Before this fix, every cancellation failed with
		// "no such column" and rolled back, so CancelSubscription never
		// actually succeeded.
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"active_subscription_id":  nil,
			"subscription_expires_at": nil,
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
