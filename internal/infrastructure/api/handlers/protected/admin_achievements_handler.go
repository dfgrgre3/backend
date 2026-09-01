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

// achievementUpdateRequest uses pointers so partial updates can distinguish
// between "field omitted" and "field set to its zero value".
type achievementUpdateRequest struct {
	Key         *string `json:"key"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Rarity      *string `json:"rarity"`
	XpReward    *int    `json:"xpReward"`
	IsSecret    *bool   `json:"isSecret"`
	Category    *string `json:"category"`
	Difficulty  *string `json:"difficulty"`
	Criteria    *string `json:"criteria"`
}

func AdminUpdateAchievement(c *gin.Context) {
	id := c.Param("id")
	var req achievementUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Key != nil {
		updates["key"] = *req.Key
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Rarity != nil {
		updates["rarity"] = *req.Rarity
	}
	if req.XpReward != nil {
		updates["xp_reward"] = *req.XpReward
	}
	if req.IsSecret != nil {
		updates["is_secret"] = *req.IsSecret
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Difficulty != nil {
		updates["difficulty"] = *req.Difficulty
	}
	if req.Criteria != nil {
		updates["criteria"] = *req.Criteria
	}

	if len(updates) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No valid fields to update")
		return
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
