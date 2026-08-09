package protected

import (
	"net/http"
	"strings"
	"time"

	apiresponse "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type interactiveQuestion struct {
	ID            string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID      string    `gorm:"type:uuid;column:lesson_id;index" json:"lessonId"`
	Question      string    `gorm:"type:text;column:question" json:"question"`
	Options       string    `gorm:"type:text;column:options" json:"options"`
	CorrectAnswer string    `gorm:"type:text;column:correct_answer" json:"correctAnswer"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (interactiveQuestion) TableName() string {
	return "InteractiveQuestion"
}

func (iq *interactiveQuestion) BeforeCreate(tx *gorm.DB) error {
	if iq.ID == "" {
		iq.ID = uuid.New().String()
	}
	if iq.CreatedAt.IsZero() {
		iq.CreatedAt = time.Now()
	}
	iq.UpdatedAt = iq.CreatedAt
	return nil
}

func GetInteractiveQuestions(c *gin.Context) {
	if db.DB == nil {
		apiresponse.Error(c, http.StatusServiceUnavailable, "Database is temporarily unavailable")
		return
	}

	lessonID := strings.TrimSpace(c.Param("lessonId"))
	if lessonID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Lesson id is required")
		return
	}

	var questions []interactiveQuestion
	if err := db.DB.Where("lesson_id = ?", lessonID).Order("created_at DESC").Find(&questions).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch interactive questions")
		return
	}

	apiresponse.Success(c, gin.H{"questions": questions})
}

func CreateInteractiveQuestion(c *gin.Context) {
	if db.DB == nil {
		apiresponse.Error(c, http.StatusServiceUnavailable, "Database is temporarily unavailable")
		return
	}

	lessonID := strings.TrimSpace(c.Param("lessonId"))
	if lessonID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Lesson id is required")
		return
	}

	var input struct {
		Question      string `json:"question" binding:"required"`
		Options       string `json:"options"`
		CorrectAnswer string `json:"correctAnswer"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	question := interactiveQuestion{
		LessonID:      lessonID,
		Question:      strings.TrimSpace(input.Question),
		Options:       strings.TrimSpace(input.Options),
		CorrectAnswer: strings.TrimSpace(input.CorrectAnswer),
	}
	if err := db.DB.Create(&question).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create interactive question")
		return
	}

	apiresponse.Success(c, gin.H{"question": question})
}

func GetInteractiveQuestion(c *gin.Context) {
	if db.DB == nil {
		apiresponse.Error(c, http.StatusServiceUnavailable, "Database is temporarily unavailable")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Question id is required")
		return
	}

	var question interactiveQuestion
	if err := db.DB.Where("id = ?", id).First(&question).Error; err != nil {
		apiresponse.Error(c, http.StatusNotFound, "Interactive question not found")
		return
	}

	apiresponse.Success(c, gin.H{"question": question})
}

func UpdateInteractiveQuestion(c *gin.Context) {
	if db.DB == nil {
		apiresponse.Error(c, http.StatusServiceUnavailable, "Database is temporarily unavailable")
		return
	}

	var input struct {
		ID            string `json:"id"`
		Question      string `json:"question"`
		Options       string `json:"options"`
		CorrectAnswer string `json:"correctAnswer"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	id := strings.TrimSpace(input.ID)
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Question id is required")
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Question) != "" {
		updates["question"] = strings.TrimSpace(input.Question)
	}
	if input.Options != "" {
		updates["options"] = strings.TrimSpace(input.Options)
	}
	if input.CorrectAnswer != "" {
		updates["correct_answer"] = strings.TrimSpace(input.CorrectAnswer)
	}
	if len(updates) == 0 {
		apiresponse.Error(c, http.StatusBadRequest, "No updates provided")
		return
	}

	if err := db.DB.Model(&interactiveQuestion{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to update interactive question")
		return
	}

	apiresponse.Success(c, gin.H{"message": "Interactive question updated"})
}

func DeleteInteractiveQuestion(c *gin.Context) {
	if db.DB == nil {
		apiresponse.Error(c, http.StatusServiceUnavailable, "Database is temporarily unavailable")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Question id is required")
		return
	}

	if err := db.DB.Where("id = ?", id).Delete(&interactiveQuestion{}).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to delete interactive question")
		return
	}

	apiresponse.Success(c, gin.H{"message": "Interactive question deleted"})
}
