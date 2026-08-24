package protected

import (
	"fmt"
	models "thanawy-backend/internal/domain/common"

	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func processPaymentItems(tx *gorm.DB, payment *models.Payment) error {
	if payment.SubjectID != nil && *payment.SubjectID != "" {
		if err := processSubjectEnrollment(tx, payment); err != nil {
			return err
		}
	}

	if payment.Method == "WALLET_TOPUP" {
		if err := processWalletTopup(tx, payment); err != nil {
			return err
		}
	}

	if payment.PlanID != "" {
		if err := processSubscriptionPayment(tx, payment); err != nil {
			return err
		}
	}

	return nil
}

func processSubjectEnrollment(tx *gorm.DB, payment *models.Payment) error {
	enrollment := models.Enrollment{
		UserID:     payment.UserID,
		SubjectID:  *payment.SubjectID,
		EnrolledAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&enrollment).Error; err != nil {
		return err
	}
	return tx.Model(&models.Subject{}).Where(idQuery, *payment.SubjectID).Update("enrolled_count", gorm.Expr("enrolled_count + 1")).Error
}

func processWalletTopup(tx *gorm.DB, payment *models.Payment) error {
	if err := tx.Model(&models.User{}).Where(idQuery, payment.UserID).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance + ?", payment.Amount),
			"version": gorm.Expr("version + 1"),
		}).Error; err != nil {
		return err
	}

	walletTx := models.WalletTransaction{
		UserID:      payment.UserID,
		Type:        models.TxTypeDeposit,
		Amount:      payment.Amount,
		Currency:    "EGP",
		WalletType:  "BALANCE",
		Description: "شحن رصيد عبر بوابة الدفع",
		ReferenceID: &payment.Reference,
	}
	return tx.Create(&walletTx).Error
}

func processSubscriptionPayment(tx *gorm.DB, payment *models.Payment) error {
	var plan models.SubscriptionPlan
	if err := tx.First(&plan, idQuery, payment.PlanID).Error; err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	duration := calculateSubscriptionDuration(plan.Interval)
	startDate := time.Now()
	endDate := startDate.Add(duration)

	sub := models.UserSubscription{
		UserID:               payment.UserID,
		PlanID:               plan.ID,
		Status:               models.SubscriptionActive,
		StartDate:            startDate,
		EndDate:              endDate,
		PaymobSubscriptionID: &payment.Reference,
	}
	if err := tx.Create(&sub).Error; err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	if err := tx.Model(&models.User{}).Where(idQuery, payment.UserID).Updates(map[string]interface{}{
		"activeSubscriptionId":  sub.ID,
		"subscriptionExpiresAt": endDate,
	}).Error; err != nil {
		return fmt.Errorf("failed to update user subscription: %w", err)
	}

	invoice := models.Invoice{
		PaymentID:     payment.ID,
		UserID:        payment.UserID,
		InvoiceNumber: "INV-" + time.Now().Format("20060102") + "-" + payment.ID[:8],
	}
	return tx.Create(&invoice).Error
}

func calculateSubscriptionDuration(interval models.SubscriptionInterval) time.Duration {
	switch interval {
	case models.IntervalYearly:
		return 365 * 24 * time.Hour
	case models.IntervalForever:
		return 100 * 365 * 24 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}
