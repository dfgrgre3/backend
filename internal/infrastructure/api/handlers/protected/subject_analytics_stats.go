package protected

import (
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const courseAnalyticsMonths = 6

var arabicMonths = [...]string{
	"يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو",
	"يوليو", "أغسطس", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر",
}

// courseAnalyticsMonthRow is one monthly enrollment bucket.
type courseAnalyticsMonthRow struct {
	MonthStart time.Time `gorm:"column:month_start"`
	Students   int64     `gorm:"column:students"`
}

// GetCourseAnalytics returns the monthly performance series and the headline
// metrics of a single course. Everything is derived from real enrollment and
// lesson-progress rows; no synthetic series are produced.
func GetCourseAnalytics(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	courseID := c.Param("id")
	var subject models.Subject
	if err := applyIDOrSlugQuery(readDB.Model(&models.Subject{}), courseID).First(&subject).Error; err != nil {
		handleSubjectError(c, courseID, err, "fetching course for analytics")
		return
	}

	price, _ := subject.Price.Float64()
	now := time.Now()

	var totalEnrollments, completedEnrollments int64
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subject.ID).
		Count(&totalEnrollments)
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subject.ID).
		Where("progress >= 100").
		Count(&completedEnrollments)

	current := courseWindowEnrollments(readDB, subject.ID, now.AddDate(0, 0, -30), now)
	previous := courseWindowEnrollments(readDB, subject.ID, now.AddDate(0, 0, -60), now.AddDate(0, 0, -30))

	completionRate := 0.0
	if totalEnrollments > 0 {
		completionRate = roundTo(float64(completedEnrollments)/float64(totalEnrollments)*100, 1)
	}

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"totalRevenue":   roundTo(float64(totalEnrollments)*price, 2),
			"newStudents":    current,
			"completionRate": completionRate,
			"watchTime":      courseWatchTimeHours(readDB, subject.ID),
			"growth": gin.H{
				"students": percentChange(float64(previous), float64(current)),
				"revenue":  percentChange(float64(previous)*price, float64(current)*price),
			},
		},
		"monthlyData": courseMonthlySeries(readDB, subject.ID, price, now),
		// No device/user-agent data is collected per enrollment yet, so the
		// admin page renders its empty state instead of inventing a split.
		"deviceData": []gin.H{},
	})
}

// courseWatchTimeHours sums the tracked watch time of every lesson that belongs
// to the course, converted to hours.
func courseWatchTimeHours(readDB *gorm.DB, subjectID string) float64 {
	var seconds int64
	readDB.Table(`"TopicProgress" AS tp`).
		Joins(`JOIN "SubTopic" AS st ON st.id = tp.sub_topic_id`).
		Joins(`JOIN "Topic" AS t ON t.id = st.topic_id`).
		Where("t.subject_id = ? AND tp.deleted_at IS NULL AND st.deleted_at IS NULL AND t.deleted_at IS NULL", subjectID).
		Select("COALESCE(SUM(tp.time_spent_seconds), 0)").
		Scan(&seconds)
	return roundTo(float64(seconds)/3600, 1)
}

// courseMonthlySeries returns the last `courseAnalyticsMonths` months of
// enrollments (oldest first), filling months without enrollments with zeros.
func courseMonthlySeries(readDB *gorm.DB, subjectID string, price float64, now time.Time) []gin.H {
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	since := firstOfMonth.AddDate(0, -(courseAnalyticsMonths - 1), 0)

	var rows []courseAnalyticsMonthRow
	readDB.Table(`"SubjectEnrollment"`).
		Where("subject_id = ? AND deleted_at IS NULL", subjectID).
		Where("enrolled_at >= ?", since).
		Select(`date_trunc('month', enrolled_at) AS month_start, COUNT(*) AS students`).
		Group("month_start").
		Scan(&rows)

	byMonth := make(map[string]int64, len(rows))
	for _, row := range rows {
		byMonth[row.MonthStart.Format("2006-01")] = row.Students
	}

	series := make([]gin.H, 0, courseAnalyticsMonths)
	for offset := courseAnalyticsMonths - 1; offset >= 0; offset-- {
		month := firstOfMonth.AddDate(0, -offset, 0)
		students := byMonth[month.Format("2006-01")]
		series = append(series, gin.H{
			"name":     arabicMonths[int(month.Month())-1],
			"month":    month.Format("2006-01"),
			"students": students,
			"revenue":  roundTo(float64(students)*price, 2),
		})
	}
	return series
}
