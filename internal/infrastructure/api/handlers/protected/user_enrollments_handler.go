package protected

import (
	"net/http"
	"strconv"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// GetUserEnrollments returns a paginated list of course enrollments for a user.
func GetUserEnrollments(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	db.DB.Model(&models.Enrollment{}).Where("user_id = ?", userID).Count(&total)

	var enrollments []models.Enrollment
	if err := db.DB.Preload("Subject").Where("user_id = ?", userID).
		Order("enrolled_at DESC").
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	totalProgress := 0.0
	items := make([]gin.H, 0, len(enrollments))
	for _, e := range enrollments {
		subjectName := ""
		subjectSlug := ""
		price := 0.0
		if e.Subject.ID != "" {
			subjectName = e.Subject.Name
			subjectSlug = stringOrEmpty(e.Subject.Slug)
			price, _ = e.Subject.Price.Float64()
		}
		progress, _ := e.Progress.Float64()
		totalProgress += progress

		status := "ACTIVE"
		if progress >= 100.0 {
			status = "COMPLETED"
		}

		items = append(items, gin.H{
			"id":         e.ID,
			"courseId":   e.SubjectID,
			"courseName": subjectName,
			"courseSlug": subjectSlug,
			"price":      price,
			"progress":   e.Progress,
			"status":     status,
			"enrolledAt": e.EnrolledAt,
		})
	}

	avgProgress := 0.0
	if len(enrollments) > 0 {
		avgProgress = totalProgress / float64(len(enrollments))
	}

	api_response.Success(c, gin.H{
		"userId":      userID,
		"total":       total,
		"avgProgress": avgProgress,
		"enrollments": items,
	})
}

// GetUserOrders returns a paginated list of subscription orders for a user.
func GetUserOrders(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	db.DB.Model(&models.UserSubscription{}).Where("user_id = ?", userID).Count(&total)

	var subscriptions []models.UserSubscription
	if err := db.DB.Preload("Plan").Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&subscriptions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch orders")
		return
	}

	items := make([]gin.H, 0, len(subscriptions))
	for _, sub := range subscriptions {
		planName := ""
		planNameAr := ""
		price := decimal.NewFromInt(0)
		interval := ""
		if sub.Plan.ID != "" {
			planName = sub.Plan.Name
			planNameAr = sub.Plan.NameAr
			price = sub.Plan.Price
			interval = string(sub.Plan.Interval)
		}

		items = append(items, gin.H{
			"id":                   sub.ID,
			"planId":               sub.PlanID,
			"planName":             planName,
			"planNameAr":           planNameAr,
			"status":               string(sub.Status),
			"startDate":            sub.StartDate,
			"endDate":              sub.EndDate,
			"autoRenew":            sub.AutoRenew,
			"paymobSubscriptionId": sub.PaymobSubscriptionID,
			"price":                price,
			"interval":             interval,
			"createdAt":            sub.CreatedAt,
			"updatedAt":            sub.UpdatedAt,
		})
	}

	api_response.List(c, items, api_response.Pagination{
		Page:       1,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"orders": items,
	})
}
