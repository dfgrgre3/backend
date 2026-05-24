package handlers

import (
	"net/http"

	"thanawy-backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// GetMetricsEndpoint returns performance metrics in JSON format
// This is an admin-protected endpoint for monitoring dashboards
func GetMetricsEndpoint(c *gin.Context) {
	accept := c.GetHeader("Accept")

	// If Prometheus format is requested, return that
	if accept == "text/plain" || c.Query("format") == "prometheus" {
		c.String(http.StatusOK, middleware.GetMetricsPrometheus())
		return
	}

	// Default: return JSON metrics
	c.JSON(http.StatusOK, middleware.GetMetrics())
}