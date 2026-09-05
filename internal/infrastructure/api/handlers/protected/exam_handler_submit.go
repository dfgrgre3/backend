package protected

import (
	"encoding/json"
	"fmt"
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SubmitExam(c *gin.Context) {
	userIDVal, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	examID := c.Param("id")

	var submission struct {
		Answers map[string]string `json:"answers"`
	}

	if err := c.ShouldBindJSON(&submission); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid submission")
		return
	}

	// ---- READ: Fetch exam from read replica ----
	var exam models.Exam
	if err := db.ReadDB().Preload("Questions", func(d *gorm.DB) *gorm.DB {
		return d.Select("id", "answer")
	}).Take(&exam, idQuery, examID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Exam not found")
		return
	}

	// ---- AUTHORIZATION: Entitlement & Resource Ownership Check ----
	// The backend MUST enforce fine-grained access: Verify if the user is enrolled in the subject.
	roleVal, _ := c.Get("role")
	roleStr, _ := roleVal.(string)
	isAdmin := roleStr == string(models.RoleAdmin) || roleStr == string(models.RoleSuperAdmin)

	if !isAdmin && exam.SubjectID != "" {
		var enrollment models.Enrollment
		if err := db.DB.Where("user_id = ? AND subject_id = ?", userID, exam.SubjectID).First(&enrollment).Error; err != nil {
			api_response.Error(c, http.StatusForbidden, "You must be enrolled in this course to take this exam")
			return
		}
	}

	// ---- BUSINESS LOGIC: Grade the exam (no DB) ----
	correctCount := 0
	totalQuestions := len(exam.Questions)
	for _, question := range exam.Questions {
		userAnswer, ok := submission.Answers[question.ID]
		if !ok {
			continue
		}
		if userAnswer == question.Answer {
			correctCount++
		}
	}

	var score float64
	if totalQuestions > 0 {
		score = (float64(correctCount) / float64(totalQuestions)) * exam.MaxScore
	}
	passed := score >= (exam.MaxScore * 0.5)

	// ---- WRITE: Save result using write source ----
	answersJSON, _ := json.Marshal(submission.Answers)
	result := models.ExamResult{
		UserID:  userID,
		ExamID:  examID,
		Score:   score,
		Passed:  passed,
		Answers: string(answersJSON),
		TakenAt: time.Now(),
	}

	if err := db.WithWriteTx(func(tx *gorm.DB) error {
		return tx.Create(&result).Error
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save result")
		return
	}

	analyticsservice.GetAuditService().LogAsync(userID, analyticsservice.AuditEventExamFinished, "exam", examID,
		map[string]interface{}{"score": score, "passed": passed}, c.ClientIP(), c.Request.UserAgent())

	statusStr := "فشل"
	if passed {
		statusStr = "نجح"
	}
	GlobalNotifyAdmins("اكتمال اختبار",
		fmt.Sprintf("أكمل المستخدم اختبار %s بنتيجة %.1f (%s)", exam.Title, score, statusStr), "info")

	api_response.Success(c, result)
}
