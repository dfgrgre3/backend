package protected

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// courseProgressEntry is a single row in GET /api/users/progress/courses,
// combining the enrollment's own progress percentage (maintained elsewhere,
// e.g. UpdateLessonProgress) with a lesson-count breakdown for that course.
type courseProgressEntry struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Progress     float64 `json:"progress"`
	TotalLessons int64   `json:"totalLessons"`
	DoneLessons  int64   `json:"doneLessons"`
}

// lessonCountRow is the shape of one row from the GROUP BY query below.
type lessonCountRow struct {
	SubjectID string
	Total     int64
}

// GetUserProgressCourses returns the authenticated user's enrolled courses
// with a per-course progress rollup, for the "courses" tab of the Progress
// page. Scoped to the caller only - no course/user id is ever accepted from
// the client.
func GetUserProgressCourses(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var enrollments []models.Enrollment
	if err := db.DB.
		Preload("Subject").
		Where("user_id = ?", userId).
		Order("updated_at DESC").
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course progress")
		return
	}

	if len(enrollments) == 0 {
		api_response.Success(c, gin.H{
			"courses":        []courseProgressEntry{},
			"totalCourses":   0,
			"completed":      0,
			"inProgress":     0,
			"notStarted":     0,
			"averagePercent": 0,
		})
		return
	}

	subjectIDs := make([]string, 0, len(enrollments))
	for _, e := range enrollments {
		subjectIDs = append(subjectIDs, e.SubjectID)
	}

	// Total lesson count per subject, in one query (not one per course).
	var totalRows []lessonCountRow
	db.DB.Table("SubTopic").
		Select(`"Topic".subject_id AS subject_id, COUNT("SubTopic".id) AS total`).
		Joins(`JOIN "Topic" ON "Topic".id = "SubTopic".topic_id`).
		Where(`"Topic".subject_id IN ?`, subjectIDs).
		Group(`"Topic".subject_id`).
		Scan(&totalRows)

	totalBySubject := make(map[string]int64, len(totalRows))
	for _, row := range totalRows {
		totalBySubject[row.SubjectID] = row.Total
	}

	// Completed lesson count per subject for this user, in one query.
	var doneRows []lessonCountRow
	db.DB.Table("LessonProgress").
		Select(`"Topic".subject_id AS subject_id, COUNT("LessonProgress".id) AS total`).
		Joins(`JOIN "SubTopic" ON "SubTopic".id = "LessonProgress".sub_topic_id`).
		Joins(`JOIN "Topic" ON "Topic".id = "SubTopic".topic_id`).
		Where(`"LessonProgress".user_id = ? AND "LessonProgress".completed = ? AND "Topic".subject_id IN ?`, userId, true, subjectIDs).
		Group(`"Topic".subject_id`).
		Scan(&doneRows)

	doneBySubject := make(map[string]int64, len(doneRows))
	for _, row := range doneRows {
		doneBySubject[row.SubjectID] = row.Total
	}

	courses := make([]courseProgressEntry, 0, len(enrollments))
	completed, inProgress, notStarted := 0, 0, 0
	var percentSum float64

	for _, e := range enrollments {
		if e.Subject.ID == "" {
			continue // subject was deleted/unavailable; skip rather than show a blank row
		}

		title := e.Subject.Name
		if e.Subject.NameAr != nil && *e.Subject.NameAr != "" {
			title = *e.Subject.NameAr
		}

		progress, _ := e.Progress.Float64()
		total := totalBySubject[e.SubjectID]
		done := doneBySubject[e.SubjectID]

		courses = append(courses, courseProgressEntry{
			ID:           e.SubjectID,
			Title:        title,
			Progress:     progress,
			TotalLessons: total,
			DoneLessons:  done,
		})

		percentSum += progress
		switch {
		case total > 0 && done >= total:
			completed++
		case done > 0:
			inProgress++
		default:
			notStarted++
		}
	}

	averagePercent := 0.0
	if len(courses) > 0 {
		averagePercent = percentSum / float64(len(courses))
	}

	api_response.Success(c, gin.H{
		"courses":        courses,
		"totalCourses":   len(courses),
		"completed":      completed,
		"inProgress":     inProgress,
		"notStarted":     notStarted,
		"averagePercent": averagePercent,
	})
}
