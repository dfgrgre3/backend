package api

import (
	"thanawy-backend/internal/application"
	authservice "thanawy-backend/internal/domain/auth/service"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/persistence/repositories"

	"thanawy-backend/internal/infrastructure/api/middleware"

	"github.com/gin-gonic/gin"
)

const (
	pathTasks         = "/tasks"
	pathTasksID       = pathTasks + "/:id"
	pathUpload        = "/upload"
	pathUploadChunked = pathUpload + "/chunked"
)

// SetupProtectedRoutes configures protected API endpoints.
// hexHandlers provides access to hexagonal-architecture handler instances
// (e.g. the self-service certificate endpoints on CourseRESTHandler) that
// are constructed once during application.Initialize and shared with
// SetupHexagonalRoutes.
func SetupProtectedRoutes(router *gin.Engine, hexHandlers *application.Handlers) {
	protected := router.Group("/api/v1")
	protected.Use(middleware.Auth())
	protected.Use(middleware.Idempotency())

	// Session listing/revocation reuses the token-refresh AuthService - a
	// second lightweight instance (same pattern as public_routes.go's
	// authHandler), since SetupProtectedRoutes doesn't share the one built
	// there. OAuth/mail-queue args are nil/zero-value: this handler only ever
	// calls GetUserSessions/RevokeSession, neither of which touches them.
	sessionHandler := handlers.NewSessionHandler(
		authservice.NewAuthService(repositories.NewAuthRepository(), authservice.NewAuthTokenService(), nil, nil, nil),
	)
	{
		// Sub-group for Admin-only routes.
		// AdminRequired() runs first, followed by StrictRBAC() to enforce Deny-by-Default.
		adminProtectedRoutes := protected.Group("")
		adminProtectedRoutes.Use(middleware.AdminRequired())
		adminProtectedRoutes.Use(middleware.StrictRBAC())
		{
			adminProtectedRoutes.POST("/billing/wallet", handlers.HandleWalletDeposit)
		}

		// General endpoints accessible to any authenticated user.
		// AnyAuthenticatedUser() runs first, followed by StrictRBAC() to enforce Deny-by-Default.
		userRoutes := protected.Group("")
		userRoutes.Use(middleware.AnyAuthenticatedUser())
		userRoutes.Use(middleware.StrictRBAC())
		{
			userRoutes.GET("/progress/summary", handlers.GetProgressSummary)
			userRoutes.GET("/users/progress/courses", handlers.GetUserProgressCourses)
			userRoutes.GET("/analytics/weekly", handlers.GetWeeklyAnalytics)
			userRoutes.GET("/analytics/time", handlers.GetTimeAnalytics)
			userRoutes.GET("/analytics/performance", handlers.GetPerformanceMetrics)
			userRoutes.GET("/analytics/predictions", handlers.GetPredictions)
			userRoutes.GET("/recommendations", handlers.GetRecommendations)
			userRoutes.GET("/tips", handlers.GetTips)

			// Protected Activity routes
			userRoutes.GET("/schedule", handlers.GetSchedule)
			userRoutes.GET("/lessons", handlers.GetLessons)
			userRoutes.POST("/lessons", handlers.CreateLesson)
			userRoutes.GET(pathTasks, handlers.GetTasks)
			userRoutes.GET("/study-sessions", handlers.GetStudySessions)
			userRoutes.GET("/reminders", handlers.GetReminders)
			userRoutes.POST("/schedule", handlers.UpdateSchedule)
			userRoutes.POST(pathTasks, handlers.CreateTask)
			userRoutes.PATCH(pathTasksID, handlers.UpdateTask)
			userRoutes.PUT(pathTasksID, handlers.UpdateTask)
			userRoutes.DELETE(pathTasksID, handlers.DeleteTask)
			userRoutes.POST("/study-sessions", handlers.CreateStudySession)
			userRoutes.POST("/reminders", handlers.CreateReminder)

			// Settings
			userRoutes.GET("/settings/preferences", handlers.GetSettings)
			userRoutes.PATCH("/settings/preferences", handlers.UpdateSettings)
			// Self-service export-data / clear-history (profile > privacy tab).
			userRoutes.POST("/settings/privacy/actions", handlers.PrivacyActions)

			// Sessions ("connected devices" in the profile > security tab).
			userRoutes.GET("/auth/sessions", sessionHandler.ListSessions)
			userRoutes.DELETE("/auth/sessions/:id", sessionHandler.RevokeSession)
			userRoutes.POST("/auth/sessions/revoke-others", sessionHandler.RevokeAllSessions)

			// Profile
			userRoutes.GET("/users/billing-summary", handlers.GetBillingSummary)
			userRoutes.GET("/users/profile", handlers.GetUserProfile)
			userRoutes.PATCH("/users/profile", handlers.UpdateProfile)

			// Activities
			userRoutes.GET("/activities/recent", handlers.GetRecentActivities)
			userRoutes.POST("/activities/:id/read", handlers.MarkActivityRead)
			userRoutes.POST("/activities/read-all", handlers.MarkAllActivitiesRead)

			// Notifications (student-facing bell/feed - built with keyset
			// pagination, L1+Redis caching, and a WebSocket refresh broadcast on
			// mark-read, but never mounted anywhere until now)
			userRoutes.GET("/notifications", handlers.GetNotifications)
			userRoutes.POST("/notifications/mark-read", handlers.MarkNotificationRead)

			// Billing & Subscriptions
			userRoutes.GET("/billing/wallet", handlers.GetWalletBalance)
			userRoutes.GET("/billing/wallet/transactions", handlers.GetUserWalletTransactions)
			userRoutes.GET("/subscriptions/plans", handlers.GetSubscriptionPlans)
			userRoutes.GET("/subscriptions", handlers.GetUserSubscription)
			userRoutes.GET("/subscriptions/addons", handlers.GetSubscriptionAddons)
			userRoutes.POST("/subscriptions/addons", handlers.PurchaseAddon)
			userRoutes.POST("/subscriptions/purchase", handlers.PurchasePlan)
			userRoutes.POST("/subscriptions/initiate-payment", handlers.InitiatePlanPayment)
			userRoutes.POST("/subscriptions/cancel", handlers.CancelSubscription)
			userRoutes.POST("/subscriptions/renew", handlers.RenewSubscription)
			userRoutes.POST("/subscriptions/checkout", handlers.SubscriptionCheckout)
			userRoutes.GET("/invoice/:id", handlers.GetInvoice)
			userRoutes.POST("/coupons/validate", handlers.ValidateCoupon)

			// User Subjects & Courses
			userRoutes.GET("/subjects", handlers.GetUserSubjects)
			userRoutes.GET("/my-courses", handlers.GetMyCourses)

			// Library
			userRoutes.GET("/library/books", handlers.GetLibraryBooks)
			userRoutes.POST("/library/books", handlers.CreateLibraryBook)

			// Enrollment & Progress
			userRoutes.POST("/courses/:id/enroll", handlers.EnrollCourse)
			userRoutes.DELETE("/courses/:id/enroll", handlers.UnenrollCourse)
			userRoutes.GET("/courses/:id/enrollment-status", handlers.GetEnrollmentStatus)
			userRoutes.POST("/courses/:id/complete", handlers.CompleteCourse)
			userRoutes.POST("/courses/:id/checkout", handlers.CourseCheckout)
			userRoutes.GET("/courses/:id/curriculum", handlers.GetSubjectCurriculum)
			userRoutes.GET("/courses/lessons/:id/progress", handlers.GetLessonProgress)
			userRoutes.POST("/courses/lessons/:id/progress", handlers.UpdateLessonProgress)
			userRoutes.POST("/courses/lessons/:id/view", handlers.TrackLessonView) // Track view stats

			// Phase 2: Advanced lesson access (drip, protection, eligibility)
			userRoutes.GET("/courses/:id/lessons/:lessonId", handlers.ProtectedLessonContent)
			userRoutes.GET("/courses/:id/lessons/:lessonId/eligibility", handlers.GetLessonEligibility)
			userRoutes.GET("/courses/:id/lessons/access", handlers.GetCourseLessonsWithAccess)

			// Lesson Notes & Reviews
			userRoutes.GET("/courses/lessons/:id/notes", handlers.GetLessonNotes)
			userRoutes.POST("/courses/lessons/:id/notes", handlers.CreateLessonNote)
			userRoutes.GET("/courses/lessons/:id/transcript", handlers.GetLessonTranscript)
			userRoutes.POST("/courses/:id/reviews", handlers.CreateCourseReview)

			// Q&A (student questions on a course, optionally scoped to a lesson;
			// instructor replies are flagged automatically, no separate route)
			userRoutes.POST("/courses/:id/questions", handlers.CreateCourseQuestion)
			userRoutes.POST("/questions/:id/answers", handlers.CreateCourseAnswer)
			userRoutes.DELETE("/questions/:id", handlers.DeleteCourseQuestion)
			userRoutes.DELETE("/answers/:id", handlers.DeleteCourseAnswer)

			// Wishlist
			userRoutes.GET("/wishlist", handlers.GetWishlist)
			userRoutes.POST("/courses/:id/wishlist", handlers.AddToWishlist)
			userRoutes.DELETE("/courses/:id/wishlist", handlers.RemoveFromWishlist)

			// Cart (multi-course checkout, separate from the single-course
			// /courses/:id/checkout path which stays as-is)
			userRoutes.GET("/cart", handlers.GetCart)
			userRoutes.POST("/cart/items", handlers.AddToCart)
			userRoutes.DELETE("/cart/items/:subjectId", handlers.RemoveFromCart)
			userRoutes.POST("/cart/checkout", handlers.CheckoutCart)

			// Upload
			userRoutes.POST("/upload/presign", handlers.PresignUpload)
			userRoutes.POST(pathUpload, handlers.Upload)
			userRoutes.DELETE(pathUpload, handlers.DeleteUpload)
			userRoutes.POST(pathUploadChunked, handlers.UploadChunked)
			userRoutes.PUT(pathUploadChunked, handlers.UploadChunked)
			userRoutes.PATCH(pathUploadChunked, handlers.UploadChunked)
			userRoutes.GET("/upload/chunked/:uploadId/status", handlers.GetUploadStatus)

			// Exam routes (Enforce StudentRequired guard at the backend route boundary)
			userRoutes.POST("/exams/:id/submit", middleware.StudentRequired(), handlers.SubmitExam)

			// Gamification routes
			userRoutes.GET("/gamification/progress", handlers.GetUserProgress)
			userRoutes.GET("/gamification/achievements", handlers.GetUserAchievements)
			userRoutes.POST("/gamification/goals", handlers.CreateCustomGoal)
			userRoutes.PATCH("/gamification/goals/:id", handlers.UpdateCustomGoal)

			// Event Ingestion (lightweight, fire-and-forget to Redis Stream)
			userRoutes.POST("/events/ingest", handlers.IngestEvent)

			// Payment routes
			userRoutes.POST("/payments/create", handlers.CreatePayment)
			userRoutes.GET("/payments/history", handlers.GetPaymentHistory)
		}

		// Teaching Dashboard routes (accessible to teachers, admins, and super_admins)
		teachingRoutes := protected.Group("/teaching")
		teachingRoutes.Use(middleware.TeacherRequired())
		teachingRoutes.Use(middleware.StrictRBAC())
		{
			// Dashboard
			teachingRoutes.GET("/dashboard/stats", handlers.GetTeachingDashboardStats)

			// Course management
			teachingRoutes.GET("/courses", handlers.TeachingListCourses)
			teachingRoutes.POST("/courses", handlers.TeachingCreateCourse)
			teachingRoutes.GET("/courses/:id", handlers.TeachingGetCourse)
			teachingRoutes.PATCH("/courses/:id", handlers.TeachingUpdateCourse)
			teachingRoutes.DELETE("/courses/:id", handlers.TeachingDeleteCourse)

			// Students
			teachingRoutes.GET("/courses/:id/students", handlers.TeachingListStudents)
			teachingRoutes.GET("/students", handlers.TeachingGetAllStudents)

			// Reviews
			teachingRoutes.GET("/courses/:id/reviews", handlers.TeachingListReviews)
			teachingRoutes.GET("/reviews", handlers.TeachingGetAllReviews)
			teachingRoutes.POST("/reviews/:id/reply", handlers.TeachingReplyToReview)

			// Activities & Notifications
			teachingRoutes.GET("/activities", handlers.TeachingGetActivities)
			teachingRoutes.GET("/notifications", handlers.TeachingGetNotifications)
			teachingRoutes.POST("/notifications/:id/read", handlers.TeachingMarkNotificationRead)
			teachingRoutes.POST("/notifications/read-all", handlers.TeachingMarkAllNotificationsRead)

			// Instructor application (for non-teachers who want to become teachers)
			teachingRoutes.POST("/apply", handlers.TeachingApplyForInstructor)
		}
	}
}
