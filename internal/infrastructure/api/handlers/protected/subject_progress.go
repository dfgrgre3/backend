package protected

import (
	"errors"
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
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save lesson progress", err)
		return
	}

	api_response.Success(c, nil)
}

// GetLessonProgress returns the authenticated user's saved progress for a
// lesson, enabling server-authoritative "resume playback" across devices.
func GetLessonProgress(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}
	lessonId := c.Param("id")

	var progress models.LessonProgress
	err := db.ReadDB().
		Where("user_id = ? AND sub_topic_id = ?", userId, lessonId).
		First(&progress).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No progress yet — not an error, just nothing to resume from.
			api_response.Success(c, gin.H{
				"lastWatchedPosition": 0,
				"completed":           false,
				"status":              models.ProgressStatusNotStarted,
			})
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to load lesson progress", err)
		return
	}

	api_response.Success(c, progress)
}
