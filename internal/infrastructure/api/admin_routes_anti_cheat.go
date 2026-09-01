package api

import (
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminAntiCheatRoutes registers the Anti-Cheat (مكافحة الغش) admin
// endpoints: aggregated review cases, raw proctoring events, and the manual
// event-recording hook (which upserts the corresponding flag).
func registerAdminAntiCheatRoutes(admin *gin.RouterGroup) {
	admin.GET("/anti-cheat", handlers.AdminListAntiCheatFlags)
	admin.GET("/anti-cheat/stats", handlers.AdminAntiCheatStats)
	admin.GET("/anti-cheat/flags/:id", handlers.AdminGetAntiCheatFlag)
	admin.PATCH("/anti-cheat/flags/:id", handlers.AdminUpdateAntiCheatFlag)
	admin.GET("/anti-cheat/events", handlers.AdminListAntiCheatEvents)
	admin.POST("/anti-cheat/events", handlers.AdminRecordAntiCheatEvent)
}
