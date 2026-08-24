package admin

import (
	"net/http"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"time"

	"github.com/gin-gonic/gin"
)

func AdminCourseAction(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		api_response.Success(c, nil)
	case http.MethodGet:
		api_response.Success(c, nil)
	default:
		api_response.Error(c, http.StatusMethodNotAllowed, shared.MsgMethodNotAllowed)
	}
}

func DatabasePartitions(c *gin.Context) {
	api_response.Success(c, gin.H{
		"status": "healthy",
		"health": gin.H{
			"status":          "healthy",
			"checkedAt":       time.Now(),
			"partitioned":     false,
			"needsAction":     false,
			"tables":          []gin.H{},
			"recommendations": []string{},
		},
		"data": gin.H{
			"status": "healthy",
			"tables": []gin.H{},
		},
	})
}
