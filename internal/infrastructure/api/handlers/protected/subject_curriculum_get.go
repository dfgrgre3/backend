package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// @Summary Get course curriculum
// @Tags courses
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/courses/{id}/curriculum [get]
func GetSubjectCurriculum(c *gin.Context) {
	id := c.Param("id")
	if id == "" || strings.EqualFold(id, "undefined") || strings.EqualFold(id, "null") {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}
	var subject models.Subject

	query := db.DB.Preload(preloadTopicsSubTopics)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err == nil {
		chaptersCount := len(subject.Topics)
		lessonsCount := 0
		freeLessonsCount := 0
		totalDuration := 0

		for _, topic := range subject.Topics {
			lessonsCount += len(topic.SubTopics)
			for _, subtopic := range topic.SubTopics {
				if subtopic.IsFree {
					freeLessonsCount++
				}
				totalDuration += subtopic.DurationMinutes
			}
		}

		curriculum := make([]gin.H, 0, len(subject.Topics))
		for _, topic := range subject.Topics {
			lessons := make([]gin.H, 0, len(topic.SubTopics))
			for _, lesson := range topic.SubTopics {
				lessons = append(lessons, gin.H{
					"id": lesson.ID, "name": lesson.Title, "title": lesson.Title,
					"order": lesson.Order, "durationMinutes": lesson.DurationMinutes,
					"type": lesson.Type, "isFree": lesson.IsFree,
					"locked": false, "completed": false,
				})
			}
			curriculum = append(curriculum, gin.H{
				"id": topic.ID, "name": topic.Title, "title": topic.Title,
				"order": topic.Order, "subTopics": lessons,
			})
		}

		var enrollment *models.Enrollment
		if userID, ok := getAuthenticatedUserID(c); ok {
			var row models.Enrollment
			if db.DB.Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&row).Error == nil {
				enrollment = &row
			}
		}
		completedRequiredExams, requiredExams := int64(0), int64(0)
		completedCourseQuizzes, requiredCourseQuizzes := int64(0), int64(0)
		if enrollment != nil {
			userID, _ := getAuthenticatedUserID(c)
			completedRequiredExams, requiredExams = courseRequiredExamCompletion(userID, subject.ID)
			completedCourseQuizzes, requiredCourseQuizzes = courseRequiredQuizCompletion(userID, subject.ID)
		}
		api_response.Success(c, gin.H{
			"subject": subject, "enrollment": enrollment,
			"completion": gin.H{
				"isComplete": enrollment != nil && enrollment.Progress.GreaterThanOrEqual(decimal.NewFromInt(100)),
				"progress": func() float64 {
					if enrollment == nil {
						return 0
					}
					value, _ := enrollment.Progress.Float64()
					return value
				}(),
				"certificateEligible":    enrollment != nil && enrollment.Progress.GreaterThanOrEqual(decimal.NewFromInt(100)) && subject.HasCertificate,
				"completedRequiredExams": completedRequiredExams,
				"requiredExams":          requiredExams,
				"completedCourseQuizzes": completedCourseQuizzes,
				"requiredCourseQuizzes":  requiredCourseQuizzes,
			},
			"curriculum": curriculum,
			"stats": gin.H{
				"chaptersCount":        chaptersCount,
				"lessonsCount":         lessonsCount,
				"freeLessonsCount":     freeLessonsCount,
				"totalDurationMinutes": totalDuration,
			},
			"topics": subject.Topics,
		})
		return
	}

	// Fallback: check LmsCourse
	var course models.LmsCourse
	if err := db.DB.Preload("Sections.Lessons").Where("id = ?", id).First(&course).Error; err != nil {
		// Course exists but has no curriculum data yet — return empty instead of 404
		api_response.Success(c, gin.H{
			"stats": gin.H{
				"chaptersCount":        0,
				"lessonsCount":         0,
				"freeLessonsCount":     0,
				"totalDurationMinutes": 0,
			},
			"topics":   []interface{}{},
			"sections": []interface{}{},
		})
		return
	}

	type curriculumSection struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Order   int    `json:"order"`
		Lessons []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Type     string `json:"type"`
			Duration int    `json:"duration"`
			IsFree   bool   `json:"isFree"`
		} `json:"lessons"`
	}

	sections := make([]curriculumSection, 0, len(course.Sections))
	totalLessons := 0
	freeLessons := 0
	totalDuration := 0

	for _, s := range course.Sections {
		ls := make([]struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Type     string `json:"type"`
			Duration int    `json:"duration"`
			IsFree   bool   `json:"isFree"`
		}, 0, len(s.Lessons))
		for _, l := range s.Lessons {
			ls = append(ls, struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Type     string `json:"type"`
				Duration int    `json:"duration"`
				IsFree   bool   `json:"isFree"`
			}{
				ID:       l.ID.String(),
				Title:    l.Title,
				Type:     string(l.Type),
				Duration: l.DurationSeconds / 60,
				IsFree:   l.IsFreePreview,
			})
			if l.IsFreePreview {
				freeLessons++
			}
			totalDuration += l.DurationSeconds / 60
		}
		sections = append(sections, curriculumSection{
			ID:      s.ID.String(),
			Title:   s.Title,
			Order:   s.OrderIndex,
			Lessons: ls,
		})
		totalLessons += len(s.Lessons)
	}

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"chaptersCount":        len(course.Sections),
			"lessonsCount":         totalLessons,
			"freeLessonsCount":     freeLessons,
			"totalDurationMinutes": totalDuration,
		},
		"sections": sections,
		"topics":   []interface{}{},
	})
}
