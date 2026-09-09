package protected

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type courseQuizInput struct {
	ID                     *string         `json:"id"`
	LessonID               *string         `json:"lessonId"`
	Title                  string          `json:"title" binding:"required"`
	Description            string          `json:"description"`
	Instructions           string          `json:"instructions"`
	TimeLimitMinutes       *int            `json:"timeLimitMinutes"`
	PassingScore           float64         `json:"passingScore"`
	MaxAttempts            int             `json:"maxAttempts"`
	Required               bool            `json:"required"`
	ShuffleQuestions       bool            `json:"shuffleQuestions"`
	ShuffleOptions         bool            `json:"shuffleOptions"`
	ShowResultsImmediately bool            `json:"showResultsImmediately"`
	ShowCorrectAnswers     bool            `json:"showCorrectAnswers"`
	AllowReview            bool            `json:"allowReview"`
	Status                 string          `json:"status"`
	Questions              json.RawMessage `json:"questions"`
}

func currentQuizUser(c *gin.Context) (string, bool) {
	userID, ok := getAuthenticatedUserID(c)
	return userID, ok
}

func quizCourseAccess(c *gin.Context, courseID, userID string) bool {
	role, _ := c.Get("role")
	roleName, _ := role.(string)
	if roleName == string(models.RoleAdmin) || roleName == string(models.RoleSuperAdmin) {
		return true
	}
	var subject models.Subject
	if db.DB.Where("id = ?", courseID).First(&subject).Error == nil && subject.InstructorId != nil && *subject.InstructorId == userID {
		return true
	}
	var enrollment models.Enrollment
	return db.DB.Where("user_id = ? AND subject_id = ?", userID, courseID).First(&enrollment).Error == nil
}

func quizViewerCanSeeUnpublished(c *gin.Context, courseID, userID string) bool {
	role, _ := c.Get("role")
	roleName, _ := role.(string)
	if roleName == string(models.RoleAdmin) || roleName == string(models.RoleSuperAdmin) {
		return true
	}

	var subject models.Subject
	return db.DB.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error == nil
}

func courseQuizQueryForViewer(c *gin.Context, courseID, userID string) *gorm.DB {
	query := db.ReadDB().Where("course_id = ?", courseID)
	if !quizViewerCanSeeUnpublished(c, courseID, userID) {
		query = query.Where("status = ?", "published")
	}
	return query
}

func findCourseQuizForViewer(c *gin.Context, courseID, quizID, userID string) (models.CourseQuiz, error) {
	var quiz models.CourseQuiz
	err := courseQuizQueryForViewer(c, courseID, userID).
		Where("id = ?", quizID).
		First(&quiz).Error
	return quiz, err
}

func publicQuestion(question map[string]interface{}) map[string]interface{} {
	for _, key := range []string{
		"isCorrect", "referenceAnswer", "graderNotes", "gradingMethod",
		"blanks",
		"correctOption", "correctOptionId", "correctAnswer", "acceptedAnswers",
		"answer", "answers", "orderItems", "matchPairs", "matchTarget",
	} {
		delete(question, key)
	}
	if options, ok := question["options"].([]interface{}); ok {
		for _, option := range options {
			if object, ok := option.(map[string]interface{}); ok {
				delete(object, "isCorrect")
				delete(object, "matchTarget")
			}
		}
	}
	return question
}

func publicQuiz(quiz models.CourseQuiz) models.CourseQuiz {
	var questions []map[string]interface{}
	if json.Unmarshal(quiz.Questions, &questions) == nil {
		for _, question := range questions {
			publicQuestion(question)
		}
		quiz.Questions, _ = json.Marshal(questions)
	}
	return quiz
}

func GetCourseQuizzes(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID := c.Param("id")
	if !quizCourseAccess(c, courseID, userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	var quizzes []models.CourseQuiz
	query := courseQuizQueryForViewer(c, courseID, userID)
	if err := query.Order("created_at ASC").Find(&quizzes).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch quizzes")
		return
	}
	for i := range quizzes {
		quizzes[i] = publicQuiz(quizzes[i])
	}
	api_response.Success(c, quizzes)
}

// GetLessonCourseQuizzes returns only quizzes attached to one lesson. This
// keeps lesson playback from downloading the entire course quiz collection.
func GetLessonCourseQuizzes(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID, lessonID := c.Param("id"), c.Param("lessonId")
	if !quizCourseAccess(c, courseID, userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	var quizzes []models.CourseQuiz
	query := courseQuizQueryForViewer(c, courseID, userID).Where("lesson_id = ?", lessonID)
	if err := query.Order("created_at ASC").Find(&quizzes).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch lesson quizzes")
		return
	}
	for i := range quizzes {
		quizzes[i] = publicQuiz(quizzes[i])
	}
	api_response.Success(c, quizzes)
}

func GetCourseQuiz(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID, quizID := c.Param("id"), c.Param("quizId")
	if !quizCourseAccess(c, courseID, userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	quiz, err := findCourseQuizForViewer(c, courseID, quizID, userID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz not found")
		return
	}
	api_response.Success(c, publicQuiz(quiz))
}

func CreateCourseQuiz(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID := c.Param("id")
	var subject models.Subject
	if err := db.DB.Where("id = ?", courseID).First(&subject).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}
	if subject.InstructorId == nil || *subject.InstructorId != userID {
		api_response.Error(c, http.StatusForbidden, "Only the course instructor can manage quizzes")
		return
	}
	var input courseQuizInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid quiz payload")
		return
	}
	if len(input.Questions) == 0 {
		input.Questions = json.RawMessage("[]")
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
	if input.LessonID != nil {
		var existing int64
		if err := db.ReadDB().Model(&models.CourseQuiz{}).
			Where("course_id = ? AND lesson_id = ?", courseID, *input.LessonID).
			Count(&existing).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to validate lesson quiz")
			return
		}
		if existing > 0 {
			api_response.Error(c, http.StatusConflict, "A lesson can have only one quiz")
			return
		}
	}
	quiz := models.CourseQuiz{CourseID: courseID, LessonID: input.LessonID, Title: strings.TrimSpace(input.Title), Description: input.Description, Instructions: input.Instructions, TimeLimitMinutes: input.TimeLimitMinutes, PassingScore: input.PassingScore, MaxAttempts: input.MaxAttempts, Required: input.Required, ShuffleQuestions: input.ShuffleQuestions, ShuffleOptions: input.ShuffleOptions, ShowResultsImmediately: input.ShowResultsImmediately, ShowCorrectAnswers: input.ShowCorrectAnswers, AllowReview: input.AllowReview, Status: input.Status, Questions: input.Questions, CreatedBy: userID}
	if err := db.WriteDB().Create(&quiz).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create quiz")
		return
	}
	api_response.Created(c, quiz)
}

func UpdateCourseQuiz(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	var quiz models.CourseQuiz
	if err := db.DB.Where("id = ? AND course_id = ?", c.Param("quizId"), c.Param("id")).First(&quiz).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz not found")
		return
	}
	var subject models.Subject
	if err := db.DB.Where("id = ?", quiz.CourseID).First(&subject).Error; err != nil || subject.InstructorId == nil || *subject.InstructorId != userID {
		api_response.Error(c, http.StatusForbidden, "Only the course instructor can manage quizzes")
		return
	}
	var input courseQuizInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid quiz payload")
		return
	}
	if input.LessonID != nil && (quiz.LessonID == nil || *quiz.LessonID != *input.LessonID) {
		var existing int64
		if err := db.ReadDB().Model(&models.CourseQuiz{}).
			Where("course_id = ? AND lesson_id = ? AND id <> ?", quiz.CourseID, *input.LessonID, quiz.ID).
			Count(&existing).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to validate lesson quiz")
			return
		}
		if existing > 0 {
			api_response.Error(c, http.StatusConflict, "A lesson can have only one quiz")
			return
		}
	}
	updates := map[string]interface{}{"title": strings.TrimSpace(input.Title), "description": input.Description, "instructions": input.Instructions, "passing_score": input.PassingScore, "max_attempts": input.MaxAttempts, "required": input.Required, "shuffle_questions": input.ShuffleQuestions, "shuffle_options": input.ShuffleOptions, "show_results_immediately": input.ShowResultsImmediately, "show_correct_answers": input.ShowCorrectAnswers, "allow_review": input.AllowReview, "status": input.Status}
	if input.LessonID != nil {
		updates["lesson_id"] = input.LessonID
	}
	if input.TimeLimitMinutes != nil {
		updates["time_limit_minutes"] = input.TimeLimitMinutes
	}
	if len(input.Questions) > 0 {
		updates["questions"] = input.Questions
	}
	if err := db.WriteDB().Model(&quiz).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update quiz")
		return
	}
	if err := db.ReadDB().Where("id = ?", quiz.ID).First(&quiz).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load updated quiz")
		return
	}
	api_response.Success(c, quiz)
}

func StartCourseQuiz(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID, quizID := c.Param("id"), c.Param("quizId")
	if !quizCourseAccess(c, courseID, userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	quiz, err := findCourseQuizForViewer(c, courseID, quizID, userID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz not found")
		return
	}
	var activeAttempt models.CourseQuizAttempt
	if err := db.ReadDB().Where("quiz_id = ? AND user_id = ? AND status = ?", quiz.ID, userID, "in_progress").Order("started_at DESC").First(&activeAttempt).Error; err == nil {
		deadline := ""
		if quiz.TimeLimitMinutes != nil {
			deadline = activeAttempt.StartedAt.Add(time.Duration(*quiz.TimeLimitMinutes) * time.Minute).Format(time.RFC3339)
		}
		api_response.Success(c, gin.H{"attemptId": activeAttempt.ID, "startedAt": activeAttempt.StartedAt, "deadline": deadline})
		return
	}
	var attempts int64
	db.ReadDB().Model(&models.CourseQuizAttempt{}).Where("quiz_id = ? AND user_id = ?", quiz.ID, userID).Count(&attempts)
	if quiz.MaxAttempts > 0 && attempts >= int64(quiz.MaxAttempts) {
		api_response.Error(c, http.StatusBadRequest, "Maximum quiz attempts exceeded")
		return
	}
	now := time.Now().UTC()
	attempt := models.CourseQuizAttempt{QuizID: quiz.ID, CourseID: courseID, UserID: userID, Answers: json.RawMessage("[]"), Status: "in_progress", StartedAt: now}
	if err := db.WriteDB().Create(&attempt).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to start quiz")
		return
	}
	deadline := ""
	if quiz.TimeLimitMinutes != nil {
		deadline = now.Add(time.Duration(*quiz.TimeLimitMinutes) * time.Minute).Format(time.RFC3339)
	}
	api_response.Success(c, gin.H{"attemptId": attempt.ID, "startedAt": now, "deadline": deadline})
}

type quizSubmission struct {
	AttemptID        string                   `json:"attemptId"`
	Answers          []map[string]interface{} `json:"answers"`
	TimeSpentSeconds int                      `json:"timeSpentSeconds"`
}

func SubmitCourseQuiz(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	courseID, quizID := c.Param("id"), c.Param("quizId")
	if !quizCourseAccess(c, courseID, userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	quiz, err := findCourseQuizForViewer(c, courseID, quizID, userID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz not found")
		return
	}
	var submission quizSubmission
	if err := c.ShouldBindJSON(&submission); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid submission")
		return
	}
	if submission.AttemptID == "" {
		api_response.Error(c, http.StatusBadRequest, "attemptId is required")
		return
	}
	var attempt models.CourseQuizAttempt
	if err := db.DB.Where("id = ? AND quiz_id = ? AND user_id = ?", submission.AttemptID, quiz.ID, userID).First(&attempt).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz attempt not found")
		return
	}
	if attempt.Status != "in_progress" {
		api_response.Error(c, http.StatusBadRequest, "Quiz attempt is no longer active")
		return
	}
	if quiz.TimeLimitMinutes != nil && time.Now().UTC().After(attempt.StartedAt.Add(time.Duration(*quiz.TimeLimitMinutes)*time.Minute+5*time.Second)) {
		api_response.Error(c, http.StatusBadRequest, "Quiz deadline exceeded")
		return
	}
	var questions []map[string]interface{}
	if err := json.Unmarshal(quiz.Questions, &questions); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Invalid quiz questions")
		return
	}
	answersByQuestion := map[string]map[string]interface{}{}
	for _, answer := range submission.Answers {
		if id, ok := answer["questionId"].(string); ok {
			answersByQuestion[id] = answer
		}
	}
	var score, maxScore float64
	items := make([]map[string]interface{}, 0, len(questions))
	for _, question := range questions {
		points, _ := question["points"].(float64)
		maxScore += points
		id, _ := question["id"].(string)
		answer := answersByQuestion[id]
		correct := false
		if options, ok := question["options"].([]interface{}); ok && answer != nil {
			selected, _ := answer["selectedOptionIds"].([]interface{})
			for _, option := range options {
				if object, ok := option.(map[string]interface{}); ok && object["isCorrect"] == true {
					for _, selectedID := range selected {
						if selectedID == object["id"] {
							correct = true
						}
					}
				}
			}
		}
		if correct {
			score += points
		}
		items = append(items, map[string]interface{}{"question": publicQuestion(question), "answer": answer, "isCorrect": correct, "pointsEarned": map[bool]float64{true: points, false: 0}[correct], "pointsPossible": points})
	}
	percentage := float64(0)
	if maxScore > 0 {
		percentage = score / maxScore * 100
	}
	passed := percentage >= quiz.PassingScore
	answersJSON, _ := json.Marshal(submission.Answers)
	attempt.Answers = answersJSON
	attempt.Score = score
	attempt.MaxScore = maxScore
	attempt.Percentage = percentage
	attempt.Passed = passed
	attempt.Status = "graded"
	attempt.TimeSpentSeconds = submission.TimeSpentSeconds
	attempt.SubmittedAt = time.Now().UTC()
	attempt.GradedAt = time.Now().UTC()
	if err := db.WriteDB().Save(&attempt).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save quiz submission")
		return
	}
	var courseProgress float64
	var courseComplete bool
	certificateEligible := false
	if passed && quiz.LessonID != nil {
		progress := models.LessonProgress{UserID: userID, LessonID: *quiz.LessonID, Completed: true, Status: models.ProgressStatusCompleted}
		if err := db.WriteDB().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "sub_topic_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"completed": true, "status": models.ProgressStatusCompleted, "updated_at": time.Now().UTC()}),
		}).Create(&progress).Error; err != nil {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update lesson completion", err)
			return
		}
	}
	if passed {
		var err error
		courseProgress, courseComplete, err = refreshEnrollmentProgress(userID, quiz.CourseID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update course progress", err)
			return
		}
		var subject models.Subject
		if err := db.ReadDB().Select("has_certificate").First(&subject, "id = ?", quiz.CourseID).Error; err == nil {
			certificateEligible = courseComplete && subject.HasCertificate
		}
	}
	api_response.Success(c, gin.H{
		"attempt": attempt,
		"quiz":    publicQuiz(quiz),
		"items":   items,
		"completion": gin.H{
			"lessonCompleted":     passed && quiz.LessonID != nil,
			"courseProgress":      courseProgress,
			"courseCompleted":     courseComplete,
			"certificateEligible": certificateEligible,
		},
	})
}

func GetCourseQuizResults(c *gin.Context) {
	userID, ok := currentQuizUser(c)
	if !ok {
		return
	}
	if !quizCourseAccess(c, c.Param("id"), userID) {
		api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course")
		return
	}
	if _, err := findCourseQuizForViewer(c, c.Param("id"), c.Param("quizId"), userID); err != nil {
		api_response.Error(c, http.StatusNotFound, "Quiz not found")
		return
	}
	var attempts []models.CourseQuizAttempt
	err := db.ReadDB().Where("quiz_id = ? AND course_id = ? AND user_id = ?", c.Param("quizId"), c.Param("id"), userID).Order("submitted_at DESC").Find(&attempts).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch quiz results")
		return
	}
	attemptsUsed := len(attempts)
	attemptsRemaining := quizMaxAttempts(c.Param("quizId"), attemptsUsed)
	var latestAttempt *models.CourseQuizAttempt
	var bestAttempt *models.CourseQuizAttempt
	for index := range attempts {
		attempt := &attempts[index]
		if latestAttempt == nil {
			latestAttempt = attempt
		}
		if bestAttempt == nil || attempt.Percentage > bestAttempt.Percentage {
			bestAttempt = attempt
		}
	}
	api_response.Success(c, gin.H{
		"attempts":          attempts,
		"attemptsUsed":      attemptsUsed,
		"attemptsRemaining": attemptsRemaining,
		"canRetake":         attemptsRemaining > 0 && (bestAttempt == nil || !bestAttempt.Passed),
		"hasPassed":         bestAttempt != nil && bestAttempt.Passed,
		"latestAttempt":     latestAttempt,
		"bestAttempt":       bestAttempt,
	})
}

func quizMaxAttempts(quizID string, attemptsUsed int) int {
	var quiz models.CourseQuiz
	if db.ReadDB().Select("max_attempts").First(&quiz, "id = ?", quizID).Error != nil {
		return 0
	}
	remaining := quiz.MaxAttempts - attemptsUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}
