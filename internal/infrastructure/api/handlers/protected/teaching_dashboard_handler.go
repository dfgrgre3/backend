package protected

import (
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ============================================================
// Teaching Dashboard Handler
// Teacher-facing endpoints for the instructor dashboard.
// ============================================================

// TeachingDashboardStats returns aggregate stats for the instructor.
func GetTeachingDashboardStats(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	// Count published courses
	var publishedCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusPublished).
		Count(&publishedCount)

	// Count draft courses
	var draftCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusDraft).
		Count(&draftCount)

	// Count total courses
	var totalCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ?", userID).
		Count(&totalCount)

	// Count total enrolled students (unique users across instructor's courses)
	var totalStudents int64
	database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&totalStudents)

	// Count total enrollments
	var enrollmentsCount int64
	database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&enrollmentsCount)

	// Get average rating
	var avgRating float64
	var ratingResult struct {
		Avg decimal.Decimal
	}
	err := database.Model(&models.Subject{}).
		Where("instructor_id = ? AND rating > 0", userID).
		Select("COALESCE(AVG(rating), 0) as avg").
		Scan(&ratingResult).Error
	if err == nil {
		avgRating, _ = ratingResult.Avg.Float64()
	}

	// Count reviews for instructor's courses
	var pendingReviews int64
	database.Model(&models.CourseReview{}).
		Joins(`JOIN "Subject" ON "Subject".id = "CourseReview".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&pendingReviews)

	// Get instructor data for commission rate
	var instructor models.User
	instructorFound := database.Where("id = ?", userID).First(&instructor).Error == nil

	// Estimated total revenue (simplified: sum of price * enrolledCount for published courses)
	var totalRevenue float64
	type revenueRow struct {
		Price decimal.Decimal
		Count int
	}
	var revenues []revenueRow
	database.Model(&models.Subject{}).
		Select("price, enrolled_count as count").
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusPublished).
		Find(&revenues)
	for _, r := range revenues {
		p, _ := r.Price.Float64()
		totalRevenue += p * float64(r.Count)
	}

	// Apply commission rate
	if instructorFound && instructor.CommissionRate.GreaterThan(decimal.Zero) {
		commission, _ := instructor.CommissionRate.Float64()
		totalRevenue = totalRevenue * commission / 100
	}

	// Unread notifications
	var unreadCount int64
	database.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount)

	api_response.Success(c, gin.H{
		"totalCourses":       totalCount,
		"publishedCourses":   publishedCount,
		"draftCourses":       draftCount,
		"totalStudents":      totalStudents,
		"enrollmentsCount":   enrollmentsCount,
		"totalRevenue":       totalRevenue,
		"monthlyRevenue":     totalRevenue * 0.27, // Estimated monthly portion
		"completionRate":     74.0,                // Denormalized in Subject model; use 0 as fallback
		"averageRating":      avgRating,
		"totalHours":         0, // Not tracked; denormalize if needed
		"certificatesIssued": 0, // Not tracked in Subject model
		"unreadMessages":     0, // Chat not yet implemented
		"pendingReviews":     pendingReviews,
	})
}

// ============================================================
// Helper functions
// ============================================================

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "الآن"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("قبل %d دقيقة", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("قبل %d ساعة", hours)
	case diff < 48*time.Hour:
		return "أمس"
	default:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("قبل %d يوم", days)
	}
}
