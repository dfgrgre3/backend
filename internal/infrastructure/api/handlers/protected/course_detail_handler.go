package protected

import (
	"net/http"
	"sort"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetCourseDetailHydration returns one consistent public/private snapshot for
// the course detail page. Protected media is included only for enrolled users.
// @Summary Get course detail
// @Description Get the public/private course detail snapshot.
// @Tags courses
// @Produce json
// @Param id path string true "Course ID or slug"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/courses/{id}/detail [get]
func GetCourseDetailHydration(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	var subject models.Subject
	query := database.Preload(preloadAdvanced)
	if err := applyIDOrSlugQuery(query, c.Param("id")).First(&subject).Error; err != nil {
		// Courses created by the admin LMS use LmsCourse rather than the
		// legacy Subject aggregate. Keep the public detail contract available
		// for both models while the migration is in progress.
		var course models.LmsCourse
		if lmsErr := applyIDOrSlugQuery(database, c.Param("id")).
			Preload("Sections.Lessons").First(&course).Error; lmsErr == nil {
			renderLmsCourseDetail(c, database, course)
			return
		}
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, msgSubjectNotFound)
		} else {
			handleSubjectError(c, c.Param("id"), err, "fetching course detail")
		}
		return
	}

	userID, _ := c.Get("userId")
	userIDString, _ := userID.(string)
	isEnrolled := false
	var enrollment *models.Enrollment
	if userIDString != "" {
		var row models.Enrollment
		if database.Where("user_id = ? AND subject_id = ?", userIDString, subject.ID).First(&row).Error == nil {
			isEnrolled = true
			enrollment = &row
		}
	}

	if !isEnrolled {
		redactLockedLessonContent(&subject)
	}

	type progressView struct {
		Completed bool `json:"completed"`
	}
	progress := map[string]progressView{}
	if userIDString != "" {
		var rows []models.LessonProgress
		database.Where("user_id = ? AND sub_topic_id IN (?)", userIDString,
			database.Table("SubTopic").Select("SubTopic.id").Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").Where("Topic.subject_id = ?", subject.ID),
		).Find(&rows)
		for _, row := range rows {
			progress[row.LessonID] = progressView{Completed: row.Completed}
		}
	}

	lessons := make([]Lesson, 0)
	for _, topic := range subject.Topics {
		for _, lesson := range topic.SubTopics {
			lessons = append(lessons, Lesson{
				ID: lesson.ID, Title: lesson.Title, Description: stringOrEmpty(lesson.Description),
				Content: stringOrEmpty(lesson.Content), VideoUrl: stringOrEmpty(lesson.VideoUrl),
				Type: string(lesson.Type), IsFree: lesson.IsFree, Order: lesson.Order,
				DurationMinutes: lesson.DurationMinutes, ExamID: stringOrEmpty(lesson.ExamID),
				Locked:      !isEnrolled && !lesson.IsFree,
				Attachments: lesson.Attachments,
			})
		}
	}
	sort.SliceStable(lessons, func(i, j int) bool { return lessons[i].Order < lessons[j].Order })
	var enrollmentView any
	if enrollment != nil {
		enrollmentView = enrollment
	}
	progressValue := 0.0
	completedRequiredExams, requiredExams := int64(0), int64(0)
	completedCourseQuizzes, requiredCourseQuizzes := int64(0), int64(0)
	if enrollment != nil {
		progressValue, _ = enrollment.Progress.Float64()
		completedRequiredExams, requiredExams = courseRequiredExamCompletion(userIDString, subject.ID)
		completedCourseQuizzes, requiredCourseQuizzes = courseRequiredQuizCompletion(userIDString, subject.ID)
	}
	api_response.Success(c, gin.H{
		"subject":    subject,
		"enrollment": enrollmentView,
		"lessons":    lessons,
		"progress":   progress,
		"access":     gin.H{"isEnrolled": isEnrolled},
		"completion": gin.H{
			"isComplete":             isEnrolled && progressValue >= 100,
			"progress":               progressValue,
			"certificateEligible":    isEnrolled && progressValue >= 100 && subject.HasCertificate,
			"completedRequiredExams": completedRequiredExams,
			"requiredExams":          requiredExams,
			"completedCourseQuizzes": completedCourseQuizzes,
			"requiredCourseQuizzes":  requiredCourseQuizzes,
		},
	})
}

func renderLmsCourseDetail(c *gin.Context, database *gorm.DB, course models.LmsCourse) {
	userIDString := c.GetString("userId")
	var enrollment *models.LmsEnrollment
	if userIDString != "" {
		if userID, err := uuid.Parse(userIDString); err == nil {
			var row models.LmsEnrollment
			if database.Where("user_id = ? AND course_id = ?", userID, course.ID).First(&row).Error == nil {
				enrollment = &row
			}
		}
	}

	isEnrolled := enrollment != nil
	type progressView struct {
		Completed bool `json:"completed"`
	}
	progress := map[string]progressView{}
	lessons := make([]Lesson, 0)
	for _, section := range course.Sections {
		for _, lesson := range section.Lessons {
			lessons = append(lessons, Lesson{
				ID: lesson.ID.String(), Title: lesson.Title, Content: stringOrEmpty(lesson.Content),
				VideoUrl: stringOrEmpty(lesson.MediaURL), Type: string(lesson.Type),
				IsFree: lesson.IsFreePreview, Order: lesson.OrderIndex,
				DurationMinutes: lesson.DurationSeconds / 60,
				Locked:          !isEnrolled && !lesson.IsFreePreview,
			})
		}
	}
	sort.SliceStable(lessons, func(i, j int) bool { return lessons[i].Order < lessons[j].Order })

	progressValue := 0.0
	if enrollment != nil {
		progressValue, _ = enrollment.Progress.Float64()
	}
	api_response.Success(c, gin.H{
		"course":     course,
		"enrollment": enrollment,
		"lessons":    lessons,
		"progress":   progress,
		"access":     gin.H{"isEnrolled": isEnrolled},
		"completion": gin.H{
			"isComplete":          isEnrolled && progressValue >= 100,
			"progress":            progressValue,
			"certificateEligible": isEnrolled && progressValue >= 100 && course.HasCertificate,
		},
	})
}
