package admin

import (
	models "thanawy-backend/internal/domain/common"
	"time"

	"gorm.io/gorm"
)

// dashboardCoreStats holds the platform-wide counters resolved in one round trip.
type dashboardCoreStats struct {
	TotalUsers         int64 `gorm:"column:total_users"`
	NewUsersToday      int64 `gorm:"column:new_users_today"`
	NewUsersThisWeek   int64 `gorm:"column:new_users_this_week"`
	TotalSubjects      int64 `gorm:"column:total_subjects"`
	TotalExams         int64 `gorm:"column:total_exams"`
	CompletedTasks     int64 `gorm:"column:completed_tasks"`
	TotalStudySessions int64 `gorm:"column:total_study_sessions"`
	StudyMinutes       int64 `gorm:"column:study_minutes"`
	ExamsTaken         int64 `gorm:"column:exams_taken"`
	TotalResources     int64 `gorm:"column:total_resources"`
	ActiveChallenges   int64 `gorm:"column:active_challenges"`
	AchievementsEarned int64 `gorm:"column:achievements_earned"`
}

const dashboardCoreStatsQuery = `
	SELECT
		(SELECT COUNT(*) FROM "User" WHERE deleted_at IS NULL) as total_users,
		(SELECT COUNT(*) FROM "User" WHERE created_at >= ? AND deleted_at IS NULL) as new_users_today,
		(SELECT COUNT(*) FROM "User" WHERE created_at >= ? AND deleted_at IS NULL) as new_users_this_week,
		(SELECT COUNT(*) FROM "Subject" WHERE deleted_at IS NULL) as total_subjects,
		(SELECT COUNT(*) FROM "Exam" WHERE deleted_at IS NULL) as total_exams,
		(SELECT COUNT(*) FROM "Task" WHERE status = 'COMPLETED' AND deleted_at IS NULL) as completed_tasks,
		(SELECT COUNT(*) FROM "StudySession" WHERE deleted_at IS NULL) as total_study_sessions,
		(SELECT COALESCE(SUM(duration_min), 0) FROM "StudySession" WHERE deleted_at IS NULL) as study_minutes,
		(SELECT COUNT(*) FROM "ExamResult" WHERE deleted_at IS NULL) as exams_taken,
		(SELECT COUNT(*) FROM "SubTopic" WHERE type != 'QUIZ' AND deleted_at IS NULL) as total_resources,
		(SELECT COUNT(*) FROM "Challenge" WHERE is_active = true AND deleted_at IS NULL) as active_challenges,
		(SELECT COUNT(*) FROM "UserAchievement" WHERE deleted_at IS NULL) as achievements_earned
`

func loadDashboardCoreStats(readDB *gorm.DB, todayStart, weekAgo time.Time) (dashboardCoreStats, error) {
	var stats dashboardCoreStats
	err := readDB.Raw(dashboardCoreStatsQuery, todayStart, weekAgo).Scan(&stats).Error
	return stats, err
}

// dashboardAudienceStats holds the user/teacher counters the dashboard renders.
type dashboardAudienceStats struct {
	TotalTeachers       int64
	ActiveStudents      int64
	NewUsersThisMonth   int64
	NewTeachersToday    int64
	NewTeachersThisWeek int64
}

func loadDashboardAudienceStats(readDB *gorm.DB, todayStart, weekAgo, monthAgo time.Time) (dashboardAudienceStats, error) {
	var stats dashboardAudienceStats

	return stats, firstQueryError(
		readDB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ?", models.RoleTeacher).Count(&stats.TotalTeachers),
		readDB.Model(&models.User{}).Where("deleted_at IS NULL AND updated_at >= ?", monthAgo).Count(&stats.ActiveStudents),
		readDB.Model(&models.User{}).Where("deleted_at IS NULL AND created_at >= ?", monthAgo).Count(&stats.NewUsersThisMonth),
		readDB.Model(&models.User{}).
			Where("deleted_at IS NULL AND role = ? AND created_at >= ?", models.RoleTeacher, todayStart).
			Count(&stats.NewTeachersToday),
		readDB.Model(&models.User{}).
			Where("deleted_at IS NULL AND role = ? AND created_at >= ?", models.RoleTeacher, weekAgo).
			Count(&stats.NewTeachersThisWeek),
	)
}

// firstQueryError returns the first error raised by the supplied query results.
// Dashboard loaders previously discarded these errors, so a failing count was
// rendered as a real zero instead of surfacing as a failure.
func firstQueryError(results ...*gorm.DB) error {
	for _, result := range results {
		if result != nil && result.Error != nil {
			return result.Error
		}
	}
	return nil
}

// dashboardCatalogStats holds the course/lesson/enrollment counters.
type dashboardCatalogStats struct {
	PublishedCourses     int64
	DraftCourses         int64
	ReviewCourses        int64
	ArchivedCourses      int64
	TotalLessons         int64
	TotalEnrollments     int64
	CompletedEnrollments int64
	// LmsEnrollments is the denominator that pairs with CompletedEnrollments.
	// TotalEnrollments counts the legacy "Enrollment" table and stays in the
	// payload for contract compatibility, but mixing the two tables in one
	// ratio produced a completion rate that could exceed 100%.
	LmsEnrollments int64
}

// CompletionRate is the share of LMS enrollments that reached completion.
// Numerator and denominator are read from the same table so the ratio is
// bounded by 100.
func (s dashboardCatalogStats) CompletionRate() float64 {
	if s.LmsEnrollments == 0 {
		return 0
	}
	return float64(s.CompletedEnrollments) / float64(s.LmsEnrollments) * 100
}

func loadDashboardCatalogStats(readDB *gorm.DB) (dashboardCatalogStats, error) {
	var stats dashboardCatalogStats

	return stats, firstQueryError(
		readDB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "PUBLISHED").Count(&stats.PublishedCourses),
		readDB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "DRAFT").Count(&stats.DraftCourses),
		readDB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "REVIEW").Count(&stats.ReviewCourses),
		readDB.Model(&models.LmsCourse{}).Where("deleted_at IS NULL AND status = ?", "ARCHIVED").Count(&stats.ArchivedCourses),
		readDB.Model(&models.Lesson{}).Where("deleted_at IS NULL").Count(&stats.TotalLessons),
		readDB.Model(&models.Enrollment{}).Where("deleted_at IS NULL").Count(&stats.TotalEnrollments),
		readDB.Model(&models.LmsEnrollment{}).Where("deleted_at IS NULL").Count(&stats.LmsEnrollments),
		readDB.Model(&models.LmsEnrollment{}).Where("deleted_at IS NULL AND completed_at IS NOT NULL").Count(&stats.CompletedEnrollments),
	)
}

// dashboardRevenueStats holds the revenue aggregates for each window.
type dashboardRevenueStats struct {
	Daily   float64
	Monthly float64
	Yearly  float64
	Pending float64
}

func loadDashboardRevenueStats(readDB *gorm.DB, todayStart, monthAgo, yearAgo time.Time) (dashboardRevenueStats, error) {
	var stats dashboardRevenueStats
	var queries []*gorm.DB

	// Only settled money counts as revenue. Without the status filter the
	// totals silently included pending, failed and refunded payments.
	sumPayments := func(since time.Time, target *float64) {
		queries = append(queries, readDB.Model(&models.Payment{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("deleted_at IS NULL AND status = ? AND created_at >= ?", models.PaymentCompleted, since).
			Scan(target))
	}

	sumPayments(todayStart, &stats.Daily)
	sumPayments(monthAgo, &stats.Monthly)
	sumPayments(yearAgo, &stats.Yearly)

	queries = append(queries, userSubscriptionScope(readDB).
		Select("COALESCE(SUM(sp.price), 0)").
		Joins("LEFT JOIN \"SubscriptionPlan\" sp ON \"UserSubscription\".plan_id = sp.id").
		Where("\"UserSubscription\".status = ?", models.SubscriptionPending).
		Scan(&stats.Pending))

	return stats, firstQueryError(queries...)
}

// dashboardOperationsStats holds the operational queue counters.
type dashboardOperationsStats struct {
	OpenTickets            int64
	PendingOrders          int64
	NewSubscriptions       int64
	CancelledSubscriptions int64
}

func loadDashboardOperationsStats(readDB *gorm.DB, periodStart time.Time) (dashboardOperationsStats, error) {
	var stats dashboardOperationsStats

	return stats, firstQueryError(
		readDB.Model(&models.SupportTicket{}).
			Where("deleted_at IS NULL AND status IN ?", []string{"open", "in_progress", "escalated"}).
			Count(&stats.OpenTickets),
		userSubscriptionScope(readDB).Where("status = ?", models.SubscriptionPending).Count(&stats.PendingOrders),
		userSubscriptionScope(readDB).Where("created_at >= ?", periodStart).Count(&stats.NewSubscriptions),
		userSubscriptionScope(readDB).
			Where("status = ? AND updated_at >= ?", models.SubscriptionCancelled, periodStart).
			Count(&stats.CancelledSubscriptions),
	)
}

// userSubscriptionScope builds a UserSubscription query that tolerates schemas
// where the soft-delete column has not been migrated yet.
func userSubscriptionScope(conn *gorm.DB) *gorm.DB {
	query := conn.Model(&models.UserSubscription{})
	if !hasUserSubscriptionDeletedAtColumn() {
		query = query.Unscoped()
	}
	return query
}
