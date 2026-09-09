package protected

import (
	"errors"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func UpdateLessonProgress(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	lessonId := c.Param("id")

	var lesson models.SubTopic
	if err := db.WriteDB().Preload("Topic").First(&lesson, "id = ?", lessonId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}

	var enrollment models.Enrollment
	if err := db.WriteDB().Where("user_id = ? AND subject_id = ?", userId, lesson.Topic.SubjectID).First(&enrollment).Error; err != nil {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled to save lesson progress")
		return
	}

	var input struct {
		Completed             bool    `json:"completed"`
		LastWatchedPosition   float64 `json:"lastWatchedPosition"`
		TimeSpentDeltaSeconds int     `json:"timeSpentDeltaSeconds"`
		// Legacy alias kept for older clients during contract migration.
		TimeSpentSeconds int    `json:"timeSpentSeconds"`
		Status           string `json:"status"`
	}
	timeSpentDelta := input.TimeSpentDeltaSeconds
	if timeSpentDelta == 0 {
		timeSpentDelta = input.TimeSpentSeconds
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	progressStatus := models.ProgressStatus(input.Status)
	if progressStatus == "" {
		if input.Completed {
			progressStatus = models.ProgressStatusCompleted
		} else {
			progressStatus = models.ProgressStatusInProgress
		}
	}

	progress := models.LessonProgress{
		UserID:              userId,
		LessonID:            lessonId,
		Completed:           input.Completed,
		LastWatchedPosition: int(input.LastWatchedPosition),
		TimeSpentSeconds:    timeSpentDelta,
		Status:              progressStatus,
	}

	// Write to database using WriteDB for CQRS write path
	if err := db.WriteDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "sub_topic_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"completed":             input.Completed,
			"last_watched_position": input.LastWatchedPosition,
			"time_spent_seconds":    gorm.Expr("time_spent_seconds + ?", timeSpentDelta),
			"status":                progressStatus,
			"updated_at":            time.Now(),
		}),
	}).Create(&progress).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save lesson progress", err)
		return
	}

	// Recompute the enrollment aggregate from persisted lesson progress. The
	// client must never derive official course completion from its local list.
	var totalLessons, completedLessons int64
	lessonIDs := db.WriteDB().Table("SubTopic").
		Select("SubTopic.id").
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("Topic.subject_id = ?", lesson.Topic.SubjectID)
	db.WriteDB().Model(&models.SubTopic{}).Where("id IN (?)", lessonIDs).Count(&totalLessons)
	db.WriteDB().Model(&models.LessonProgress{}).
		Where("user_id = ? AND completed = ? AND sub_topic_id IN (?)", userId, true, lessonIDs).
		Count(&completedLessons)
	completedRequiredExams, requiredExams := courseRequiredExamCompletion(userId, lesson.Topic.SubjectID)
	completedCourseQuizzes, requiredCourseQuizzes := courseRequiredQuizCompletion(userId, lesson.Topic.SubjectID)
	isCourseComplete := courseCompletionSatisfied(totalLessons, completedLessons, requiredExams, completedRequiredExams) && completedCourseQuizzes >= requiredCourseQuizzes

	var courseProgress float64
	if totalLessons > 0 {
		courseProgress = float64(completedLessons) / float64(totalLessons) * 100
		if courseProgress > 99 && !isCourseComplete {
			courseProgress = 99
		}
	}
	if isCourseComplete {
		courseProgress = 100
	}
	var subject models.Subject
	_ = db.ReadDB().Select("has_certificate").Where(idQuery, lesson.Topic.SubjectID).First(&subject).Error
	certificateEligible := isCourseComplete && subject.HasCertificate
	if err := db.WriteDB().Model(&enrollment).Updates(map[string]interface{}{"progress": courseProgress}).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update course progress", err)
		return
	}

	lessonProgress := 0.0
	if input.Completed {
		lessonProgress = 100
	} else if input.LastWatchedPosition > 0 && lesson.DurationMinutes > 0 {
		lessonProgress = float64(input.LastWatchedPosition) / float64(lesson.DurationMinutes*60) * 100
		if lessonProgress > 99 {
			lessonProgress = 99
		}
	}

	api_response.Success(c, gin.H{
		"lessonProgress":         lessonProgress,
		"courseProgress":         courseProgress,
		"isCourseComplete":       isCourseComplete,
		"certificateEligible":    certificateEligible,
		"completedLessons":       completedLessons,
		"totalLessons":           totalLessons,
		"requiredExams":          requiredExams,
		"completedRequiredExams": completedRequiredExams,
		"requiredCourseQuizzes":  requiredCourseQuizzes,
		"completedCourseQuizzes": completedCourseQuizzes,
	})
}

// GetLessonProgress returns the authenticated user's saved progress for a
// lesson, enabling server-authoritative "resume playback" across devices.
func GetLessonProgress(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	lessonId := c.Param("id")

	var progress models.LessonProgress
	err := db.ReadDB().
		Where("user_id = ? AND sub_topic_id = ?", userId, lessonId).
		First(&progress).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No progress yet — not an error, just nothing to resume from.
			api_response.Success(c, gin.H{
				"lastWatchedPosition": 0,
				"completed":           false,
				"status":              models.ProgressStatusNotStarted,
			})
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to load lesson progress", err)
		return
	}

	api_response.Success(c, progress)
}

// refreshEnrollmentProgress is the single server-side aggregate used after
// lesson and quiz completion writes.
func refreshEnrollmentProgress(userID, subjectID string) (float64, bool, error) {
	var enrollment models.Enrollment
	if err := db.WriteDB().Where("user_id = ? AND subject_id = ?", userID, subjectID).First(&enrollment).Error; err != nil {
		return 0, false, err
	}
	var totalLessons, completedLessons int64
	lessonIDs := db.WriteDB().Table("SubTopic").Select("SubTopic.id").Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").Where("Topic.subject_id = ?", subjectID)
	db.WriteDB().Model(&models.SubTopic{}).Where("id IN (?)", lessonIDs).Count(&totalLessons)
	db.WriteDB().Model(&models.LessonProgress{}).Where("user_id = ? AND completed = ? AND sub_topic_id IN (?)", userID, true, lessonIDs).Count(&completedLessons)
	passedExams, requiredExams := courseRequiredExamCompletion(userID, subjectID)
	passedQuizzes, requiredQuizzes := courseRequiredQuizCompletion(userID, subjectID)
	complete := courseCompletionSatisfied(totalLessons, completedLessons, requiredExams, passedExams) && passedQuizzes >= requiredQuizzes
	progress := 0.0
	if totalLessons > 0 {
		progress = float64(completedLessons) / float64(totalLessons) * 100
		if progress > 99 && !complete {
			progress = 99
		}
	}
	if complete {
		progress = 100
	}
	if err := db.WriteDB().Model(&enrollment).Updates(map[string]interface{}{"progress": progress}).Error; err != nil {
		return 0, false, err
	}
	return progress, complete, nil
}
