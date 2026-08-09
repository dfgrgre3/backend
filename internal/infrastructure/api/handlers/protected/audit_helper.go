package protected

import (
	"thanawy-backend/internal/infrastructure/api/handlers/shared"

	"github.com/gin-gonic/gin"
)

// LogAudit logs an administrative action asynchronously
func LogAudit(c *gin.Context, action string, resource string, resourceId string, metadata interface{}) {
	shared.LogAudit(c, action, resource, resourceId, metadata)
}
