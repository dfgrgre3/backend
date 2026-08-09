package api

import (
	aidelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	analyticsdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	coursedelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	gamificationdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"
	notificationdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	paymentdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	systemdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	userdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"thanawy-backend/internal/infrastructure/api/middleware"

	"github.com/gin-gonic/gin"
)

const (
	pathTasks         = "/tasks"
	pathTasksID       = pathTasks + "/:id"
	pathUpload        = "/upload"
	pathUploadChunked = pathUpload + "/chunked"
)

// SetupProtectedRoutes configures protected API endpoints
func SetupProtectedRoutes(router *gin.Engine) {
	protected := router.Group("/api")
	protected.Use(middleware.Auth())
	protected.Use(middleware.Idempotency())
	{
		// Sub-group for Admin-only routes.
		// AdminRequired() runs first, followed by StrictRBAC() to enforce Deny-by-Default.
		adminProtectedRoutes := protected.Group("")
		adminProtectedRoutes.Use(middleware.AdminRequired())
		adminProtectedRoutes.Use(middleware.StrictRBAC())
		{
			adminProtectedRoutes.POST("/billing/wallet", paymentdelivery.HandleWalletDeposit)
		}

		// General endpoints accessible to any authenticated user.
		// AnyAuthenticatedUser() runs first, followed by StrictRBAC() to enforce Deny-by-Default.
		userRoutes := protected.Group("")
		userRoutes.Use(middleware.AnyAuthenticatedUser())
		userRoutes.Use(middleware.StrictRBAC())
		{
			userRoutes.GET("/progress/summary", handlers.GetProgressSummary)
			userRoutes.GET("/analytics/weekly", handlers.GetWeeklyAnalytics)
			userRoutes.GET("/analytics/time", analyticsdelivery.GetTimeAnalytics)
			userRoutes.GET("/analytics/performance", handlers.GetWeeklyAnalytics)
			userRoutes.GET("/analytics/predictions", handlers.GetWeeklyAnalytics)
			userRoutes.GET("/recommendations", aidelivery.GetAIRecommendations)

			// Protected Activity routes
			userRoutes.GET("/schedule", analyticsdelivery.GetSchedule)
			userRoutes.GET("/lessons", coursedelivery.GetLessons)
			userRoutes.POST("/lessons", coursedelivery.CreateLesson)
			userRoutes.GET(pathTasks, analyticsdelivery.GetTasks)
			userRoutes.GET("/study-sessions", analyticsdelivery.GetStudySessions)
			userRoutes.GET("/reminders", analyticsdelivery.GetReminders)
			userRoutes.POST("/schedule", analyticsdelivery.UpdateSchedule)
			userRoutes.POST(pathTasks, analyticsdelivery.CreateTask)
			userRoutes.PATCH(pathTasksID, analyticsdelivery.UpdateTask)
			userRoutes.PUT(pathTasksID, analyticsdelivery.UpdateTask)
			userRoutes.DELETE(pathTasksID, analyticsdelivery.DeleteTask)
			userRoutes.POST("/study-sessions", analyticsdelivery.CreateStudySession)
			userRoutes.POST("/reminders", analyticsdelivery.CreateReminder)

			// Notifications
			userRoutes.GET("/notifications", notificationdelivery.GetNotifications)
			userRoutes.GET("/notifications/unread-count", systemdelivery.GetUnreadNotificationsCount)
			userRoutes.POST("/notifications/mark-read", notificationdelivery.MarkNotificationRead)
			userRoutes.POST("/notifications/enqueue", notificationdelivery.CreateNotificationTask)

			// Settings
			userRoutes.GET("/settings/preferences", systemdelivery.GetSettings)
			userRoutes.PATCH("/settings/preferences", systemdelivery.UpdateSettings)

			// Profile
			userRoutes.GET("/users/billing-summary", paymentdelivery.GetBillingSummary)
			userRoutes.GET("/users/profile", userdelivery.GetUserProfile)
			userRoutes.PATCH("/users/profile", userdelivery.UpdateProfile)
			userRoutes.GET("/users/progress/courses", handlers.GetUserCoursesProgress)
			userRoutes.GET("/users/progress/time", handlers.GetUserTimeProgress)
			userRoutes.GET("/users/progress/achievements", gamificationdelivery.GetUserAchievementsProgress)

			// Activities
			userRoutes.GET("/activities/recent", systemdelivery.GetRecentActivities)
			userRoutes.POST("/activities/:id/read", analyticsdelivery.MarkActivityRead)
			userRoutes.POST("/activities/read-all", analyticsdelivery.MarkAllActivitiesRead)

			// Billing & Subscriptions
			userRoutes.GET("/billing/wallet", systemdelivery.GetWalletBalance)
			userRoutes.GET("/billing/wallet/transactions", paymentdelivery.GetUserWalletTransactions)
			userRoutes.GET("/subscriptions/plans", paymentdelivery.GetSubscriptionPlans)
			userRoutes.GET("/subscriptions", paymentdelivery.GetUserSubscription)
			userRoutes.GET("/subscriptions/addons", paymentdelivery.GetSubscriptionAddons)
			userRoutes.POST("/subscriptions/addons", paymentdelivery.PurchaseAddon)
			userRoutes.POST("/subscriptions/purchase", paymentdelivery.PurchasePlan)
			userRoutes.POST("/subscriptions/initiate-payment", paymentdelivery.InitiatePlanPayment)
			userRoutes.POST("/subscriptions/cancel", paymentdelivery.CancelSubscription)
			userRoutes.POST("/subscriptions/renew", paymentdelivery.RenewSubscription)
			userRoutes.POST("/coupons/validate", systemdelivery.ValidateCoupon)

			// User Subjects & Courses
			userRoutes.GET("/subjects", coursedelivery.GetUserSubjects)
			userRoutes.GET("/my-courses", coursedelivery.GetMyCourses)

			// Library
			userRoutes.GET("/library/books", systemdelivery.GetLibraryBooks)
			userRoutes.POST("/library/books", systemdelivery.CreateLibraryBook)

			// Enrollment & Progress
			userRoutes.POST("/courses/:id/enroll", coursedelivery.EnrollCourse)
			userRoutes.DELETE("/courses/:id/enroll", handlers.UnenrollCourse)
			userRoutes.GET("/courses/:id/enrollment-status", handlers.GetEnrollmentStatus)
			userRoutes.POST("/courses/:id/complete", handlers.CompleteCourse)
			userRoutes.POST("/courses/:id/checkout", coursedelivery.CourseCheckout)
			userRoutes.GET("/courses/:id/curriculum", coursedelivery.GetSubjectCurriculum)
			userRoutes.POST("/courses/lessons/:id/progress", coursedelivery.UpdateLessonProgress)
			userRoutes.POST("/courses/lessons/:id/view", coursedelivery.TrackLessonView) // Track view stats

			// Phase 2: Advanced lesson access (drip, protection, eligibility)
			userRoutes.GET("/courses/:id/lessons/:lessonId", coursedelivery.ProtectedLessonContent)
			userRoutes.GET("/courses/:id/lessons/:lessonId/eligibility", coursedelivery.GetLessonEligibility)
			userRoutes.GET("/courses/:id/lessons/access", coursedelivery.GetCourseLessonsWithAccess)

			// Lesson Notes & Reviews
			userRoutes.GET("/courses/lessons/:id/notes", systemdelivery.GetLessonNotes)
			userRoutes.POST("/courses/lessons/:id/notes", coursedelivery.CreateLessonNote)
			userRoutes.POST("/courses/:id/reviews", coursedelivery.CreateCourseReview)

			// Upload
			userRoutes.POST("/upload/presign", handlers.PresignUpload)
			userRoutes.POST(pathUpload, handlers.Upload)
			userRoutes.DELETE(pathUpload, handlers.DeleteUpload)
			userRoutes.POST(pathUploadChunked, handlers.UploadChunked)
			userRoutes.PUT(pathUploadChunked, handlers.UploadChunked)
			userRoutes.PATCH(pathUploadChunked, handlers.UploadChunked)
			userRoutes.GET("/upload/chunked/:uploadId/status", handlers.GetUploadStatus)

			// Exam routes
			userRoutes.POST("/exams/:id/submit", handlers.SubmitExam)

			// Gamification routes
			userRoutes.GET("/gamification/progress", gamificationdelivery.GetUserProgress)
			userRoutes.GET("/gamification/achievements", gamificationdelivery.GetUserAchievements)
			userRoutes.POST("/gamification/goals", gamificationdelivery.CreateCustomGoal)
			userRoutes.PATCH("/gamification/goals/:id", gamificationdelivery.UpdateCustomGoal)

			// Event Ingestion (lightweight, fire-and-forget to Redis Stream)
			userRoutes.POST("/events/ingest", analyticsdelivery.IngestEvent)

			// Payment routes
			userRoutes.POST("/payments/create", paymentdelivery.CreatePayment)
			userRoutes.GET("/payments/history", paymentdelivery.GetPaymentHistory)
		}

		// Teaching Dashboard routes (accessible to teachers, admins, and super_admins)
		teachingRoutes := protected.Group("/teaching")
		teachingRoutes.Use(middleware.TeacherRequired())
		teachingRoutes.Use(middleware.StrictRBAC())
		{
			// Dashboard
			teachingRoutes.GET("/dashboard/stats", userdelivery.GetTeachingDashboardStats)

			// Course management
			teachingRoutes.GET("/courses", userdelivery.TeachingListCourses)
			teachingRoutes.POST("/courses", userdelivery.TeachingCreateCourse)
			teachingRoutes.GET("/courses/:id", userdelivery.TeachingGetCourse)
			teachingRoutes.PATCH("/courses/:id", userdelivery.TeachingUpdateCourse)
			teachingRoutes.DELETE("/courses/:id", userdelivery.TeachingDeleteCourse)

			// Students
			teachingRoutes.GET("/courses/:id/students", userdelivery.TeachingListStudents)
			teachingRoutes.GET("/students", userdelivery.TeachingGetAllStudents)

			// Reviews
			teachingRoutes.GET("/courses/:id/reviews", userdelivery.TeachingListReviews)
			teachingRoutes.GET("/reviews", userdelivery.TeachingGetAllReviews)
			teachingRoutes.POST("/reviews/:id/reply", userdelivery.TeachingReplyToReview)

			// Activities & Notifications
			teachingRoutes.GET("/activities", userdelivery.TeachingGetActivities)
			teachingRoutes.GET("/notifications", userdelivery.TeachingGetNotifications)
			teachingRoutes.POST("/notifications/:id/read", userdelivery.TeachingMarkNotificationRead)
			teachingRoutes.POST("/notifications/read-all", userdelivery.TeachingMarkAllNotificationsRead)

			// Instructor application (for non-teachers who want to become teachers)
			teachingRoutes.POST("/apply", userdelivery.TeachingApplyForInstructor)
		}
	}
}
