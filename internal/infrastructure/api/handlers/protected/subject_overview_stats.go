package protected

import (
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const courseOverviewWeeks = 7

// courseOverviewAgg holds the lifetime enrollment metrics of a single course.
type courseOverviewAgg struct {
	TotalEnrollments int64   `gorm:"column:total_enrollments"`
	ActiveStudents   int64   `gorm:"column:active_students"`
	CompletedStudent int64   `gorm:"column:completed_students"`
	NotStarted       int64   `gorm:"column:not_started"`
	AvgCompletion    float64 `gorm:"column:avg_completion"`
}

// courseOverviewBucket is one weekly bucket, where bucket 0 is the current week.
type courseOverviewBucket struct {
	Bucket      int   `gorm:"column:bucket"`
	Enrollments int64 `gorm:"column:enrollments"`
	Active      int64 `gorm:"column:active"`
	Completed   int64 `gorm:"column:completed"`
}

// GetCourseOverviewStats returns the real enrollment metrics and the weekly
// enrollment/engagement timeline for one course, replacing the synthetic numbers
// the admin overview page used to generate client-side.
func GetCourseOverviewStats(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	courseID := c.Param("id")
	var subject models.Subject
	if err := applyIDOrSlugQuery(readDB.Model(&models.Subject{}), courseID).First(&subject).Error; err != nil {
		handleSubjectError(c, courseID, err, "fetching course for stats")
		return
	}

	price, _ := subject.Price.Float64()

	var agg courseOverviewAgg
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subject.ID).
		Select(`COUNT(*) AS total_enrollments,
			COUNT(DISTINCT CASE WHEN progress > 0 AND progress < 100 THEN user_id END) AS active_students,
			COUNT(DISTINCT CASE WHEN progress >= 100 THEN user_id END) AS completed_students,
			COUNT(DISTINCT CASE WHEN progress = 0 THEN user_id END) AS not_started,
			COALESCE(AVG(progress), 0) AS avg_completion`).
		Scan(&agg)

	now := time.Now()
	current := courseWindowEnrollments(readDB, subject.ID, now.AddDate(0, 0, -30), now)
	previous := courseWindowEnrollments(readDB, subject.ID, now.AddDate(0, 0, -60), now.AddDate(0, 0, -30))

	timeline := courseOverviewTimeline(readDB, subject.ID, price, now)

	avgScore, gradedResults := courseAverageExamScore(readDB, subject.ID)

	dropoffRate := 0.0
	if agg.TotalEnrollments > 0 {
		dropoffRate = roundTo(float64(agg.NotStarted)/float64(agg.TotalEnrollments)*100, 1)
	}

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"totalEnrollments":  agg.TotalEnrollments,
			"activeStudents":    agg.ActiveStudents,
			"completedStudents": agg.CompletedStudent,
			"completionRate":    roundTo(agg.AvgCompletion, 1),
			"dropoffRate":       dropoffRate,
			"totalRevenue":      roundTo(float64(agg.TotalEnrollments)*price, 2),
			"avgScore":          avgScore,
			"gradedResults":     gradedResults,
			"growth": gin.H{
				"enrollments": percentChange(float64(previous), float64(current)),
			},
			"timeline": timeline,
		},
	})
}

// courseAverageExamScore averages the exam results of a course as a percentage
// of each exam's maximum score. Returns nil when the course has no results yet.
func courseAverageExamScore(readDB *gorm.DB, subjectID string) (*float64, int64) {
	var row struct {
		AvgScore *float64 `gorm:"column:avg_score"`
		Total    int64    `gorm:"column:total"`
	}
	readDB.Table(`"ExamResult" AS r`).
		Joins(`JOIN "Exam" AS e ON e.id = r.exam_id AND e.deleted_at IS NULL`).
		Where("e.subject_id = ?", subjectID).
		Select(`AVG(r.score / NULLIF(e.max_score, 0)) * 100 AS avg_score, COUNT(*) AS total`).
		Scan(&row)

	if row.AvgScore == nil {
		return nil, row.Total
	}
	rounded := roundTo(*row.AvgScore, 1)
	return &rounded, row.Total
}

func courseWindowEnrollments(readDB *gorm.DB, subjectID string, from, to time.Time) int64 {
	var count int64
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subjectID).
		Where("enrolled_at >= ? AND enrolled_at < ?", from, to).
		Count(&count)
	return count
}

// courseOverviewTimeline buckets the last `courseOverviewWeeks` weeks of
// enrollments into a chronological series (oldest first) for the overview chart.
func courseOverviewTimeline(readDB *gorm.DB, subjectID string, price float64, now time.Time) []gin.H {
	since := now.AddDate(0, 0, -7*courseOverviewWeeks)

	var buckets []courseOverviewBucket
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subjectID).
		Where("enrolled_at >= ?", since).
		Select(`FLOOR(EXTRACT(EPOCH FROM (?::timestamptz - enrolled_at)) / 604800)::int AS bucket,
			COUNT(*) AS enrollments,
			COUNT(DISTINCT CASE WHEN progress > 0 AND progress < 100 THEN user_id END) AS active,
			COUNT(DISTINCT CASE WHEN progress >= 100 THEN user_id END) AS completed`, now).
		Group("bucket").
		Scan(&buckets)

	byBucket := make(map[int]courseOverviewBucket, len(buckets))
	for _, bucket := range buckets {
		byBucket[bucket.Bucket] = bucket
	}

	series := make([]gin.H, 0, courseOverviewWeeks)
	for offset := courseOverviewWeeks - 1; offset >= 0; offset-- {
		bucket := byBucket[offset]
		weekEnd := now.AddDate(0, 0, -7*offset)
		series = append(series, gin.H{
			"weekStart":   weekEnd.AddDate(0, 0, -7).Format("2006-01-02"),
			"enrollments": bucket.Enrollments,
			"revenue":     roundTo(float64(bucket.Enrollments)*price, 2),
			"active":      bucket.Active,
			"completed":   bucket.Completed,
		})
	}
	return series
}
