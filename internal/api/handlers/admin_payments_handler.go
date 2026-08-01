package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

const revenueSumQuery = "COALESCE(SUM(amount), 0)"

// GetAdminPayments returns paginated payments with summary stats
func GetAdminPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Build query
	query := db.DB.Model(&models.Payment{})

	if status != "" {
		query = query.Where(statusQuery, status)
	}

	if search != "" {
		query = query.Joins("LEFT JOIN \"User\" ON \"Payment\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"Payment\".reference ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Count total
	var total int64
	query.Count(&total)

	// Fetch payments
	var payments []models.Payment
	if err := query.
		Preload("Subject").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&payments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payments")
		return
	}

	// Build response items with user info
	items := make([]gin.H, 0, len(payments))
	for _, p := range payments {
		// Get user info
		var user models.User
		db.DB.Select("id", "name", "email", "avatar").Where("id = ?", p.UserID).First(&user)

		subjectData := gin.H(nil)
		if p.SubjectID != nil && *p.SubjectID != "" {
			subjectData = gin.H{
				"id":     p.Subject.ID,
				"name":   p.Subject.Name,
				"nameAr": p.Subject.NameAr,
			}
		}

		userName := ""
		if user.Name != nil {
			userName = *user.Name
		}
		userAvatar := ""
		if user.Avatar != nil {
			userAvatar = *user.Avatar
		}

		items = append(items, gin.H{
			"id":            p.ID,
			"userId":        p.UserID,
			"amount":        p.Amount,
			"currency":      p.Currency,
			"status":        p.Status,
			"method":        p.Method,
			"transactionId": p.Reference,
			"subjectId":     p.SubjectID,
			"createdAt":     p.CreatedAt,
			"updatedAt":     p.UpdatedAt,
			"user": gin.H{
				"id":     user.ID,
				"name":   userName,
				"email":  user.Email,
				"avatar": userAvatar,
			},
			"subject": subjectData,
		})
	}

	// Summary stats
	var totalRevenue float64
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentCompleted).
		Select(revenueSumQuery).Scan(&totalRevenue)

	var completedCount, pendingCount, failedCount int64
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentCompleted).Count(&completedCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentPending).Count(&pendingCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentFailed).Count(&failedCount)

	api_response.Success(c, gin.H{
		"payments": items,
		"summary": gin.H{
			"totalPayments":  total,
			"totalRevenue":   totalRevenue,
			"completedCount": completedCount,
			"pendingCount":   pendingCount,
			"failedCount":    failedCount,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// AdminRefundPayment marks a completed payment as refunded.
// Registered at POST /api/admin/payments/refund (admin panel refunds action).
func AdminRefundPayment(c *gin.Context) {
	var input struct {
		PaymentID string  `json:"paymentId" binding:"required"`
		Amount    float64 `json:"amount" binding:"required"`
		Reason    string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if input.Amount <= 0 {
		api_response.Error(c, http.StatusBadRequest, "Refund amount must be greater than zero")
		return
	}

	var payment models.Payment
	if err := db.DB.Where("id = ?", input.PaymentID).First(&payment).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Payment not found")
		return
	}

	if payment.Status != models.PaymentCompleted {
		api_response.Error(c, http.StatusConflict, "Only completed payments can be refunded")
		return
	}

	paymentAmount, _ := payment.Amount.Float64()
	if input.Amount > paymentAmount {
		api_response.Error(c, http.StatusBadRequest, "Refund amount exceeds the original payment amount")
		return
	}

	if err := db.DB.Model(&models.Payment{}).Where("id = ?", payment.ID).
		Updates(map[string]interface{}{
			"status":     models.PaymentRefunded,
			"updated_at": time.Now(),
		}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to process refund")
		return
	}

	api_response.Success(c, gin.H{
		"message":   "Payment refunded successfully",
		"paymentId": payment.ID,
		"amount":    input.Amount,
		"reason":    input.Reason,
	})
}

// GetAdminRevenue returns revenue analytics data
func GetAdminRevenue(c *gin.Context) {
	// Summary
	var todayRevenue, monthRevenue float64
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	db.DB.Model(&models.Payment{}).
		Where("status = ? AND created_at >= ?", models.PaymentCompleted, startOfDay).
		Select(revenueSumQuery).Scan(&todayRevenue)

	db.DB.Model(&models.Payment{}).
		Where("status = ? AND created_at >= ?", models.PaymentCompleted, startOfMonth).
		Select(revenueSumQuery).Scan(&monthRevenue)

	var totalTransactions int64
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentCompleted).Count(&totalTransactions)

	var totalUsers int64
	db.DB.Model(&models.User{}).Count(&totalUsers)

	conversionRate := "0%"
	if totalUsers > 0 {
		rate := float64(totalTransactions) / float64(totalUsers) * 100
		conversionRate = strconv.FormatFloat(rate, 'f', 1, 64) + "%"
	}

	chartData := getChartData(now)
	topPlans := getTopPlansData()

	api_response.Success(c, gin.H{
		"summary": gin.H{
			"today":             todayRevenue,
			"thisMonth":         monthRevenue,
			"totalTransactions": totalTransactions,
			"conversionRate":    conversionRate,
		},
		"chartData": chartData,
		"topPlans":  topPlans,
	})
}

func getChartData(now time.Time) []gin.H {
	chartData := make([]gin.H, 0, 6)
	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		startMonth := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, d.Location())
		endMonth := startMonth.AddDate(0, 1, 0)

		var revenue float64
		db.DB.Model(&models.Payment{}).
			Where("status = ? AND created_at >= ? AND created_at < ?",
				models.PaymentCompleted, startMonth, endMonth).
			Select(revenueSumQuery).Scan(&revenue)

		chartData = append(chartData, gin.H{
			"month":   int(d.Month()), // Send index for i18n
			"revenue": revenue,
		})
	}
	return chartData
}

func getTopPlansData() []gin.H {
	var topPlans []gin.H
	rows, err := db.DB.Model(&models.Payment{}).
		Select("subject_id, COUNT(*) as count").
		Where("status = ? AND subject_id IS NOT NULL", models.PaymentCompleted).
		Group("subject_id").
		Order("count DESC").
		Limit(5).
		Rows()

	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	for rows.Next() {
		var subjectID string
		var count int64
		if err := rows.Scan(&subjectID, &count); err == nil {
			topPlans = append(topPlans, gin.H{
				"name":  getSubjectNameForAdmin(subjectID),
				"count": count,
			})
		}
	}
	return topPlans
}

func getSubjectNameForAdmin(subjectID string) string {
	var subject models.Subject
	if err := db.DB.Select("name", "\"nameAr\"").Where("id = ?", subjectID).First(&subject).Error; err != nil {
		return "باقة عامة"
	}
	if subject.NameAr != nil && *subject.NameAr != "" {
		return *subject.NameAr
	}
	return subject.Name
}
