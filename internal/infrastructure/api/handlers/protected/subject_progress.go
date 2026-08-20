package protected

import (
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

	var input struct {
		Completed           bool    `json:"completed"`
		LastWatchedPosition float64 `json:"lastWatchedPosition"`
		TimeSpentSeconds    int     `json:"timeSpentSeconds"`
		Status              string  `json:"status"`
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
		TimeSpentSeconds:    input.TimeSpentSeconds,
		Status:              progressStatus,
	}

	// Write to database using WriteDB for CQRS write path
	if err := db.WriteDB().Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "sub_topic_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"completed":             input.Completed,
			"last_watched_position": input.LastWatchedPosition,
			"time_spent_seconds":    gorm.Expr("time_spent_seconds + ?", input.TimeSpentSeconds),
			"status":                progressStatus,
			"updated_at":            time.Now(),
		}),
	}).Create(&progress).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save lesson progress: "+err.Error())
		return
	}

	api_response.Success(c, nil)
}
