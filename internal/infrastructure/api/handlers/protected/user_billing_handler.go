package protected

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type billingSummaryEntry struct {
	data      gin.H
	expiresAt time.Time
}

var (
	billingSummaryL1    sync.Map
	billingSummaryL1TTL = 30 * time.Second
	billingRedisTTL     = 2 * time.Minute
)

const billingSummaryCachePrefix = "billing_summary:"

func GetBillingSummary(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	uid := userId.(string)

	cacheKey := billingSummaryCachePrefix + uid

	if checkBillingCaches(c, cacheKey) {
		return
	}

	responseData := fetchBillingData(uid)
	if responseData == nil {
		return
	}

	storeBillingCache(cacheKey, responseData)
	api_response.Success(c, responseData)
}

func checkBillingCaches(c *gin.Context, cacheKey string) bool {
	if val, ok := billingSummaryL1.Load(cacheKey); ok {
		entry := val.(*billingSummaryEntry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, entry.data)
			return true
		}
		billingSummaryL1.Delete(cacheKey)
	}

	if cache.Redis != nil {
		redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		cachedVal, err := cache.Redis.Get(redisCtx, cacheKey).Result()
		cancel()
		if err == nil {
			var cachedData gin.H
			if json.Unmarshal([]byte(cachedVal), &cachedData) == nil {
				billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: cachedData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
				api_response.Success(c, cachedData)
				return true
			}
		}
	}
	return false
}

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
		// Fix: Use separate session to avoid concurrent map writes
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
	var activeSub models.UserSubscription
	// Use read replica if available, and degrade gracefully if the table
	// has not been created yet (e.g. before migrations are applied).
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	if readDB == nil {
		return nil
	}
	err := readDB.
		Preload("Plan").
		Where("user_id = ? AND status = ? AND end_date > ?", uid, models.SubscriptionActive, time.Now()).
		First(&activeSub).Error
	if err != nil {
		// Silently handle the "table does not exist" case (SQLSTATE 42P01)
		// and the "no active subscription" case so the billing summary
		// still returns a valid payload.
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

// isTableMissingError detects PostgreSQL "relation does not exist" errors so
// that the application can degrade gracefully when a table has not been
// created yet (e.g. before migrations are applied).
func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "42P01") || // undefined_table
		strings.Contains(msg, "does not exist")
}

func storeBillingCache(cacheKey string, responseData gin.H) {
	billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: responseData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
	if cache.Redis != nil {
		go func(key string, data gin.H) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if cacheBytes, err := json.Marshal(data); err == nil {
				cache.Redis.Set(ctx, key, cacheBytes, billingRedisTTL)
			}
		}(cacheKey, responseData)
	}
}
