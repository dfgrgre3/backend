package protected

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

const revenueSumQuery = "COALESCE(SUM(amount), 0)"

// GetAdminPayments returns paginated payments with summary stats and analytics.
// Supports advanced filters: status, method, subjectId, date range, amount range, search.
func GetAdminPayments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")
	method := c.Query("method")
	subjectID := c.Query("subjectId")
	minAmountStr := c.Query("minAmount")
	maxAmountStr := c.Query("maxAmount")
	from := c.Query("from")
	to := c.Query("to")

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
		// Normalize to lowercase — the DB stores statuses as lowercase
		// (e.g. "completed") while admin clients may send uppercase.
		query = query.Where(statusQuery, strings.ToLower(status))
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if subjectID != "" {
		query = query.Where("subject_id = ?", subjectID)
	}
	if minAmountStr != "" {
		if minAmount, err := strconv.ParseFloat(minAmountStr, 64); err == nil {
			query = query.Where("amount >= ?", minAmount)
		}
	}
	if maxAmountStr != "" {
		if maxAmount, err := strconv.ParseFloat(maxAmountStr, 64); err == nil {
			query = query.Where("amount <= ?", maxAmount)
		}
	}
	if from != "" {
		if fromTime, err := time.Parse("2006-01-02", from); err == nil {
			query = query.Where("created_at >= ?", fromTime)
		}
	}
	if to != "" {
		if toTime, err := time.Parse("2006-01-02", to); err == nil {
			query = query.Where("created_at < ?", toTime.AddDate(0, 0, 1))
		}
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

		completedAt := ""
		if !p.CompletedAt.IsZero() {
			completedAt = p.CompletedAt.Format(time.RFC3339)
		}

		items = append(items, gin.H{
			"id":            p.ID,
			"userId":        p.UserID,
			"planId":        p.PlanID,
			"amount":        p.Amount,
			"currency":      p.Currency,
			"status":        p.Status,
			"method":        p.Method,
			"transactionId": p.Reference,
			"externalTxnId": p.ExternalTxnID,
			"paymobOrderId": p.PaymobOrderID,
			"subjectId":     p.SubjectID,
			"createdAt":     p.CreatedAt,
			"updatedAt":     p.UpdatedAt,
			"completedAt":   completedAt,
			"user": gin.H{
				"id":     user.ID,
				"name":   userName,
				"email":  user.Email,
				"avatar": userAvatar,
			},
			"subject": subjectData,
		})
	}

	api_response.Success(c, gin.H{
		"payments":     items,
		"summary":      buildPaymentSummary(),
		"methods":      getPaymentMethodsDistribution(),
		"dailyRevenue": getDailyRevenue(30),
		"topSubjects":  getTopPaymentSubjects(),
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// buildPaymentSummary returns global payment summary stats (unfiltered KPIs).
func buildPaymentSummary() gin.H {
	var totalRevenue float64
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentCompleted).
		Select(revenueSumQuery).Scan(&totalRevenue)

	var totalCount, completedCount, pendingCount, failedCount, refundedCount int64
	db.DB.Model(&models.Payment{}).Count(&totalCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentCompleted).Count(&completedCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentPending).Count(&pendingCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentFailed).Count(&failedCount)
	db.DB.Model(&models.Payment{}).Where(statusQuery, models.PaymentRefunded).Count(&refundedCount)

	var todayRevenue, thisMonthRevenue float64
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	db.DB.Model(&models.Payment{}).
		Where("status = ? AND created_at >= ?", models.PaymentCompleted, startOfDay).
		Select(revenueSumQuery).Scan(&todayRevenue)
	db.DB.Model(&models.Payment{}).
		Where("status = ? AND created_at >= ?", models.PaymentCompleted, startOfMonth).
		Select(revenueSumQuery).Scan(&thisMonthRevenue)

	avgOrderValue := 0.0
	if completedCount > 0 {
		avgOrderValue = totalRevenue / float64(completedCount)
	}
	refundRate := 0.0
	if totalCount > 0 {
		refundRate = float64(refundedCount) / float64(totalCount) * 100
	}
	successRate := 0.0
	if totalCount > 0 {
		successRate = float64(completedCount) / float64(totalCount) * 100
	}

	return gin.H{
		"totalPayments":    totalCount,
		"totalRevenue":     totalRevenue,
		"completedCount":   completedCount,
		"pendingCount":     pendingCount,
		"failedCount":      failedCount,
		"refundedCount":    refundedCount,
		"todayRevenue":     todayRevenue,
		"thisMonthRevenue": thisMonthRevenue,
		"avgOrderValue":    avgOrderValue,
		"refundRate":       refundRate,
		"successRate":      successRate,
	}
}

// getPaymentMethodsDistribution returns completed-payment totals grouped by method.
func getPaymentMethodsDistribution() []gin.H {
	type methodRow struct {
		Method string
		Count  int64
		Total  float64
	}
	var rows []methodRow
	if err := db.DB.Model(&models.Payment{}).
		Select("method, COUNT(*) as count, COALESCE(SUM(amount), 0) as total").
		Where(statusQuery, models.PaymentCompleted).
		Group("method").
		Order("count DESC").
		Scan(&rows).Error; err != nil {
		return []gin.H{}
	}

	result := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		result = append(result, gin.H{
			"method": r.Method,
			"count":  r.Count,
			"total":  r.Total,
		})
	}
	return result
}

// getDailyRevenue returns the last `days` days of completed-payment revenue/count.
func getDailyRevenue(days int) []gin.H {
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))

	type dailyRow struct {
		Day     string
		Revenue float64
		Count   int64
	}
	var rows []dailyRow
	if err := db.DB.Model(&models.Payment{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COALESCE(SUM(amount), 0) as revenue, COUNT(*) as count").
		Where("status = ? AND created_at >= ?", models.PaymentCompleted, start).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("day").
		Scan(&rows).Error; err != nil {
		return []gin.H{}
	}

	byDay := make(map[string]dailyRow, len(rows))
	for _, r := range rows {
		byDay[r.Day] = r
	}

	result := make([]gin.H, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		if r, ok := byDay[key]; ok {
			result = append(result, gin.H{"date": key, "revenue": r.Revenue, "count": r.Count})
		} else {
			result = append(result, gin.H{"date": key, "revenue": 0.0, "count": 0})
		}
	}
	return result
}

// getTopPaymentSubjects returns top subjects by completed-payment count.
func getTopPaymentSubjects() []gin.H {
	type subjectRow struct {
		SubjectID string
		Count     int64
		Revenue   float64
	}
	var rows []subjectRow
	if err := db.DB.Model(&models.Payment{}).
		Select("subject_id as subject_id, COUNT(*) as count, COALESCE(SUM(amount), 0) as revenue").
		Where("status = ? AND subject_id IS NOT NULL", models.PaymentCompleted).
		Group("subject_id").
		Order("count DESC").
		Limit(6).
		Scan(&rows).Error; err != nil {
		return []gin.H{}
	}

	result := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		result = append(result, gin.H{
			"id":      r.SubjectID,
			"name":    getSubjectNameForAdmin(r.SubjectID),
			"count":   r.Count,
			"revenue": r.Revenue,
		})
	}
	return result
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

// AdminBulkRefundPayments refunds multiple completed payments in one request.
// Registered at POST /api/admin/payments/refund/bulk (admin panel bulk refunds action).
func AdminBulkRefundPayments(c *gin.Context) {
	var input struct {
		PaymentIDs []string `json:"paymentIds" binding:"required,min=1,max=500"`
		Reason     string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Deduplicate IDs
	seen := make(map[string]struct{}, len(input.PaymentIDs))
	ids := make([]string, 0, len(input.PaymentIDs))
	for _, id := range input.PaymentIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No valid payment ids provided")
		return
	}

	var payments []models.Payment
	if err := db.DB.Where("id IN ? AND status = ?", ids, models.PaymentCompleted).
		Find(&payments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payments")
		return
	}

	if len(payments) == 0 {
		api_response.Error(c, http.StatusNotFound, "No completed payments found for the provided ids")
		return
	}

	refundableIDs := make([]string, 0, len(payments))
	for _, p := range payments {
		refundableIDs = append(refundableIDs, p.ID)
	}

	if err := db.DB.Model(&models.Payment{}).
		Where("id IN ?", refundableIDs).
		Updates(map[string]interface{}{
			"status":     models.PaymentRefunded,
			"updated_at": time.Now(),
		}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to process bulk refund")
		return
	}

	api_response.Success(c, gin.H{
		"message":    "Payments refunded successfully",
		"refunded":   len(refundableIDs),
		"totalCount": len(ids),
		"reason":     input.Reason,
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
