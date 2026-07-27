package router

import (
	"thanawy-backend/internal/app"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	hexUserByIDRoute          = "/hex/users/:id"
	hexSubjectsRoute          = "/hex/subjects"
	hexSubjectByIDRoute       = hexSubjectsRoute + "/:id"
	hexSubjectBySlugRoute     = hexSubjectsRoute + "/slug/:slug"
	hexSubjectCurriculumRoute = hexSubjectByIDRoute + "/curriculum"
	hexSubjectLessonsRoute    = hexSubjectByIDRoute + "/lessons"
	hexSubjectReviewsRoute    = hexSubjectByIDRoute + "/reviews"
	hexSubjectEnrollRoute     = hexSubjectByIDRoute + "/enroll"
	hexSubjectStatusRoute     = hexSubjectByIDRoute + "/enrollment-status"
	hexSubjectCompleteRoute   = hexSubjectByIDRoute + "/complete"
	hexLessonProgressRoute    = "/hex/courses/lessons/:id/progress"
	hexMyCoursesRoute         = "/hex/my-courses"
	hexCourseStudentsRoute    = hexSubjectByIDRoute + "/students"
	hexCourseDuplicateRoute   = "/hex/courses/duplicate"
	hexCourseBatchRoute       = "/hex/courses/batch"
	hexDashboardStatsRoute    = "/hex/dashboard/stats"

	// New Course Domain Routes (CQRS Architecture)
	hexCoursesRoute           = "/hex/v3/courses"
	hexCourseByIDRoute        = hexCoursesRoute + "/:id"
	hexCourseSectionsRoute    = hexCourseByIDRoute + "/sections"
	hexCourseSectionByIDRoute = hexCourseSectionsRoute + "/:sectionId"
	hexCourseLessonsRoute     = hexCourseSectionByIDRoute + "/lessons"
	hexCourseLessonByIDRoute  = hexCourseLessonsRoute + "/:lessonId"
	hexCourseEnrollmentsRoute = hexCourseByIDRoute + "/enrollments"
	hexCourseEnrollmentRoute  = hexCourseEnrollmentsRoute + "/:userId"
	hexCoursePricingRoute     = hexCourseByIDRoute + "/pricing"
)

// SetupHexagonalRoutes configures routes using the new Hexagonal Architecture handlers
// This runs alongside legacy routes for gradual migration
func SetupHexagonalRoutes(router *gin.Engine, handlers *app.Handlers) {
	if handlers == nil {
		return
	}

	// User Management (Hexagonal)
	adminGroup := router.Group("/api/admin")
	adminGroup.Use(middleware.Auth())
	adminGroup.Use(middleware.AdminOrModerator())
	{
		// Replace legacy user routes with hexagonal handler
		adminGroup.GET("/hex/users", middleware.PermissionRequired(models.PermUsersView), handlers.UserHandler.ListUsers)
		adminGroup.GET(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersView), handlers.UserHandler.GetUser)
		adminGroup.POST("/hex/users", middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.CreateUser)
		adminGroup.PATCH(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.UpdateUser)
		adminGroup.DELETE(hexUserByIDRoute, middleware.PermissionRequired(models.PermUsersManage), handlers.UserHandler.DeleteUser)
	}

	// ============================================================================
	// Subject Management (Hexagonal) - Public & Protected Routes
	// ============================================================================

	// Public subject routes (no auth required)
	public := router.Group("/api")
	{
		public.GET(hexSubjectsRoute, handlers.SubjectHandler.ListSubjects)
		public.GET(hexSubjectByIDRoute, handlers.SubjectHandler.GetSubject)
		public.GET(hexSubjectBySlugRoute, handlers.SubjectHandler.GetSubject)
		public.GET(hexSubjectLessonsRoute, handlers.SubjectHandler.GetCourseLessons)
		public.GET(hexSubjectReviewsRoute, handlers.SubjectHandler.GetReviews)
		public.GET(hexSubjectCurriculumRoute, handlers.SubjectHandler.GetCurriculum)
	}

	// Public certificate routes
	certPublic := router.Group("/api")
	certPublic.Use(middleware.Auth())
	certPublic.Use(middleware.AnyAuthenticatedUser())
	{
		certPublic.GET("/hex/certificates/:id", handlers.CertificateHandler.GetCertificate)
		certPublic.GET("/hex/certificates", handlers.CertificateHandler.ListMyCertificates)
		certPublic.GET("/hex/courses/:id/certificate", handlers.CertificateHandler.GetCertificateByCourse)
	}

	// Authenticated routes for course operations
	authGroup := router.Group("/api")
	authGroup.Use(middleware.Auth())
	authGroup.Use(middleware.AnyAuthenticatedUser())
	{
		// Enrollment
		authGroup.POST(hexSubjectEnrollRoute, handlers.SubjectHandler.EnrollCourse)
		authGroup.DELETE(hexSubjectEnrollRoute, handlers.SubjectHandler.UnenrollCourse)
		authGroup.GET(hexSubjectStatusRoute, handlers.SubjectHandler.GetEnrollmentStatus)
		authGroup.POST(hexSubjectCompleteRoute, handlers.SubjectHandler.CompleteCourse)

		// My Courses
		authGroup.GET(hexMyCoursesRoute, handlers.SubjectHandler.GetMyCourses)
		authGroup.GET("/hex/users/subjects", handlers.SubjectHandler.GetUserSubjects)

		// Lesson Progress
		authGroup.POST(hexLessonProgressRoute, handlers.SubjectHandler.UpdateLessonProgress)
		authGroup.GET("/hex/courses/:id/progress", handlers.SubjectHandler.GetLessonProgress)

		// Reviews
		authGroup.POST(hexSubjectReviewsRoute, handlers.SubjectHandler.CreateReview)
	}

	// ============================================================================
	// Admin Subject Routes (ADMIN, SUPER_ADMIN, MODERATOR)
	// ============================================================================
	admin := router.Group("/api/admin")
	admin.Use(middleware.Auth())
	admin.Use(middleware.AdminOrModerator())
	admin.Use(middleware.StrictRBAC())
	{
		// Subject CRUD
		admin.GET(hexSubjectsRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.ListSubjects)
		admin.GET(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetSubject)
		admin.POST(hexSubjectsRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.CreateSubject)
		admin.PATCH(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.UpdateSubject)
		admin.DELETE(hexSubjectByIDRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.DeleteSubject)

		// Curriculum
		admin.GET(hexSubjectCurriculumRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetCurriculum)
		admin.PATCH(hexSubjectCurriculumRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.UpdateCurriculum)
		admin.PUT(hexSubjectCurriculumRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.UpdateCurriculum)

		// Course Students
		admin.GET(hexCourseStudentsRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetCourseStudents)

		// Duplicate & Batch operations
		admin.POST(hexCourseDuplicateRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.DuplicateCourse)
		admin.POST(hexCourseBatchRoute, middleware.PermissionRequired(models.PermSubjectsManage), handlers.SubjectHandler.BatchCourseAction)

		// Certificate Management (Admin)
		admin.POST("/hex/certificates", middleware.PermissionRequired(models.PermSubjectsManage), handlers.CertificateHandler.IssueCertificate)
		admin.GET("/hex/certificates", middleware.PermissionRequired(models.PermSubjectsView), handlers.CertificateHandler.AdminListCertificates)

		// Dashboard Stats
		admin.GET(hexDashboardStatsRoute, middleware.PermissionRequired(models.PermSubjectsView), handlers.SubjectHandler.GetDashboardStats)
	}

	// ============================================================================
	// Course Aliases (for frontend compatibility with /api/courses)
	// These override legacy routes with hexagonal handler
	// ============================================================================

	// Public course routes (override legacy)
	coursesPublic := router.Group("/api")
	{
		coursesPublic.GET("/hex/v2/courses", handlers.SubjectHandler.ListSubjects)
		coursesPublic.GET("/hex/v2/courses/:id", handlers.SubjectHandler.GetSubject)
		coursesPublic.GET("/hex/v2/courses/:id/lessons", handlers.SubjectHandler.GetCourseLessons)
		coursesPublic.GET("/hex/v2/courses/:id/reviews", handlers.SubjectHandler.GetReviews)
		coursesPublic.GET("/hex/v2/courses/:id/curriculum", handlers.SubjectHandler.GetCurriculum)
	}

	// ============================================================================
	// New Course Domain Routes (CQRS Architecture)
	// These use the new CourseRESTHandler with domain-driven design
	// ============================================================================

	if handlers.CourseRESTHandler != nil {
		courseAdmin := router.Group("/api/admin/courses")
		courseAdmin.Use(middleware.Auth())
		courseAdmin.Use(middleware.AdminOrModerator())
		{
			// Course CRUD
			courseAdmin.GET("", handlers.CourseRESTHandler.ListCourses)
			courseAdmin.POST("", handlers.CourseRESTHandler.CreateCourse)
			courseAdmin.GET("/:id", handlers.CourseRESTHandler.GetCourse)
			courseAdmin.GET("/:id/reviews", handlers.CourseRESTHandler.GetReviews)
			courseAdmin.PATCH("/:id", handlers.CourseRESTHandler.UpdateCourse)
			courseAdmin.DELETE("/:id", handlers.CourseRESTHandler.DeleteCourse)

			// Course Workflow
			courseAdmin.POST("/:id/submit-review", handlers.CourseRESTHandler.SubmitForReview)
			courseAdmin.POST("/:id/approve", handlers.CourseRESTHandler.ApproveCourse)
			courseAdmin.POST("/:id/reject", handlers.CourseRESTHandler.RejectCourse)
			courseAdmin.POST("/:id/archive", handlers.CourseRESTHandler.ArchiveCourse)
			courseAdmin.POST("/:id/unarchive", handlers.CourseRESTHandler.UnarchiveCourse)

			// Course Review Queue
			courseAdmin.GET("/pending-review", handlers.CourseRESTHandler.GetCoursesPendingReview)
			courseAdmin.POST("/bulk-status", handlers.CourseRESTHandler.BulkStatusChange)

			// Sections
			courseAdmin.GET("/:id/sections", handlers.CourseRESTHandler.ListSections)
			courseAdmin.POST("/:id/sections", handlers.CourseRESTHandler.CreateSection)
			courseAdmin.POST("/:id/sections/reorder", handlers.CourseRESTHandler.ReorderSections)
			courseAdmin.PATCH("/:id/sections/:sectionId", handlers.CourseRESTHandler.UpdateSection)
			courseAdmin.DELETE("/:id/sections/:sectionId", handlers.CourseRESTHandler.DeleteSection)

			// Lessons
			courseAdmin.GET("/:id/sections/:sectionId/lessons", handlers.CourseRESTHandler.ListLessons)
			courseAdmin.POST("/:id/sections/:sectionId/lessons", handlers.CourseRESTHandler.CreateLesson)
			courseAdmin.POST("/:id/sections/:sectionId/lessons/reorder", handlers.CourseRESTHandler.ReorderLessons)
			courseAdmin.PATCH("/:id/sections/:sectionId/lessons/:lessonId", handlers.CourseRESTHandler.UpdateLesson)
			courseAdmin.DELETE("/:id/sections/:sectionId/lessons/:lessonId", handlers.CourseRESTHandler.DeleteLesson)

			// Enrollments
			courseAdmin.GET("/:id/enrollments", handlers.CourseRESTHandler.GetEnrollment)
			courseAdmin.POST("/:id/enrollments", handlers.CourseRESTHandler.EnrollUser)
			courseAdmin.PATCH("/:id/enrollments/:userId", handlers.CourseRESTHandler.UpdateProgress)
		}
	}
}
