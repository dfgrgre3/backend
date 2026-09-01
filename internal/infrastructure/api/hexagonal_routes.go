package api

import (
	"thanawy-backend/internal/application"
	"thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/api/middleware"

	"github.com/gin-gonic/gin"
)

// SetupHexagonalRoutes configures routes using the new Hexagonal Architecture handlers
// This runs alongside legacy routes for gradual migration
func SetupHexagonalRoutes(router *gin.Engine, handlers *application.Handlers) {
	if handlers == nil {
		return
	}

	// ============================================================================
	// New Course Domain Routes (CQRS Architecture)
	// These use the new CourseRESTHandler with domain-driven design
	// ============================================================================

	if handlers.CourseRESTHandler != nil {
		courseAdmin := router.Group("/api/v1/admin/courses")
		courseAdmin.Use(middleware.Auth())
		courseAdmin.Use(middleware.AdminOrModerator())
		{
			// Course CRUD
			courseAdmin.GET("", handlers.CourseRESTHandler.ListCourses)
			courseAdmin.POST("", handlers.CourseRESTHandler.CreateCourse)
			courseAdmin.GET("/check-slug", handlers.CourseRESTHandler.CheckSlug)
			courseAdmin.GET("/meta", handlers.CourseRESTHandler.GetCourseMeta)
			courseAdmin.GET("/:id", handlers.CourseRESTHandler.GetCourse)
			// AdminListCourseReviews reads the legacy CourseReview table (the one
			// real students actually write to via CreateCourseReview /
			// GetCourseReviews), not the hexagonal LmsReview table the removed
			// CourseRESTHandler.GetReviews bound here — see project bug notes.
			courseAdmin.GET("/:id/reviews", protected.AdminListCourseReviews)
			courseAdmin.PATCH("/:id/reviews", protected.AdminSetReviewVisibility)
			courseAdmin.DELETE("/:id/reviews", protected.AdminDeleteReview)
			courseAdmin.PATCH("/:id", handlers.CourseRESTHandler.UpdateCourse)
			courseAdmin.DELETE("/:id", handlers.CourseRESTHandler.DeleteCourse)

			// Course Workflow
			courseAdmin.POST("/:id/submit-review", handlers.CourseRESTHandler.SubmitForReview)
			courseAdmin.POST("/:id/approve", handlers.CourseRESTHandler.ApproveCourse)
			courseAdmin.POST("/:id/reject", handlers.CourseRESTHandler.RejectCourse)
			courseAdmin.POST("/:id/archive", handlers.CourseRESTHandler.ArchiveCourse)
			courseAdmin.POST("/:id/unarchive", handlers.CourseRESTHandler.UnarchiveCourse)

			// Course Review Comments
			courseAdmin.GET("/:id/review-comments", handlers.CourseRESTHandler.ListCourseReviewComments)
			courseAdmin.POST("/:id/review-comments", handlers.CourseRESTHandler.AddCourseReviewComment)

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

			// Lesson attachments
			courseAdmin.POST("/:id/sections/:sectionId/lessons/:lessonId/attachments", handlers.CourseRESTHandler.AddLessonAttachment)
			courseAdmin.DELETE("/:id/sections/:sectionId/lessons/:lessonId/attachments/:attachmentId", handlers.CourseRESTHandler.DeleteLessonAttachment)

			// Lesson exam link
			courseAdmin.POST("/:id/sections/:sectionId/lessons/:lessonId/exam", handlers.CourseRESTHandler.LinkLessonExam)
			courseAdmin.DELETE("/:id/sections/:sectionId/lessons/:lessonId/exam", handlers.CourseRESTHandler.UnlinkLessonExam)

			// Assignments (course-scoped catalog + lesson linking)
			courseAdmin.GET("/:id/assignments", handlers.CourseRESTHandler.ListCourseAssignments)
			courseAdmin.POST("/:id/assignments", handlers.CourseRESTHandler.CreateCourseAssignment)
			courseAdmin.DELETE("/:id/assignments/:assignmentId", handlers.CourseRESTHandler.DeleteCourseAssignment)
			courseAdmin.POST("/:id/assignments/:assignmentId/link", handlers.CourseRESTHandler.LinkAssignment)
			courseAdmin.DELETE("/:id/assignments/:assignmentId/link", handlers.CourseRESTHandler.UnlinkAssignment)

			// Instructors (multi-teacher assignment)
			courseAdmin.GET("/:id/instructors", handlers.CourseRESTHandler.ListCourseInstructors)
			courseAdmin.POST("/:id/instructors", handlers.CourseRESTHandler.AddCourseInstructor)
			courseAdmin.DELETE("/:id/instructors/:instructorId", handlers.CourseRESTHandler.RemoveCourseInstructor)

			// Enrollments
			courseAdmin.GET("/:id/enrollments", handlers.CourseRESTHandler.ListEnrollments)
			courseAdmin.POST("/:id/enrollments", handlers.CourseRESTHandler.EnrollUser)
			courseAdmin.GET("/:id/enrollments/:userId", handlers.CourseRESTHandler.GetEnrollment)
			courseAdmin.PATCH("/:id/enrollments/:userId", handlers.CourseRESTHandler.UpdateProgress)

			// Pricing
			courseAdmin.GET("/:id/pricing", handlers.CourseRESTHandler.GetPricing)
			courseAdmin.PUT("/:id/pricing", handlers.CourseRESTHandler.SetPricing)
			// The admin frontend calls POST for this upsert — kept as an alias
			// alongside PUT rather than changing every frontend call site.
			courseAdmin.POST("/:id/pricing", handlers.CourseRESTHandler.SetPricing)

			// Versioning
			courseAdmin.GET("/:id/versions", handlers.CourseRESTHandler.ListCourseVersions)
			courseAdmin.POST("/:id/versions", handlers.CourseRESTHandler.CreateCourseVersion)
			courseAdmin.POST("/:id/versions/:versionId/restore", handlers.CourseRESTHandler.RestoreCourseVersion)
			courseAdmin.GET("/:id/changelog", handlers.CourseRESTHandler.GetCourseChangelog)
		}
	}

	// ============================================================================
	// Certificate Templates Routes (shared, global library)
	// ============================================================================
	if handlers.CourseRESTHandler != nil {
		certAdmin := router.Group("/api/v1/admin/certificates")
		certAdmin.Use(middleware.Auth())
		certAdmin.Use(middleware.AdminOrModerator())
		{
			certAdmin.GET("/templates", handlers.CourseRESTHandler.ListCertificateTemplates)
			certAdmin.POST("/templates", handlers.CourseRESTHandler.CreateCertificateTemplate)
			certAdmin.DELETE("/templates/:templateId", handlers.CourseRESTHandler.DeleteCertificateTemplate)
		}
	}

	// ============================================================================
	// Course Bundles Routes
	// ============================================================================
	if handlers.CourseRESTHandler != nil {
		bundleAdmin := router.Group("/api/v1/admin/bundles")
		bundleAdmin.Use(middleware.Auth())
		bundleAdmin.Use(middleware.AdminOrModerator())
		{
			bundleAdmin.GET("", handlers.CourseRESTHandler.ListBundles)
			bundleAdmin.POST("", handlers.CourseRESTHandler.CreateBundle)
			bundleAdmin.GET("/:id", handlers.CourseRESTHandler.GetBundle)
			bundleAdmin.PATCH("/:id", handlers.CourseRESTHandler.UpdateBundle)
			bundleAdmin.DELETE("/:id", handlers.CourseRESTHandler.DeleteBundle)
			bundleAdmin.POST("/:id/courses", handlers.CourseRESTHandler.AddCoursesToBundle)
			bundleAdmin.DELETE("/:id/courses", handlers.CourseRESTHandler.RemoveCoursesFromBundle)
			bundleAdmin.GET("/:id/enrollments", handlers.CourseRESTHandler.GetBundleEnrollments)
		}
	}
}
