package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

const errInvalidRequest = "Invalid request"
const errPlanNotFound = "Plan not found"

func GetSubscriptionPlans(c *gin.Context) {
	var plans []models.SubscriptionPlan
	if err := db.DB.Where(isActiveQuery, true).Order("price asc").Find(&plans).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch plans")
		return
	}
	api_response.Success(c, plans)
}

func GetUserSubscription(c *gin.Context) {
	userId, _ := c.Get("userId")

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	if user.ActiveSubscriptionID == nil {
		api_response.Success(c, gin.H{"active": false})
		return
	}

	var sub models.UserSubscription
	if err := db.DB.Preload("Plan").First(&sub, idQuery, *user.ActiveSubscriptionID).Error; err != nil {
		api_response.Success(c, gin.H{"active": false})
		return
	}

	api_response.Success(c, gin.H{
		"active":       true,
		"subscription": sub,
	})
}
