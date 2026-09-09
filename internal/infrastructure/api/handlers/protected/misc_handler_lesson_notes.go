package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	gormclause "gorm.io/gorm/clause"
)

func GetLessonNotes(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}
	lessonId := c.Param("id")
	if !lessonBelongsToEnrolledCourse(c, userId, lessonId) {
		return
	}

	var note models.LessonNoteContent
	err := db.ReadDB().
		Where("user_id = ? AND lesson_id = ?", userId, lessonId).
		First(&note).Error

	if err != nil {
		// No notes saved yet — not an error, just empty content.
		api_response.Success(c, gin.H{"content": ""})
		return
	}

	api_response.Success(c, gin.H{"content": note.Content, "updatedAt": note.UpdatedAt})
}

func CreateLessonNote(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, authRequired)
		return
	}
	lessonId := c.Param("id")
	if !lessonBelongsToEnrolledCourse(c, userId, lessonId) {
		return
	}

	var input struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	note := models.LessonNoteContent{
		UserID:   userId,
		LessonID: lessonId,
		Content:  input.Content,
	}

	if err := db.WriteDB().Clauses(gormclause.OnConflict{
		Columns: []gormclause.Column{{Name: "user_id"}, {Name: "lesson_id"}},
		DoUpdates: gormclause.Assignments(map[string]interface{}{
			"content":    input.Content,
			"updated_at": time.Now(),
		}),
	}).Create(&note).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save lesson notes", err)
		return
	}

	api_response.Created(c, gin.H{"success": true})
}

// lessonBelongsToEnrolledCourse is deliberately resolved server-side from
// SubTopic -> Topic -> Subject. A lesson id alone must never be enough to
// read or write a user's private notes.
func lessonBelongsToEnrolledCourse(c *gin.Context, userID, lessonID string) bool {
	var lesson models.SubTopic
	if err := db.ReadDB().Preload("Topic").Where("id = ?", lessonID).First(&lesson).Error; err != nil || lesson.Topic == nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return false
	}

	var enrollment models.Enrollment
	if err := db.ReadDB().Where("user_id = ? AND subject_id = ?", userID, lesson.Topic.SubjectID).First(&enrollment).Error; err != nil {
		api_response.Error(c, http.StatusForbidden, "Course enrollment is required")
		return false
	}
	return true
}
