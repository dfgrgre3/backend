package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"

	"github.com/gin-gonic/gin"
)

// registerAdminBadgeCMSRoutes registers Badges, Attendance, CMS Pages and
// Integrations management routes.
func registerAdminBadgeCMSRoutes(admin, sensitive *gin.RouterGroup) {
	// -------------------------------
	// Badges Management
	// -------------------------------
	admin.GET("/badges", admindelivery.AdminListBadges)
	admin.GET("/badges/:id", admindelivery.AdminGetBadge)
	admin.POST("/badges", admindelivery.AdminCreateBadge)
	admin.PATCH("/badges/:id", admindelivery.AdminUpdateBadge)
	admin.DELETE("/badges/:id", admindelivery.AdminDeleteBadge)

	// -------------------------------
	// Attendance Management
	// -------------------------------
	admin.GET("/attendance", admindelivery.AdminListAttendance)
	admin.GET("/attendance/stats", admindelivery.AdminGetAttendanceStats)
	admin.POST("/attendance", admindelivery.AdminCreateAttendance)
	admin.PATCH("/attendance/:id", admindelivery.AdminUpdateAttendance)

	// -------------------------------
	// CMS Pages Management
	// -------------------------------
	admin.GET("/cms/pages", admindelivery.AdminListCMSPages)
	admin.GET("/cms/pages/:id", admindelivery.AdminGetCMSPage)
	admin.POST("/cms/pages", admindelivery.AdminCreateCMSPage)
	admin.PATCH("/cms/pages/:id", admindelivery.AdminUpdateCMSPage)
	admin.DELETE("/cms/pages/:id", admindelivery.AdminDeleteCMSPage)

	// -------------------------------
	// Integrations Management
	// -------------------------------
	admin.GET("/integrations", admindelivery.AdminListIntegrations)
	admin.GET("/integrations/:id", admindelivery.AdminGetIntegration)
	admin.POST("/integrations", admindelivery.AdminCreateIntegration)
	admin.PATCH("/integrations/:id", admindelivery.AdminUpdateIntegration)
	admin.DELETE("/integrations/:id", admindelivery.AdminDeleteIntegration)
	admin.POST("/integrations/:id/test", admindelivery.AdminTestIntegration)
}
