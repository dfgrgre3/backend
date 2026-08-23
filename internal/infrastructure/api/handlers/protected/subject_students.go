package protected

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxCourseStudentsExportRows = 5000

// courseStudentRow is one enrolled student together with the lesson-progress
// aggregates the admin students table needs.
type courseStudentRow struct {
	ID               string     `gorm:"column:id"`
	UserID           string     `gorm:"column:user_id"`
	Name             string     `gorm:"column:name"`
	Email            string     `gorm:"column:email"`
	Avatar           *string    `gorm:"column:avatar"`
	Progress         float64    `gorm:"column:progress"`
	EnrolledAt       time.Time  `gorm:"column:enrolled_at"`
	CompletedLessons int64      `gorm:"column:completed_lessons"`
	LastActive       *time.Time `gorm:"column:last_active"`
}

// status maps the stored progress onto the three states the admin UI filters on.
func (r courseStudentRow) status() string {
	switch {
	case r.Progress >= 100:
		return "completed"
	case r.Progress > 0:
		return "active"
	default:
		return "inactive"
	}
}

// GetCourseStudents returns the paginated list of students enrolled in a course
// with their real lesson progress and last activity.
// GET /api/admin/courses/:id/students
func GetCourseStudents(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	var subject models.Subject
	if !resolveCourseForStudents(c, readDB, &subject) {
		return
	}

	limit := 20
	if parsed, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && parsed > 0 {
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if raw := c.Query("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			offset = parsed
		}
	} else if parsed, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && parsed > 1 {
		offset = (parsed - 1) * limit
	}

	var total int64
	courseStudentsQuery(readDB, subject.ID, c).Count(&total)

	rows := fetchCourseStudents(readDB, subject.ID, c, limit, offset)
	totalLessons := courseLessonCount(readDB, subject.ID)

	students := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		students = append(students, courseStudentPayload(row, totalLessons))
	}

	api_response.Success(c, gin.H{
		"students": students,
		"pagination": api_response.Pagination{
			Page:       offset/limit + 1,
			Limit:      limit,
			Total:      total,
			TotalPages: int64(math.Ceil(float64(total) / float64(limit))),
		},
		"totalEnrolled": subject.EnrolledCount,
		"totalLessons":  totalLessons,
	})
}

// ExportCourseStudentsCSV streams the same filtered student list as a CSV file.
// GET /api/admin/courses/:id/students/export
func ExportCourseStudentsCSV(c *gin.Context) {
	readDB, aborted := safeReadDB(c)
	if aborted {
		return
	}

	var subject models.Subject
	if !resolveCourseForStudents(c, readDB, &subject) {
		return
	}

	rows := fetchCourseStudents(readDB, subject.ID, c, maxCourseStudentsExportRows, 0)
	totalLessons := courseLessonCount(readDB, subject.ID)

	filename := fmt.Sprintf("course-%s-students-%s.csv", subject.ID, time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	// BOM so Excel renders the Arabic names correctly.
	if _, err := c.Writer.WriteString("\uFEFF"); err != nil {
		return
	}

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{
		"userId", "name", "email", "status", "progress",
		"completedLessons", "totalLessons", "enrolledAt", "lastActive",
	}); err != nil {
		return
	}

	for _, row := range rows {
		lastActive := ""
		if row.LastActive != nil {
			lastActive = row.LastActive.Format(time.RFC3339)
		}
		if err := writer.Write([]string{
			row.UserID,
			row.Name,
			row.Email,
			row.status(),
			strconv.FormatFloat(row.Progress, 'f', 2, 64),
			strconv.FormatInt(row.CompletedLessons, 10),
			strconv.Itoa(totalLessons),
			row.EnrolledAt.Format(time.RFC3339),
			lastActive,
		}); err != nil {
			return
		}
	}
	writer.Flush()
}

func resolveCourseForStudents(c *gin.Context, readDB *gorm.DB, subject *models.Subject) bool {
	courseID := c.Param("id")
	if err := applyIDOrSlugQuery(readDB.Model(&models.Subject{}), courseID).First(subject).Error; err != nil {
		handleSubjectError(c, courseID, err, "fetching course for students list")
		return false
	}
	return true
}

// courseStudentsQuery builds the filtered enrollment query shared by the list,
// the count and the CSV export so all three always agree.
func courseStudentsQuery(readDB *gorm.DB, subjectID string, c *gin.Context) *gorm.DB {
	query := readDB.Table(`"SubjectEnrollment" AS e`).
		Joins(`JOIN "User" AS u ON u.id = e.user_id AND u.deleted_at IS NULL`).
		Where("e.subject_id = ? AND e.deleted_at IS NULL", subjectID)

	// The UI sends `status`; older callers send `progress`. Both are accepted.
	status := c.Query("status")
	if status == "" {
		status = c.Query("progress")
	}
	switch status {
	case "completed":
		query = query.Where("e.progress >= 100")
	case "active", "in_progress":
		query = query.Where("e.progress > 0 AND e.progress < 100")
	case "inactive", "not_started":
		query = query.Where("e.progress = 0")
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		pattern := "%" + search + "%"
		query = query.Where("(u.name ILIKE ? OR u.email ILIKE ?)", pattern, pattern)
	}

	return query
}

func fetchCourseStudents(readDB *gorm.DB, subjectID string, c *gin.Context, limit, offset int) []courseStudentRow {
	completedLessons := `(SELECT COUNT(*) FROM "TopicProgress" tp
		JOIN "SubTopic" st ON st.id = tp.sub_topic_id AND st.deleted_at IS NULL
		JOIN "Topic" t ON t.id = st.topic_id AND t.deleted_at IS NULL
		WHERE t.subject_id = ? AND tp.user_id = e.user_id AND tp.completed = true AND tp.deleted_at IS NULL)`
	lastActive := `(SELECT MAX(tp.updated_at) FROM "TopicProgress" tp
		JOIN "SubTopic" st ON st.id = tp.sub_topic_id AND st.deleted_at IS NULL
		JOIN "Topic" t ON t.id = st.topic_id AND t.deleted_at IS NULL
		WHERE t.subject_id = ? AND tp.user_id = e.user_id AND tp.deleted_at IS NULL)`

	var rows []courseStudentRow
	courseStudentsQuery(readDB, subjectID, c).
		Select(`e.id AS id,
			e.user_id AS user_id,
			COALESCE(u.name, '') AS name,
			u.email AS email,
			u.avatar AS avatar,
			e.progress AS progress,
			e.enrolled_at AS enrolled_at,
			`+completedLessons+` AS completed_lessons,
			`+lastActive+` AS last_active`, subjectID, subjectID).
		Order("e.enrolled_at DESC").
		Offset(offset).
		Limit(limit).
		Scan(&rows)
	return rows
}

// courseLessonCount counts the publishable lessons of a course.
func courseLessonCount(readDB *gorm.DB, subjectID string) int {
	var count int64
	readDB.Table(`"SubTopic" AS st`).
		Joins(`JOIN "Topic" AS t ON t.id = st.topic_id AND t.deleted_at IS NULL`).
		Where("t.subject_id = ? AND st.deleted_at IS NULL", subjectID).
		Count(&count)
	return int(count)
}

func courseStudentPayload(row courseStudentRow, totalLessons int) gin.H {
	payload := gin.H{
		"id":               row.ID,
		"userId":           row.UserID,
		"name":             row.Name,
		"email":            row.Email,
		"avatar":           row.Avatar,
		"progress":         roundTo(row.Progress, 1),
		"enrolledAt":       row.EnrolledAt,
		"completedLessons": row.CompletedLessons,
		"totalLessons":     totalLessons,
		"status":           row.status(),
		"completed":        row.Progress >= 100,
	}
	if row.LastActive != nil {
		payload["lastActive"] = row.LastActive
	} else {
		payload["lastActive"] = nil
	}
	return payload
}
