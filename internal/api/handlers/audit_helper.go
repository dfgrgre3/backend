package handlers

import (
	"encoding/json"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// LogAudit logs an administrative action asynchronously
func LogAudit(c *gin.Context, action string, resource string, resourceId string, metadata interface{}) {
	userId, _ := c.Get("userId")
	userIdStr := ""
	if userId != nil {
		if id, ok := userId.(string); ok {
			userIdStr = id
		}
	}

	// Detect impersonation and enrich metadata/actor details
	isImpersonating := c.GetBool("isImpersonating")
	if isImpersonating {
		originalAdminID, _ := c.Get("originalAdminId")
		originalAdminIDStr, _ := originalAdminID.(string)

		// Enrich metadata with impersonation details
		enrichedMeta := map[string]interface{}{
			"impersonating":  true,
			"target_user_id": userIdStr,
		}

		if metadata != nil {
			// Try to merge if metadata is a map, otherwise serialize and wrap it
			if metaMap, ok := metadata.(map[string]interface{}); ok {
				for k, v := range metaMap {
					enrichedMeta[k] = v
				}
			} else if metaJson, err := json.Marshal(metadata); err == nil {
				var parsed map[string]interface{}
				if json.Unmarshal(metaJson, &parsed) == nil {
					for k, v := range parsed {
						enrichedMeta[k] = v
					}
				} else {
					enrichedMeta["original_metadata"] = metadata
				}
			} else {
				enrichedMeta["original_metadata"] = metadata
			}
		}

		// Log under the admin ID as the actor, but document the impersonation
		if originalAdminIDStr != "" {
			userIdStr = originalAdminIDStr
		}
		metadata = enrichedMeta
	}

	services.GetAuditService().LogAsync(
		userIdStr,
		action,
		resource,
		resourceId,
		metadata,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
}
