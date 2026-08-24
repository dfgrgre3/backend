package protected

import (
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func CreateExam(c *gin.Context) {
	var input struct {
		Title     string `json:"title" binding:"required"`
		SubjectID string `json:"subjectId" binding:"required"`
		Year      int    `json:"year"`
		URL       string `json:"url"`
		Type      string `json:"type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exam := models.Exam{
		SubjectID: input.SubjectID,
		Title:     input.Title,
		Type:      models.ExamType(input.Type),
	}
	if exam.Type == "" {
		exam.Type = models.ExamTypeQuiz
	}

	if err := db.WriteDB().Create(&exam).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create exam")
		return
	}

	analyticsservice.GetAuditService().LogAsync("", analyticsservice.AuditEventAdminAction, "exam", exam.ID, map[string]interface{}{"action": "create", "title": exam.Title}, c.ClientIP(), c.Request.UserAgent())

	api_response.Success(c, gin.H{"success": true, "exam": exam})
}

func UpdateExam(c *gin.Context) {
	var input struct {
		ID        string `json:"id" binding:"required"`
		Title     string `json:"title"`
		SubjectID string `json:"subjectId"`
		Type      string `json:"type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exam models.Exam
	if err := db.ReadDB().Take(&exam, idQuery, input.ID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Exam not found")
		return
	}

	type examUpdates struct {
		Title     *string `gorm:"column:title"`
		SubjectID *string `gorm:"column:subject_id"`
		Type      *string `gorm:"column:type"`
	}

	updates := examUpdates{}
	if input.Title != "" {
		updates.Title = &input.Title
	}
	if input.SubjectID != "" {
		updates.SubjectID = &input.SubjectID
	}
	if input.Type != "" {
		updates.Type = &input.Type
	}

	if err := db.WriteDB().Model(&models.Exam{}).Where(idQuery, exam.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update exam")
		return
	}

	analyticsservice.GetAuditService().LogAsync("", analyticsservice.AuditEventAdminAction, "exam", exam.ID, map[string]interface{}{"action": "update", "updates": updates}, c.ClientIP(), c.Request.UserAgent())

	api_response.Success(c, gin.H{"success": true})
}

func DeleteExam(c *gin.Context) {
	var input struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.WriteDB().Delete(&models.Exam{}, idQuery, input.ID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete exam")
		return
	}

	analyticsservice.GetAuditService().LogAsync("", analyticsservice.AuditEventDataDeletion, "exam", input.ID, map[string]interface{}{"action": "delete"}, c.ClientIP(), c.Request.UserAgent())

	api_response.Success(c, gin.H{"success": true})
}
