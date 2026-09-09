package protected

import (
	"encoding/json"
	"errors"
	"strings"

	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// persistTeachingQuiz is deliberately transaction-scoped. Course authoring
// must not commit a curriculum while its quiz is only half-saved.
func persistTeachingQuizzes(tx *gorm.DB, courseID, userID string, quizzes []*courseQuizInput, legacy *courseQuizInput) error {
	if len(quizzes) == 0 && legacy != nil {
		quizzes = []*courseQuizInput{legacy}
	}
	for _, quiz := range quizzes {
		if err := persistTeachingQuiz(tx, courseID, userID, quiz); err != nil {
			return err
		}
	}
	return nil
}

func persistTeachingQuiz(tx *gorm.DB, courseID, userID string, input *courseQuizInput) error {
	if input == nil || len(input.Questions) == 0 {
		return nil
	}
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("quiz title is required")
	}
	if input.PassingScore <= 0 {
		input.PassingScore = 60
	}
	if input.MaxAttempts <= 0 {
		input.MaxAttempts = 1
	}
	if input.Status == "" {
		input.Status = "draft"
	}

	lessonID, err := resolveTeachingQuizLesson(tx, courseID, input.LessonID)
	if err != nil {
		return err
	}

	quiz := models.CourseQuiz{
		CourseID: courseID, LessonID: lessonID, Title: strings.TrimSpace(input.Title),
		Description: input.Description, Instructions: input.Instructions,
		TimeLimitMinutes: input.TimeLimitMinutes, PassingScore: input.PassingScore,
		MaxAttempts: input.MaxAttempts, Required: input.Required,
		ShuffleQuestions: input.ShuffleQuestions, ShuffleOptions: input.ShuffleOptions,
		ShowResultsImmediately: input.ShowResultsImmediately,
		ShowCorrectAnswers:     input.ShowCorrectAnswers, AllowReview: input.AllowReview,
		Status: input.Status, Questions: json.RawMessage(input.Questions), CreatedBy: userID,
	}

	if input.ID != nil && strings.TrimSpace(*input.ID) != "" && uuid.Validate(strings.TrimSpace(*input.ID)) == nil {
		var existing models.CourseQuiz
		if err := tx.Where("id = ? AND course_id = ?", strings.TrimSpace(*input.ID), courseID).First(&existing).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"lesson_id": lessonID, "title": quiz.Title, "description": quiz.Description,
			"instructions": quiz.Instructions, "time_limit_minutes": quiz.TimeLimitMinutes,
			"passing_score": quiz.PassingScore, "max_attempts": quiz.MaxAttempts,
			"required": quiz.Required, "shuffle_questions": quiz.ShuffleQuestions,
			"shuffle_options": quiz.ShuffleOptions, "show_results_immediately": quiz.ShowResultsImmediately,
			"show_correct_answers": quiz.ShowCorrectAnswers, "allow_review": quiz.AllowReview,
			"status": quiz.Status, "questions": quiz.Questions,
		}
		return tx.Model(&existing).Updates(updates).Error
	}

	// No ID means a new quiz. A course can therefore own one quiz per QUIZ
	// lesson; updates must carry the persisted quiz ID explicitly.
	return tx.Create(&quiz).Error
}

func resolveTeachingQuizLesson(tx *gorm.DB, courseID string, requested *string) (*string, error) {
	if requested == nil || strings.TrimSpace(*requested) == "" {
		return nil, errors.New("quiz must be attached to an explicit QUIZ lesson")
	}
	var lesson models.SubTopic
	if err := tx.Table("sub_topic").Joins("JOIN topic ON topic.id = sub_topic.topic_id").Where("sub_topic.id = ? AND topic.subject_id = ? AND sub_topic.type = ?", strings.TrimSpace(*requested), courseID, models.SubTopicQuiz).First(&lesson).Error; err != nil {
		return nil, errors.New("quiz lessonId must reference a QUIZ lesson in this course")
	}
	return &lesson.ID, nil
}
