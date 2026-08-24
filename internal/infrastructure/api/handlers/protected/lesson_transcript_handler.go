package protected

import (
	"net/http"
	"strings"

	models "thanawy-backend/internal/domain/common"
	courseservice "thanawy-backend/internal/domain/course/service"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxTranscriptContentBytes = 2 * 1024 * 1024 // 2MB — generous for an SRT/VTT file

// GetLessonTranscript is student-facing (mounted under /api/courses) — any
// enrolled viewer can read a lesson's transcript, same access level as its
// video and notes.
//
// SECURITY: this used to skip entitlement entirely, so any authenticated
// user (not just enrolled ones) could read the transcript of a paid lesson
// by guessing/observing its lesson id. Now gated by the same
// CheckLessonEligibility check ProtectedLessonContent already enforces for
// the lesson's video/notes.
func GetLessonTranscript(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	lessonID := strings.TrimSpace(c.Param("id"))
	if lessonID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Lesson id is required")
		return
	}

	lessonService := courseservice.NewLessonService()
	eligibility, err := lessonService.CheckLessonEligibility(userID, lessonID)
	if err != nil {
		apiresponse.Error(c, http.StatusNotFound, err.Error())
		return
	}
	if !eligibility.Eligible {
		apiresponse.Error(c, http.StatusForbidden, eligibility.Reason)
		return
	}

	var transcript models.LessonTranscript
	err = db.ReadDB().Where("lesson_id = ?", lessonID).First(&transcript).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			apiresponse.Success(c, gin.H{"content": "", "format": "srt", "language": "ar"})
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to load transcript: "+err.Error())
		return
	}

	apiresponse.Success(c, transcript)
}

// UpsertLessonTranscript is admin/instructor-facing (mounted under
// /api/admin) — replaces the lesson's transcript with the uploaded content.
func UpsertLessonTranscript(c *gin.Context) {
	lessonID := strings.TrimSpace(c.Param("lessonId"))
	if lessonID == "" {
		lessonID = strings.TrimSpace(c.Param("id"))
	}
	if lessonID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Lesson id is required")
		return
	}

	var input struct {
		Content  string `json:"content" binding:"required"`
		Format   string `json:"format"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	if len(input.Content) > maxTranscriptContentBytes {
		apiresponse.Error(c, http.StatusBadRequest, "Transcript content is too large")
		return
	}

	format := strings.ToLower(strings.TrimSpace(input.Format))
	if format != "vtt" {
		format = "srt"
	}
	language := strings.TrimSpace(input.Language)
	if language == "" {
		language = "ar"
	}

	transcript := models.LessonTranscript{
		LessonID: lessonID,
		Format:   format,
		Content:  input.Content,
		Language: language,
	}

	if err := db.WriteDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "lesson_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"format":     format,
			"content":    input.Content,
			"language":   language,
			"updated_at": gorm.Expr("now()"),
		}),
	}).Create(&transcript).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to save transcript: "+err.Error())
		return
	}

	apiresponse.Success(c, transcript)
}

// DeleteLessonTranscript removes a lesson's transcript (admin/instructor-facing).
func DeleteLessonTranscript(c *gin.Context) {
	lessonID := strings.TrimSpace(c.Param("lessonId"))
	if lessonID == "" {
		lessonID = strings.TrimSpace(c.Param("id"))
	}
	if lessonID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Lesson id is required")
		return
	}

	if err := db.WriteDB().Where("lesson_id = ?", lessonID).Delete(&models.LessonTranscript{}).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to delete transcript: "+err.Error())
		return
	}

	apiresponse.Success(c, nil)
}
