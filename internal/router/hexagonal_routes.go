package router

import (
	"thanawy-backend/internal/app"
	"thanawy-backend/internal/middleware"

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
