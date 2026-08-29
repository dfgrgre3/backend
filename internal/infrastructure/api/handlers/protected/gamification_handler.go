package protected

import (
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	gamificationservice "thanawy-backend/internal/domain/gamification/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	gamificationQuery = gamificationservice.NewGamificationQueryService()
)

// resolveGamificationUserID returns the user ID whose gamification data the
// caller may view: their own by default, or another user's only if the
// caller holds an admin/moderator role. Without this check, any
// authenticated user could pass ?userId=<victim> and read another user's
// progress/achievements (BOLA/IDOR) — see GetLessons for the same pattern
// applied elsewhere in this package.
func resolveGamificationUserID(c *gin.Context) (string, bool) {
	authUserIDVal, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return "", false
	}
	authUserID, _ := authUserIDVal.(string)

	userID := c.Query("userId")
	if userID == "" {
		userID = authUserID
	}

	if userID != authUserID {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		isAdmin := roleStr == "ADMIN" || roleStr == "SUPER_ADMIN" || roleStr == "MODERATOR"
		if !isAdmin {
			api_response.Error(c, http.StatusForbidden, "You are not authorized to view this user's data")
			return "", false
		}
	}

	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return "", false
	}

	return userID, true
}

// GetLeaderboard returns the top users by XP
func GetLeaderboard(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}

	entries, err := gamificationQuery.GetLeaderboard(limit)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch leaderboard")
		return
	}

	leaderboard := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		leaderboard = append(leaderboard, gin.H{
			"rank":     e.Rank,
			"id":       e.ID,
			"userId":   e.ID,
			"name":     e.Name,
			"username": e.Name,
			"avatar":   e.Avatar,
			"totalXP":  e.TotalXP,
			"level":    e.Level,
			"role":     e.Role,
		})
	}

	api_response.Success(c, gin.H{
		"leaderboard": leaderboard,
	})
}

// GetUserProgress returns the current gamification progress for a specific user.
func GetUserProgress(c *gin.Context) {
	userID, ok := resolveGamificationUserID(c)
	if !ok {
		return
	}

	progress, err := gamificationQuery.GetUserProgress(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Success(c, gamificationservice.NewDefaultUserProgress(userID))
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch user progress")
		return
	}

	api_response.Success(c, progress)
}

func CreateCustomGoal(c *gin.Context) {
	userID := c.GetString("userId")
	var input struct {
		UserID       string  `json:"userId"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		TargetValue  float64 `json:"targetValue"`
		CurrentValue float64 `json:"currentValue"`
		Unit         string  `json:"unit"`
		Category     string  `json:"category"`
		XPReward     int     `json:"xpReward"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if input.UserID != "" && input.UserID != userID {
		api_response.Error(c, http.StatusForbidden, "Cannot create goals for another user")
		return
	}

	input.Title = strings.TrimSpace(input.Title)
	input.Unit = strings.TrimSpace(input.Unit)
	input.Category = strings.TrimSpace(input.Category)
	if input.Title == "" || input.TargetValue <= 0 {
		api_response.Error(c, http.StatusBadRequest, "title and a positive targetValue are required")
		return
	}
	if input.Unit == "" {
		input.Unit = "units"
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if input.XPReward <= 0 {
		input.XPReward = 10
	}

	now := time.Now()
	goal := models.CustomGoal{
		UserID:       userID,
		Title:        input.Title,
		Description:  strings.TrimSpace(input.Description),
		TargetValue:  decimal.NewFromFloat(input.TargetValue),
		CurrentValue: decimal.NewFromFloat(input.CurrentValue),
		Unit:         input.Unit,
		Category:     input.Category,
		XPReward:     input.XPReward,
	}
	currentValue, _ := goal.CurrentValue.Float64()
	targetValue, _ := goal.TargetValue.Float64()
	if currentValue >= targetValue {
		goal.IsCompleted = true
		goal.CompletedAt = &now
	}

	if err := db.DB.Create(&goal).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create custom goal")
		return
	}
	api_response.Success(c, goal)
}

func UpdateCustomGoal(c *gin.Context) {
	userID := c.GetString("userId")
	var input struct {
		CurrentValue float64 `json:"currentValue"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var goal models.CustomGoal
	if err := db.DB.First(&goal, "id = ? AND \"userId\" = ?", c.Param("id"), userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Custom goal not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch custom goal")
		return
	}

	goal.CurrentValue = decimal.NewFromFloat(input.CurrentValue)
	currentValue, _ := goal.CurrentValue.Float64()
	targetValue, _ := goal.TargetValue.Float64()
	if currentValue >= targetValue {
		if !goal.IsCompleted {
			now := time.Now()
			goal.CompletedAt = &now
		}
		goal.IsCompleted = true
	} else {
		goal.IsCompleted = false
		goal.CompletedAt = nil
	}

	if err := db.DB.Save(&goal).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update custom goal")
		return
	}
	api_response.Success(c, goal)
}

// GetUserAchievements returns achievements for a specific user
func GetUserAchievements(c *gin.Context) {
	userID, ok := resolveGamificationUserID(c)
	if !ok {
		return
	}

	achievements, err := gamificationQuery.GetUserAchievements(userID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch achievements")
		return
	}

	result := make([]gin.H, 0, len(achievements))
	for _, a := range achievements {
		result = append(result, gin.H{
			"id":          a.ID,
			"key":         a.Key,
			"title":       a.Title,
			"description": a.Description,
			"icon":        a.Icon,
			"unlockedAt":  a.UnlockedAt,
			"rarity":      a.Rarity,
			"xpReward":    a.XpReward,
		})
	}

	api_response.Success(c, gin.H{
		"achievements": result,
	})
}
