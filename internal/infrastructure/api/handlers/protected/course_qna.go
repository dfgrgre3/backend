package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// isAdminRole reports whether the authenticated request's role is an admin
// role, matching the check used by other handlers in this package (e.g.
// subject_delete.go).
func isAdminRole(c *gin.Context) bool {
	role, exists := c.Get("role")
	return exists && (role == "ADMIN" || role == "SUPER_ADMIN")
}

// GetCourseQuestions returns a course's Q&A, optionally filtered to one
// lesson via ?subTopicId=, each question preloaded with its answers.
func GetCourseQuestions(c *gin.Context) {
	id := c.Param("id")

	var subject models.Subject
	query := db.DB.Select("id").WithContext(c.Request.Context())
	query = applyIDOrSlugQuery(query, id)
	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "resolving subject for questions")
		return
	}

	q := db.DB.WithContext(c.Request.Context()).
		Preload("User").
		Preload("Answers", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at asc")
		}).
		Preload("Answers.User").
		Where("subject_id = ?", subject.ID)

	if subTopicID := c.Query("subTopicId"); subTopicID != "" {
		q = q.Where("sub_topic_id = ?", subTopicID)
	}

	var questions []models.CourseQuestion
	if err := q.Order("created_at DESC").Find(&questions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch questions")
		return
	}

	api_response.Success(c, gin.H{"questions": questions})
}

// CreateCourseQuestion lets an authenticated student ask a question on a
// course, optionally scoped to one lesson.
func CreateCourseQuestion(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	subjectId := c.Param("id")
	var subject models.Subject
	query := db.DB.Select("id").WithContext(c.Request.Context())
	query = applyIDOrSlugQuery(query, subjectId)
	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, subjectId, err, "resolving subject for question creation")
		return
	}

	var body struct {
		Title      string  `json:"title" binding:"required,min=3,max=300"`
		Body       string  `json:"body" binding:"max=5000"`
		SubTopicID *string `json:"subTopicId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	question := models.CourseQuestion{
		SubjectID:  subject.ID,
		SubTopicID: body.SubTopicID,
		UserID:     userId,
		Title:      body.Title,
		Body:       body.Body,
	}
	if err := SafeCreate(db.DB, &question); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create question")
		return
	}

	api_response.Success(c, gin.H{"question": question})
}

// CreateCourseAnswer lets an authenticated user (student or the course's
// instructor) reply to a question. IsInstructorAnswer is derived server-side
// by comparing the caller against the question's course instructor.
func CreateCourseAnswer(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	questionId := c.Param("id")
	var question models.CourseQuestion
	if err := db.DB.WithContext(c.Request.Context()).Where(idQuery, questionId).First(&question).Error; err != nil {
		handleSubjectError(c, questionId, err, "resolving question for answer creation")
		return
	}

	var body struct {
		Body string `json:"body" binding:"required,min=1,max=5000"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	var subject models.Subject
	isInstructor := false
	if err := db.DB.Select("instructor_id").Where(idQuery, question.SubjectID).First(&subject).Error; err == nil {
		isInstructor = subject.InstructorId != nil && *subject.InstructorId == userId
	}

	answer := models.CourseAnswer{
		QuestionID:         question.ID,
		UserID:             userId,
		Body:                body.Body,
		IsInstructorAnswer: isInstructor,
	}
	if err := SafeCreate(db.DB, &answer); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create answer")
		return
	}

	api_response.Success(c, gin.H{"answer": answer})
}

// DeleteCourseQuestion soft-deletes a question. Only the question's author
// or an admin may delete it.
func DeleteCourseQuestion(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	questionId := c.Param("id")
	var question models.CourseQuestion
	if err := db.DB.Where(idQuery, questionId).First(&question).Error; err != nil {
		handleSubjectError(c, questionId, err, "resolving question for deletion")
		return
	}

	if question.UserID != userId && !isAdminRole(c) {
		api_response.Error(c, http.StatusForbidden, "You are not allowed to delete this question")
		return
	}

	if err := db.DB.Delete(&models.CourseQuestion{}, idQuery, questionId).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete question")
		return
	}

	api_response.Success(c, gin.H{"deleted": true})
}

// DeleteCourseAnswer soft-deletes an answer. Only the answer's author or an
// admin may delete it.
func DeleteCourseAnswer(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}

	answerId := c.Param("id")
	var answer models.CourseAnswer
	if err := db.DB.Where(idQuery, answerId).First(&answer).Error; err != nil {
		handleSubjectError(c, answerId, err, "resolving answer for deletion")
		return
	}

	if answer.UserID != userId && !isAdminRole(c) {
		api_response.Error(c, http.StatusForbidden, "You are not allowed to delete this answer")
		return
	}

	if err := db.DB.Delete(&models.CourseAnswer{}, idQuery, answerId).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete answer")
		return
	}

	api_response.Success(c, gin.H{"deleted": true})
}
