package api

import (
	"log"
	"net/http"
	"os"
	"thanawy-backend/internal/application/services"
	authservice "thanawy-backend/internal/domain/auth/service"
	"thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/persistence/repositories"

	"thanawy-backend/internal/infrastructure/api/middleware"
	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// SetupPublicRoutes configures public API endpoints
func SetupPublicRoutes(router *gin.Engine) {
	// Health Checks
	router.GET("/health/live", protected.LivenessCheck)
	router.GET("/health/ready", protected.ReadinessCheck)

	// Public Course routes
	router.GET("/api/courses", protected.GetSubjects)
	router.GET("/api/courses/popular", protected.GetPopularCourses)
	router.GET("/api/courses/:id", protected.GetSubject)
	router.GET("/api/homepage", protected.GetHomepageData)
	router.GET("/api/courses/:id/lessons", protected.GetCourseLessons)
	router.GET("/api/lessons/:lessonId/subtitles", protected.GetLessonSubtitles) // Public subtitles
	router.GET("/api/lessons/:lessonId/chapters", protected.GetVideoChapters)    // Public chapters
	router.GET("/api/courses/:id/reviews", protected.GetCourseReviews)
	router.GET("/api/categories", protected.GetCategories)
	router.GET("/api/courses/categories", protected.GetCategories)
	router.GET("/api/teachers", protected.GetTeachers)

	// Public settings route
	router.GET("/api/settings", protected.GetSystemSettings)

	// Public blog route (published posts only)
	router.GET("/api/blog", protected.GetPublicBlogPosts)
	router.GET("/api/blog/:slug", protected.GetPublicBlogPost)
	router.GET("/api/blog/categories", protected.GetBlogCategories)

	// Public events route
	router.GET("/api/events", protected.GetPublicEvents)

	// Public Resources route
	router.GET("/api/resources", protected.GetResources)

	// Public Exam routes (read-only)
	router.GET("/api/exams", protected.GetExams)
	router.GET("/api/exams/results", protected.GetExamResults)

	// Activity routes moved to protected group

	// AI routes (require auth & rate limiting)
	ai := router.Group("/api/ai")
	ai.Use(middleware.Auth(), middleware.AIRateLimiter())
	{
		ai.POST("/exam", protected.AIExamProxy)
		// Polling endpoint for the async exam generation job queue.
		// The frontend receives a {jobId} from POST /api/ai/exam and then
		// calls this every ~1.5s until it sees status="completed".
		// Defined BEFORE the /conversation/:id route so the literal segment
		// "exam" doesn't get matched as an id.
		ai.GET("/exam/status/:jobId", protected.GetExamStatus)
		ai.POST("/suggest", protected.AISuggestProxy)
		ai.POST("/chat", protected.AIChatProxy)
		ai.POST("/tips", protected.AITipsProxy)
		ai.GET("/conversations", protected.GetConversations)
		ai.GET("/conversation/:id", protected.GetConversation)
		ai.DELETE("/conversation/:id", protected.DeleteConversation)
		ai.POST("/explain-mistake", protected.ExplainMistakeProxy)
		ai.POST("/study-planner", protected.GenerateStudyPlanProxy)
		ai.POST("/summarize", protected.SummarizeLessonProxy)
		// Polling for async lesson summarization
		ai.GET("/summarize/status/:jobId", protected.GetSummarizeStatus)
		ai.POST("/grade-essay", protected.GradeEssayProxy)
		// Polling for async essay grading
		ai.GET("/grade-essay/status/:jobId", protected.GetEssayGradeStatus)
		ai.GET("/recommendations", protected.GetAIRecommendations)
		ai.POST("/recommendations/track", protected.TrackAIRecommendation)
		ai.POST("/search/track", protected.TrackSearchHistory)
		ai.GET("/search/history", protected.GetUserSearchHistory)
	}

	// Analytics routes (public - for tracking)
	router.POST("/api/analytics/promo", protected.TrackPromoEvent)
	router.POST("/api/analytics/mega-menu", protected.TrackMegaMenuEvent)

	// Local authentication endpoints
	oauthRedirectBase := os.Getenv("OAUTH_REDIRECT_BASE_URL")
	if oauthRedirectBase == "" {
		oauthRedirectBase = "http://localhost:8082"
	}
	oauthService, err := authservice.NewOAuthService(authservice.OAuthConfig{
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AppleClientID:      os.Getenv("APPLE_CLIENT_ID"),
		AppleClientSecret:  os.Getenv("APPLE_SECRET"),
		AppleKeyID:         os.Getenv("APPLE_KEY_ID"),
		AppleTeamID:        os.Getenv("APPLE_TEAM_ID"),
		RedirectURL:        oauthRedirectBase,
	})
	if err != nil {
		log.Printf("OAuth service not configured: %v", err)
	}
	cfg, configErr := config.LoadSafe()
	if configErr != nil {
		if gin.Mode() != gin.TestMode {
			log.Fatalf("FATAL: invalid configuration: %v", configErr)
		}
		cfg = &config.Config{}
	}
	mailQueueWorker := services.GetMailQueueWorker()
	authService := authservice.NewAuthService(repositories.NewAuthRepository(), authservice.NewAuthTokenService(), oauthService, cfg, mailQueueWorker)
	authHandler := protected.NewAuthHandler(authService)

	// CSRF bootstrap endpoint used by the admin client before state-changing requests.
	// It ensures the browser receives a _csrf cookie even before the first write request.
	router.GET("/api/auth/csrf", func(c *gin.Context) {
		middleware.EnsureCSRFToken(c)
		c.Status(http.StatusOK)
	})

	router.POST("/api/auth/register", middleware.GuestOnly(), middleware.AuthRateLimiter(), authHandler.Register)
	router.POST("/api/auth/login", middleware.GuestOnly(), middleware.LoginRateLimiter(), authHandler.Login)
	router.POST("/api/auth/refresh", middleware.RefreshTokenRateLimiter(), authHandler.RefreshToken)
	router.POST("/api/auth/logout", middleware.OptionalAuth(), authHandler.Logout)
	router.GET("/api/auth/me", middleware.Auth(), authHandler.Me)
	router.POST("/api/auth/change-password", middleware.Auth(), authHandler.ChangePassword)
	router.POST("/api/auth/forgot-password", middleware.AuthRateLimiter(), authHandler.ForgotPassword)
	router.POST("/api/auth/forgot-password/verify-code", middleware.AuthRateLimiter(), authHandler.VerifyForgotPasswordCode)
	router.POST("/api/auth/reset-password", middleware.AuthRateLimiter(), authHandler.ResetPassword)
	router.POST("/api/auth/verify-email", middleware.Auth(), authHandler.VerifyEmail)
	router.POST("/api/auth/resend-verification", middleware.Auth(), middleware.AuthRateLimiter(), authHandler.ResendVerification)

	// Advanced & Security-Hardened Authentication Routes
	router.POST("/api/auth/refresh-session", middleware.RefreshTokenRateLimiter(), authHandler.RefreshSession)
	router.PATCH("/api/auth/profile", middleware.Auth(), authHandler.UpdateProfile)
	router.DELETE("/api/auth/account", middleware.Auth(), authHandler.DeleteAccount)
	router.POST("/api/auth/validate-token", middleware.AuthRateLimiter(), authHandler.ValidateToken)
	router.POST("/api/auth/recovery/initiate", middleware.AuthRateLimiter(), authHandler.AccountRecovery)
	router.POST("/api/auth/recovery/finalize", middleware.AuthRateLimiter(), authHandler.RecoverAccount)

	sessionHandler := protected.NewSessionHandler(authService)
	router.GET("/api/auth/sessions", middleware.Auth(), sessionHandler.ListSessions)
	router.DELETE("/api/auth/sessions/:id", middleware.Auth(), sessionHandler.RevokeSession)
	router.DELETE("/api/auth/sessions", middleware.Auth(), sessionHandler.RevokeAllSessions)

	mfaService := authservice.NewMFAService()
	mfaHandler := protected.NewMFAHandler(mfaService, authservice.NewAuthTokenService(), authService)
	router.POST("/api/auth/mfa/setup", middleware.Auth(), mfaHandler.SetupMFA)
	router.POST("/api/auth/mfa/enable", middleware.Auth(), mfaHandler.EnableMFA)
	router.POST("/api/auth/mfa/disable", middleware.Auth(), mfaHandler.DisableMFA)
	router.POST("/api/auth/mfa/verify", middleware.LoginRateLimiter(), mfaHandler.VerifyMFA)

	// Social Authentication (OAuth redirect & callback)
	router.GET("/api/auth/social/:provider", authHandler.SocialLogin)
	router.GET("/api/auth/callback/:provider", authHandler.OAuthCallback)

	// OAuth Provider Management
	router.POST("/api/auth/social/link", middleware.Auth(), authHandler.LinkProvider)
	router.POST("/api/auth/social/unlink", middleware.Auth(), authHandler.UnlinkProvider)
	router.GET("/api/auth/social/accounts", middleware.Auth(), authHandler.GetLinkedAccounts)

	// Guest User
	router.GET("/api/users/guest", protected.GetGuestUser)

	// Paymob Webhook (POST only — GET is a CSRF vector)
	router.POST("/api/payments/paymob/callback", protected.PaymobWebhook)

	// WebSocket (require auth & rate limiting)
	router.GET("/api/ws", middleware.Auth(), middleware.WebSocketRateLimiter(), protected.WSHandler)

	// Public Forum routes
	router.GET("/api/forum/categories", protected.GetForumCategories)
	router.GET("/api/forum/posts", protected.GetForumPosts)
	router.POST("/api/forum/posts", middleware.Auth(), protected.CreateForumPost)
	router.GET("/api/forum/posts/:id", protected.GetForumPost)
	router.POST("/api/forum/posts/:id/view", protected.IncrementForumPostView)
	router.GET("/api/forum/posts/:id/replies", protected.GetForumPostReplies)
	router.POST("/api/forum/posts/:id/replies", middleware.Auth(), protected.CreateForumPostReply)

	// Public community routes
	router.GET("/api/announcements", protected.GetPublicAnnouncements)
	router.POST("/api/announcements", middleware.Auth(), protected.CreatePublicAnnouncement)

	// Lightweight community chat compatibility routes
	router.GET("/api/chat/conversations/:userId", middleware.Auth(), protected.GetChatConversations)
	router.GET("/api/chat/messages/:userId/:chatUserId", middleware.Auth(), protected.GetChatMessages)
	router.POST("/api/chat/messages", middleware.Auth(), protected.SendChatMessage)

	// Metrics endpoints (admin auth required for detailed metrics)
	// Auth() must run first so that AdminRequired() has a user_id to inspect.
	router.GET("/api/metrics", middleware.Auth(), middleware.AdminRequired(), protected.GetMetricsEndpoint)

	// Public Library routes
	router.GET("/api/library/categories", protected.GetLibraryCategories)

	// Public Gamification routes
	router.GET("/api/gamification/leaderboard", protected.GetLeaderboard)

	// Navigation / Mega Menu routes
	router.GET("/api/navigation/menu", protected.GetNavigationMenu)
	router.GET("/api/navigation/main", protected.GetMainNavItems)

	// Public Unified Search (courses, resources, teachers, videos)
	router.GET("/api/search", protected.PublicSearch)
}
