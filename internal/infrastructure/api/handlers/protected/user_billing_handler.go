package protected

import (
	"net/http"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

func GetBillingSummary(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	uid, ok := userId.(string)
	if !ok || uid == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	cacheKey := billingSummaryCachePrefix + uid

	if checkBillingCaches(c, cacheKey) {
		return
	}

	responseData := fetchBillingData(uid)
	if responseData == nil {
		return
	}

	storeBillingCache(cacheKey, responseData)
	api_response.Success(c, responseData)
}
