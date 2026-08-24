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
		return
	}
	lessonId := c.Param("id")

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
		return
	}
	lessonId := c.Param("id")

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
		api_response.Error(c, http.StatusInternalServerError, "Failed to save lesson notes: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"success": true})
}
