package protected

import (
	"math"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// courseEnrollmentAgg holds the aggregated enrollment metrics for the filtered
// course set.
type courseEnrollmentAgg struct {
	TotalEnrollments int64   `gorm:"column:total_enrollments"`
	ActiveStudents   int64   `gorm:"column:active_students"`
	AvgCompletion    float64 `gorm:"column:avg_completion"`
	TotalRevenue     float64 `gorm:"column:total_revenue"`
}

// courseWindowAgg holds enrollment/revenue totals for a single time window,
// used to compute month-over-month growth.
type courseWindowAgg struct {
	Enrollments int64   `gorm:"column:enrollments"`
	Revenue     float64 `gorm:"column:revenue"`
}

type courseStatsAgg struct {
	TotalCourses     int64 `gorm:"column:total_courses"`
	PublishedCourses int64 `gorm:"column:published_courses"`
	DraftCourses     int64 `gorm:"column:draft_courses"`
	ArchivedCourses  int64 `gorm:"column:archived_courses"`
	PaidCourses      int64 `gorm:"column:paid_courses"`
}

// GetCourseStats returns real aggregated course statistics. It honours exactly
// the same query parameters as GetSubjects / ExportSubjectsCSV, so the numbers
// always describe the filter combination the admin currently sees.
func GetCourseStats(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	// A fresh filtered query per aggregate: reusing one *gorm.DB would stack
	// the extra WHERE clauses of the previous count.
	filtered := func() *gorm.DB {
		return buildSubjectFilters(readDB.Model(&models.Subject{}), c)
	}

	// Compute all course counters in one scan. The old implementation issued
	// five COUNT queries before doing the enrollment aggregates, which made the
	// stats endpoint especially slow on the admin page.
	var courseStats courseStatsAgg
	if err := filtered().Select(`
		COUNT(*) AS total_courses,
		COUNT(*) FILTER (WHERE is_published = TRUE) AS published_courses,
		COUNT(*) FILTER (WHERE is_published = FALSE AND status <> ?) AS draft_courses,
		COUNT(*) FILTER (WHERE status = ?) AS archived_courses,
		COUNT(*) FILTER (WHERE price > 0) AS paid_courses`,
		models.CourseStatusArchived, models.CourseStatusArchived).Scan(&courseStats).Error; err != nil {
		api_response.ErrorDetail(c, 500, "Failed to calculate course statistics", err)
		return
	}

	var agg courseEnrollmentAgg
	courseEnrollmentQuery(readDB, filtered()).
		Select(`COUNT(*) AS total_enrollments,
			COUNT(DISTINCT CASE WHEN e.progress > 0 AND e.progress < 100 THEN e.user_id END) AS active_students,
			COALESCE(AVG(e.progress), 0) AS avg_completion,
			COALESCE(SUM(s.price), 0) AS total_revenue`).
		Scan(&agg)

	now := time.Now()
	current := courseEnrollmentWindow(readDB, filtered(), now.AddDate(0, 0, -30), now)
	previous := courseEnrollmentWindow(readDB, filtered(), now.AddDate(0, 0, -60), now.AddDate(0, 0, -30))

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"totalCourses":     courseStats.TotalCourses,
			"publishedCourses": courseStats.PublishedCourses,
			"draftCourses":     courseStats.DraftCourses,
			"archivedCourses":  courseStats.ArchivedCourses,
			"paidCourses":      courseStats.PaidCourses,
			"freeCourses":      courseStats.TotalCourses - courseStats.PaidCourses,
			"totalEnrollments": agg.TotalEnrollments,
			"activeStudents":   agg.ActiveStudents,
			"avgCompletion":    roundTo(agg.AvgCompletion, 1),
			"totalRevenue":     roundTo(agg.TotalRevenue, 2),
			"growth": gin.H{
				"enrollments": percentChange(float64(previous.Enrollments), float64(current.Enrollments)),
				"revenue":     percentChange(previous.Revenue, current.Revenue),
			},
		},
	})
}

// courseEnrollmentQuery joins enrollments to the filtered course set. The course
// filters are applied through a subquery because "SubjectEnrollment" and
// "Subject" share column names (id, created_at, deleted_at).
func courseEnrollmentQuery(readDB *gorm.DB, filteredSubjects *gorm.DB) *gorm.DB {
	return readDB.Table(`"SubjectEnrollment" AS e`).
		Joins(`JOIN "Subject" AS s ON s.id = e.subject_id`).
		Where("e.deleted_at IS NULL").
		Where("e.subject_id IN (?)", filteredSubjects.Select("id"))
}

func courseEnrollmentWindow(readDB *gorm.DB, filteredSubjects *gorm.DB, from, to time.Time) courseWindowAgg {
	var window courseWindowAgg
	courseEnrollmentQuery(readDB, filteredSubjects).
		Where("e.enrolled_at >= ? AND e.enrolled_at < ?", from, to).
		Select(`COUNT(*) AS enrollments, COALESCE(SUM(s.price), 0) AS revenue`).
		Scan(&window)
	return window
}

// percentChange returns the growth percentage from previous to current, rounded
// to one decimal. A jump from zero is reported as 100% rather than infinity.
func percentChange(previous, current float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return roundTo((current-previous)/previous*100, 1)
}

func roundTo(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}
