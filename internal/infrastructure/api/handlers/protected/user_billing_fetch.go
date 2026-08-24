package protected

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fetchBillingData(uid string) gin.H {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	type billingResult struct {
		payments     []models.Payment
		totalSpent   float64
		successCount int64
		pendingCount int64
		failedCount  int64
	}

	var (
		user models.User
		wg   sync.WaitGroup
		res  billingResult
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		if u, err := getUserRepo().FindByID(uid); err == nil {
			user = *u
		}
	}()

	go func() {
		defer wg.Done()
		paymentRdb := readDB.Session(&gorm.Session{NewDB: true})
		paymentRdb.
			Model(&models.Payment{}).
			Select("id", "amount", "status", "created_at").
			Where("user_id = ?", uid).
			Order("created_at desc").
			Limit(10).
			Find(&res.payments)

		for _, p := range res.payments {
			amount, _ := p.Amount.Float64()
			switch p.Status {
			case models.PaymentCompleted:
				res.totalSpent += amount
				res.successCount++
			case models.PaymentPending:
				res.pendingCount++
			default:
				res.failedCount++
			}
		}
	}()

	wg.Wait()

	activeSubscriptionData := fetchActiveSubscription(uid)

	return gin.H{
		"name":                  stringOrEmpty(user.Name),
		"email":                 user.Email,
		"balance":               user.Balance,
		"additionalAiCredits":   user.AiCredits,
		"additionalExamCredits": user.ExamCredits,
		"activeSubscription":    activeSubscriptionData,
		"paymentHistory":        res.payments,
		"stats": gin.H{
			"totalSpent":   res.totalSpent,
			"paymentCount": len(res.payments),
			"successCount": res.successCount,
			"pendingCount": res.pendingCount,
			"failedCount":  res.failedCount,
		},
	}
}

func fetchActiveSubscription(uid string) interface{} {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	if readDB == nil {
		return nil
	}
	var activeSub models.UserSubscription
	err := readDB.
		Preload("Plan").
		Where("user_id = ? AND status = ? AND end_date > ?", uid, models.SubscriptionActive, time.Now()).
		First(&activeSub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isTableMissingError(err) {
			return nil
		}
		log.Printf("[billing] fetchActiveSubscription unexpected error: %v", err)
		return nil
	}
	return gin.H{
		"id":        activeSub.ID,
		"status":    activeSub.Status,
		"startDate": activeSub.StartDate,
		"endDate":   activeSub.EndDate,
		"plan": gin.H{
			"id":     activeSub.Plan.ID,
			"name":   activeSub.Plan.Name,
			"nameAr": activeSub.Plan.NameAr,
			"price":  activeSub.Plan.Price,
		},
		"payments": []gin.H{},
	}
}

// isTableMissingError detects PostgreSQL "relation does not exist" errors.
func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "42P01") || strings.Contains(msg, "does not exist")
}
