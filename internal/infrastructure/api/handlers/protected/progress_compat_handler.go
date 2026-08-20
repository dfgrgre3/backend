package protected

import (
	"log/slog"
	"net/http"

	gamificationservice "thanawy-backend/internal/domain/gamification/service"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// TimeProgressResponse returns the learner's overall time, focus, and streak metrics.
type TimeProgressResponse struct {
	TotalMinutes   int     `json:"totalMinutes"`
	AverageFocus   float64 `json:"averageFocus"`
	TasksCompleted int64   `json:"tasksCompleted"`
	StreakDays     int     `json:"streakDays"`
}

// AchievementsProgressResponse wraps the achievements list with a total count.
type AchievementsProgressResponse struct {
	Achievements []gamificationservice.UserAchievementReadModel `json:"achievements"`
	Total        int                                            `json:"total"`
}

// GetUserTimeProgress returns the learner's overall time, focus, and streak metrics.
// progressQuery is a package-level service defined in progress_handler.go.
func GetUserTimeProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	summary, err := progressQuery.GetSummary(userID)
	if err != nil {
		slog.Error("failed to fetch time progress", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to fetch time progress")
		return
	}

	apiresponse.Success(c, TimeProgressResponse{
		TotalMinutes:   summary.TotalMinutes,
		AverageFocus:   summary.AverageFocus,
		TasksCompleted: summary.TasksCompleted,
		StreakDays:     summary.StreakDays,
	})
}

// GetUserAchievementsProgress returns the learner's unlocked and locked achievements.
// gamificationQuery is a package-level service defined in gamification_handler.go.
func GetUserAchievementsProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		apiresponse.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	achievements, err := gamificationQuery.GetUserAchievements(userID)
	if err != nil {
		slog.Error("failed to fetch achievements progress", "userID", userID, "error", err)
		apiresponse.Error(c, mapErrorToHTTPStatus(err), "Failed to fetch achievements progress")
		return
	}

	apiresponse.Success(c, AchievementsProgressResponse{
		Achievements: achievements,
		Total:        len(achievements),
	})
}
