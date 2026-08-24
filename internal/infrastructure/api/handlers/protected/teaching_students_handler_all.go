package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingGetAllStudents returns all enrolled students across all instructor's courses.
func TeachingGetAllStudents(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))

	baseQuery := database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Preload("User").
		Preload("Subject")

	if search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		baseQuery = baseQuery.
			Joins(`JOIN users ON users.id = "SubjectEnrollment".user_id`).
			Where("LOWER(COALESCE(users.name, '')) LIKE ? OR LOWER(COALESCE(users.email, '')) LIKE ?", pattern, pattern)
	}

	var total int64
	baseQuery.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * limit
	var enrollments []models.Enrollment
	if err := baseQuery.
		Order(`"SubjectEnrollment".enrolled_at DESC`).
		Offset(offset).
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch students")
		return
	}

	// Deduplicate by userId - only show each student once
	seen := make(map[string]bool)
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

	allStudents := make([]StudentItem, 0, len(enrollments))
	for _, e := range enrollments {
		if seen[e.UserID] {
			// Add course progress to existing student
			for i := range allStudents {
				if allStudents[i].ID == e.UserID {
					progress, _ := e.Progress.Float64()
					allStudents[i].CourseProgress = append(allStudents[i].CourseProgress, StudentCourseProgress{
						CourseID:        e.SubjectID,
						CourseTitle:     e.Subject.Name,
						ProgressPercent: progress,
						LastActive:      e.UpdatedAt.Format("2006-01-02"),
					})
					break
				}
			}
			continue
		}
		seen[e.UserID] = true

		name := ""
		avatar := ""
		email := e.User.Email
		if e.User.ID != "" {
			name = stringPtrToString(e.User.Name)
			avatar = stringPtrToString(e.User.Avatar)
			if name == "" {
				name = email
			}
		}

		progress, _ := e.Progress.Float64()
		allStudents = append(allStudents, StudentItem{
			ID:     e.UserID,
			Name:   name,
			Avatar: avatar,
			Email:  email,
			CourseProgress: []StudentCourseProgress{
				{
					CourseID:        e.SubjectID,
					CourseTitle:     e.Subject.Name,
					ProgressPercent: progress,
					LastActive:      e.UpdatedAt.Format("2006-01-02"),
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
		"students": allStudents,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}
