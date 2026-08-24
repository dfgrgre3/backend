package protected

import (
	models "thanawy-backend/internal/domain/common"

	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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
