package router

import (
	"github.com/gin-gonic/gin"
	"thanawy-backend/internal/app"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
)

const (
	hexUserByIDRoute          = "/hex/users/:id"
	hexSubjectsRoute          = "/hex/subjects"
	hexSubjectByIDRoute       = hexSubjectsRoute + "/:id"
	hexSubjectBySlugRoute     = hexSubjectsRoute + "/slug/:slug"
	hexSubjectCurriculumRoute = hexSubjectByIDRoute + "/curriculum"
)

// SetupHexagonalRoutes configures routes using the new Hexagonal Architecture handlers
// This runs alongside legacy routes for gradual migration
func SetupHexagonalRoutes(router *gin.Engine, handlers *app.Handlers) {
	if handlers == nil {
		return
	}

	// User Management (Hexagonal)
	admin := router.Group("/api/admin")
	admin.Use(middleware.Auth())
	admin.Use(middleware.AdminRequired())
	{
		// Replace legacy user routes with hexagonal handler
		admin.GET("/hex/users", middleware.PermissionRequired(models.PermUsersView), handlers.UserHandler.ListUsers)
		admin.GET(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersView), handlers.UserHandler.GetUser)
		admin.POST("/hex/users", middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.CreateUser)
		admin.PATCH(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.UpdateUser)
		admin.DELETE(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.DeleteUser)
	}

	// Subject Management (Hexagonal)
	{
		// Public subject routes
		public := router.Group("/api")
		public.GET(hexSubjectsRoute, handlers.SubjectHandler.ListSubjects)
		public.GET(hexSubjectByIDRoute, handlers.SubjectHandler.GetSubject)
		public.GET(hexSubjectBySlugRoute, handlers.SubjectHandler.GetSubject)

		// Admin subject routes
		admin := router.Group("/api/admin")
		admin.Use(middleware.Auth())
		admin.Use(middleware.AdminRequired())
		{
			admin.GET(hexSubjectsRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.ListSubjects)
			admin.GET(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetSubject)
			admin.POST(hexSubjectsRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.CreateSubject)
			admin.PATCH(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.UpdateSubject)
			admin.DELETE(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.DeleteSubject)
			admin.GET(hexSubjectCurriculumRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetCurriculum)
			admin.PATCH(hexSubjectCurriculumRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.UpdateCurriculum)
		}
	}
}
