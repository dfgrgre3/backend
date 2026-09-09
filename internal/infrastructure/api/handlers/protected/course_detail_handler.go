package protected

import (
	"sort"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// GetCourseDetailHydration returns one consistent public/private snapshot for
// the course detail page. Protected media is included only for enrolled users.
func GetCourseDetailHydration(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	var subject models.Subject
	query := database.Preload(preloadAdvanced)
	if err := applyIDOrSlugQuery(query, c.Param("id")).First(&subject).Error; err != nil {
		handleSubjectError(c, c.Param("id"), err, "fetching course detail")
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
