package admin

import (
	"fmt"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// buildDashboardGoals derives the weekly goals from real activity instead of
// fixed placeholder values.
func buildDashboardGoals(totalUsers, newUsersThisWeek, studyMinutes int64, monthlyRevenue float64, totalEnrollments int64) []gin.H {
	weeklyUserTarget := int64(50)
	if totalUsers > 0 {
		weeklyUserTarget = maxInt64(totalUsers/20, 10)
	}
	studyHours := studyMinutes / 60
	studyTarget := maxInt64(studyHours*2, 100)
	enrollmentTarget := maxInt64(totalEnrollments+10, 50)
	revenueTarget := maxFloat64(monthlyRevenue*1.2, 1000)

	return []gin.H{
		{
			"id": "users-weekly", "title": "مستخدمين جدد", "current": newUsersThisWeek,
			"target": weeklyUserTarget, "unit": "مستخدم", "category": "users", "priority": "medium",
		},
		{
			"id": "study-hours", "title": "ساعات الدراسة", "current": studyHours,
			"target": studyTarget, "unit": "ساعة", "category": "engagement", "priority": "high",
		},
		{
			"id": "enrollments", "title": "التسجيلات", "current": totalEnrollments,
			"target": enrollmentTarget, "unit": "تسجيل", "category": "content", "priority": "medium",
		},
		{
			"id": "revenue-monthly", "title": "الإيرادات الشهرية", "current": int64(monthlyRevenue),
			"target": int64(revenueTarget), "unit": "ج.م", "category": "revenue", "priority": "high",
		},
	}
}

func buildDashboardCriticalKPIs(activeStudents, totalUsers, publishedCourses, totalSubjects int64, completionRate, dailyRevenue float64) []gin.H {
	activeTarget := totalUsers
	if activeTarget == 0 {
		activeTarget = 100
	}
	contentTarget := totalSubjects
	if contentTarget == 0 {
		contentTarget = publishedCourses + 5
	}

	return []gin.H{
		{"name": "الطلاب النشطون", "value": activeStudents, "target": activeTarget, "unit": "طالب"},
		{"name": "الدورات المنشورة", "value": publishedCourses, "target": contentTarget, "unit": "دورة"},
		{"name": "معدل الإكمال", "value": int64(completionRate), "target": 75, "unit": "%"},
		{"name": "إيرادات اليوم", "value": int64(dailyRevenue), "target": int64(maxFloat64(dailyRevenue*1.2, 500)), "unit": "ج.م"},
	}
}

func buildDashboardSystemAlerts(reviewCourses, openTickets, securityAlertCount, liveClassCount int64) []gin.H {
	now := time.Now().UTC().Format(time.RFC3339)
	alerts := make([]gin.H, 0, 4)

	if reviewCourses > 0 {
		alerts = append(alerts, gin.H{
			"id": "review-courses", "type": "review", "severity": "warning",
			"message": fmt.Sprintf("هناك %d دورات بانتظار المراجعة", reviewCourses), "createdAt": now,
		})
	}
	if openTickets > 0 {
		alerts = append(alerts, gin.H{
			"id": "open-tickets", "type": "support", "severity": "warning",
			"message": fmt.Sprintf("يوجد %d تذكرة دعم مفتوحة", openTickets), "createdAt": now,
		})
	}
	if securityAlertCount > 0 {
		alerts = append(alerts, gin.H{
			"id": "security-events", "type": "security", "severity": "error",
			"message": fmt.Sprintf("تم رصد %d حدث أمني حديث", securityAlertCount), "createdAt": now,
		})
	}
	if liveClassCount > 0 {
		alerts = append(alerts, gin.H{
			"id": "live-classes", "type": "live", "severity": "info",
			"message": fmt.Sprintf("%d فصل مباشر نشط الآن", liveClassCount), "createdAt": now,
		})
	}

	return alerts
}

func calculateUserGrowthTrend() float64 {
	now := time.Now()
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := thisMonthStart.AddDate(0, -1, 0)

	var thisMonth int64
	var lastMonth int64

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	readDB.Model(&models.User{}).Where(createdAtGte, thisMonthStart).Count(&thisMonth)
	readDB.Model(&models.User{}).Where(createdAtGte+" AND created_at < ?", lastMonthStart, thisMonthStart).Count(&lastMonth)

	if lastMonth == 0 {
		if thisMonth > 0 {
			return 100.0
		}
		return 0.0
	}

	return float64(thisMonth-lastMonth) / float64(lastMonth) * 100.0
}

func calculateStudyTimeTrend() float64 {
	now := time.Now()
	thisWeekStart := now.AddDate(0, 0, -int(now.Weekday()))
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)

	var thisWeek int64
	var lastWeek int64

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	readDB.Model(&models.StudySession{}).Where("start_time >= ?", thisWeekStart).Select(coalesceSumDuration).Scan(&thisWeek)
	readDB.Model(&models.StudySession{}).Where("start_time >= ? AND start_time < ?", lastWeekStart, thisWeekStart).Select(coalesceSumDuration).Scan(&lastWeek)

	if lastWeek == 0 {
		if thisWeek > 0 {
			return 100.0
		}
		return 0.0
	}

	return float64(thisWeek-lastWeek) / float64(lastWeek) * 100.0
}

func calculateRevenueTrend(currentStart, previousStart time.Time) float64 {
	currentEnd := currentStart.AddDate(0, 0, 1)
	var currentTotal float64
	var previousTotal float64

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	readDB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("deleted_at IS NULL AND created_at >= ? AND created_at < ?", currentStart, currentEnd).
		Scan(&currentTotal)

	readDB.Model(&models.Payment{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("deleted_at IS NULL AND created_at >= ? AND created_at < ?", previousStart, currentStart).
		Scan(&previousTotal)

	if previousTotal == 0 {
		if currentTotal > 0 {
			return 100.0
		}
		return 0.0
	}

	return (currentTotal - previousTotal) / previousTotal * 100.0
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// readDashboardDB prefers the read replica and falls back to the primary.
func readDashboardDB() *gorm.DB {
	if readDB := db.ReadDB(); readDB != nil {
		return readDB
	}
	return db.DB
}
