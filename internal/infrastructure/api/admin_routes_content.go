package api

import (
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminContentRoutes registers Teachers, Instructor management and
// Category routes.
func registerAdminContentRoutes(admin *gin.RouterGroup) {
	// Teachers
	admin.GET(adminTeachersRoute, handlers.GetTeachersForAdmin)
	admin.POST(adminTeachersRoute, handlers.CreateTeacher)
	admin.PATCH(adminTeachersRoute, handlers.UpdateTeacher)
	admin.DELETE(adminTeachersRoute, handlers.DeleteTeacher)
	admin.GET(adminTeachersRoute+"/applications", handlers.GetTeacherApplications)
	admin.DELETE(adminTeachersRoute+"/applications", handlers.ReviewTeacherApplication)

	// Instructor management
	admin.GET("/instructors", handlers.GetInstructors)
	admin.POST("/instructors", handlers.CreateInstructor)
	admin.GET("/instructors/statistics", handlers.GetInstructorStatistics)
	admin.GET("/instructors/export", handlers.ExportInstructors)
	admin.POST("/instructors/bulk-delete", handlers.BulkDeleteInstructors)
	admin.POST("/instructors/bulk-notifications", handlers.BulkSendInstructorNotifications)
	admin.GET("/instructors/:id", handlers.GetInstructor)
	admin.PATCH("/instructors/:id", handlers.UpdateInstructor)
	admin.DELETE("/instructors/:id", handlers.DeleteInstructor)
	admin.POST("/instructors/:id/approve", handlers.ApproveInstructor)
	admin.POST("/instructors/:id/reject", handlers.RejectInstructor)
	admin.POST("/instructors/:id/suspend", handlers.SuspendInstructor)
	admin.GET("/instructors/:id/documents", handlers.GetInstructorDocuments)
	admin.POST("/instructors/:id/documents/:documentId/review", handlers.ReviewInstructorDocument)
	admin.GET("/instructors/:id/contracts", handlers.GetInstructorContracts)
	admin.POST("/instructors/:id/contracts", handlers.CreateInstructorContract)
	admin.GET("/instructors/:id/payouts", handlers.GetInstructorPayouts)
	admin.GET("/instructors/:id/performance", handlers.GetInstructorPerformance)
	admin.GET("/instructors/:id/violations", handlers.GetInstructorViolations)
	admin.POST("/instructors/:id/violations", handlers.CreateInstructorViolation)
	admin.POST("/instructors/:id/violations/:violationId/resolve", handlers.ResolveInstructorViolation)
	admin.POST("/instructors/:id/notifications", handlers.SendInstructorNotification)

	// Categories
	admin.GET(adminCourseCategoriesRoute, handlers.GetCategoriesForAdmin)
	admin.POST(adminCourseCategoriesRoute, handlers.CreateCategory)
	admin.PATCH(adminCourseCategoriesRoute, handlers.UpdateCategory)
	admin.DELETE(adminCourseCategoriesRoute, handlers.DeleteCategory)
}
