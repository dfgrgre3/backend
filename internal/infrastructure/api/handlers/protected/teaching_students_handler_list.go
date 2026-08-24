package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingListStudents returns enrolled students for a course.
func TeachingListStudents(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	page, limit := parsePagination(c)
	var total int64
	database.Model(&models.Enrollment{}).
		Where("subject_id = ?", courseID).
		Count(&total)

	offset := (page - 1) * limit
	var enrollments []models.Enrollment
	if err := database.
		Where("subject_id = ?", courseID).
		Order("enrolled_at DESC").
		Offset(offset).
		Limit(limit).
		Preload("User").
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch students")
		return
	}

	type StudentCourseProgress struct {
		CourseID        string  `json:"courseId"`
		CourseTitle     string  `json:"courseTitle"`
		ProgressPercent float64 `json:"progressPercent"`
		LastActive      string  `json:"lastActive"`
	}

	type StudentItem struct {
		ID             string                  `json:"id"`
		Name           string                  `json:"name"`
		Avatar         string                  `json:"avatar"`
		Email          string                  `json:"email"`
		CourseProgress []StudentCourseProgress `json:"courseProgress"`
		JoinDate       string                  `json:"joinDate"`
	}

	students := make([]StudentItem, 0, len(enrollments))
	for _, e := range enrollments {
		progress, _ := e.Progress.Float64()

		// Get avatar and name from user
		name := ""
		avatar := ""
		email := ""
		if e.User.ID != "" {
			name = stringPtrToString(e.User.Name)
			avatar = stringPtrToString(e.User.Avatar)
			email = e.User.Email
			if name == "" {
				name = email
			}
		}

		// Relative time for last activity
		lastActive := e.UpdatedAt.Format("2006-01-02")

		students = append(students, StudentItem{
			ID:     e.UserID,
			Name:   name,
			Avatar: avatar,
			Email:  email,
			CourseProgress: []StudentCourseProgress{
				{
					CourseID:        subject.ID,
					CourseTitle:     subject.Name,
					ProgressPercent: progress,
					LastActive:      lastActive,
				},
			},
			JoinDate: e.EnrolledAt.Format("2006-01-02"),
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	api_response.Success(c, gin.H{
		"students": students,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}
