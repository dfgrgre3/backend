package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	gamificationservice "thanawy-backend/internal/domain/gamification/service"
	gamificationrepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

var achievementService = gamificationservice.NewAchievementService(gamificationrepo.NewAchievementRepository())

func AdminGetAchievements(c *gin.Context) {
	achievements, err := achievementService.GetAllAchievements(c.Request.Context())
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch achievements")
		return
	}
	api_response.Success(c, achievements)
}

func AdminCreateAchievement(c *gin.Context) {
	var achievement models.Achievement
	if err := c.ShouldBindJSON(&achievement); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := achievementService.CreateAchievement(c.Request.Context(), &achievement); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "Achievement already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create achievement")
		return
	}

	LogAudit(c, "CREATE", "achievement", achievement.ID, achievement)
	api_response.Created(c, achievement)
}

func AdminUpdateAchievement(c *gin.Context) {
	id := c.Param("id")
	var achievement models.Achievement
	if err := c.ShouldBindJSON(&achievement); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if achievement.Key != "" {
		updates["key"] = achievement.Key
	}
	if achievement.Title != "" {
		updates["title"] = achievement.Title
	}
	if achievement.Description != "" {
		updates["description"] = achievement.Description
	}
	if achievement.Icon != "" {
		updates["icon"] = achievement.Icon
	}
	if achievement.Rarity != "" {
		updates["rarity"] = achievement.Rarity
	}
	if achievement.XpReward > 0 {
		updates["xp_reward"] = achievement.XpReward
	}
	updates["is_secret"] = achievement.IsSecret
	if achievement.Category != "" {
		updates["category"] = achievement.Category
	}
	if achievement.Difficulty != "" {
		updates["difficulty"] = achievement.Difficulty
	}

	updatedAchievement, err := achievementService.UpdateAchievement(c.Request.Context(), id, updates)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update achievement")
		return
	}

	LogAudit(c, "UPDATE", "achievement", id, updates)
	api_response.Success(c, updatedAchievement)
}

func AdminDeleteAchievement(c *gin.Context) {
	id := c.Param("id")
	if err := achievementService.DeleteAchievement(c.Request.Context(), id); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete achievement")
		return
	}
	LogAudit(c, "DELETE", "achievement", id, nil)
	api_response.Success(c, nil)
}
