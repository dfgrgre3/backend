package protected

import (
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// GetSubscriptionPlans is defined in subscription_handler.go
// This file now delegates to that implementation

func GlobalSearch(c *gin.Context) {
	api_response.Success(c, gin.H{
		"results": []interface{}{},
	})
}

func GetLibraryBooks(c *gin.Context) {
	api_response.Success(c, gin.H{
		"books": []interface{}{},
	})
}
