package api

import (
	"log"
	"net/http"
	"os"
	"thanawy-backend/internal/application/services"
	authservice "thanawy-backend/internal/domain/auth/service"
	"thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/persistence/repositories"
	"time"

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
	router.GET("/api/v1/courses", protected.GetSubjects)
	router.GET("/api/v1/courses/popular", protected.GetPopularCourses)
	router.GET("/api/v1/courses/:id", middleware.OptionalAuth(), protected.GetSubject)
	router.GET("/api/v1/homepage", protected.GetHomepageData)
	router.GET("/api/v1/courses/:id/lessons", protected.GetCourseLessons)
	router.GET("/api/v1/lessons/:lessonId/subtitles", protected.GetLessonSubtitles) // Public subtitles
	router.GET("/api/v1/lessons/:lessonId/chapters", protected.GetVideoChapters)    // Public chapters
	router.GET("/api/v1/courses/:id/reviews", protected.GetCourseReviews)
	router.GET("/api/v1/courses/:id/questions", protected.GetCourseQuestions)
	router.GET("/api/v1/categories", protected.GetCategories)
	router.GET("/api/v1/courses/categories", protected.GetCategories)
	router.GET("/api/v1/teachers", protected.GetTeachers)

	// Public settings route
	router.GET("/api/v1/settings", protected.GetSystemSettings)

	// Public blog route (published posts only)
	router.GET("/api/v1/blog", protected.GetPublicBlogPosts)
	router.GET("/api/v1/blog/:slug", protected.GetPublicBlogPost)
	router.GET("/api/v1/blog/categories", protected.GetBlogCategories)

	// Public events route
	router.GET("/api/v1/events", protected.GetPublicEvents)

	// Public Resources route
	router.GET("/api/v1/resources", protected.GetResources)

	// Public Exam routes (read-only)
	router.GET("/api/v1/exams", protected.GetExams)
	// GetExamResults returns the caller's own exam history/answers and must
	// be authenticated — see the SECURITY note in exam_handler.go.
	router.GET("/api/v1/exams/results", middleware.Auth(), protected.GetExamResults)

	// Activity routes moved to protected group

	// AI routes (require auth & rate limiting)
	ai := router.Group("/api/v1/ai")
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

		ai.POST("/recommendations/track", protected.TrackAIRecommendation)
		ai.POST("/search/track", protected.TrackSearchHistory)
		ai.GET("/search/history", protected.GetUserSearchHistory)
	}

	// Analytics routes (public - for tracking). Unauthenticated and write to
	// the DB on every call, so they need a rate limit like every other
	// public-facing endpoint to avoid unbounded AnalyticsEvent growth / abuse.
	router.POST("/api/v1/analytics/promo", middleware.GlobalRateLimiter(120, time.Minute), protected.TrackPromoEvent)
	router.POST("/api/v1/analytics/mega-menu", middleware.GlobalRateLimiter(120, time.Minute), protected.TrackMegaMenuEvent)

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
	router.GET("/api/v1/auth/csrf", func(c *gin.Context) {
		middleware.EnsureCSRFToken(c)
		c.Status(http.StatusOK)
	})

	router.POST("/api/v1/auth/register", middleware.GuestOnly(), middleware.AuthRateLimiter(), authHandler.Register)
	router.POST("/api/v1/auth/login", middleware.GuestOnly(), middleware.LoginRateLimiter(), authHandler.Login)
	router.POST("/api/v1/auth/refresh", middleware.RefreshTokenRateLimiter(), authHandler.RefreshToken)
	router.POST("/api/v1/auth/logout", middleware.OptionalAuth(), authHandler.Logout)
	router.GET("/api/v1/auth/me", middleware.Auth(), authHandler.Me)
	router.POST("/api/v1/auth/change-password", middleware.Auth(), authHandler.ChangePassword)
	router.POST("/api/v1/auth/forgot-password", middleware.AuthRateLimiter(), authHandler.ForgotPassword)
	router.POST("/api/v1/auth/forgot-password/verify-code", middleware.AuthRateLimiter(), authHandler.VerifyForgotPasswordCode)
	router.POST("/api/v1/auth/reset-password", middleware.AuthRateLimiter(), authHandler.ResetPassword)
	router.POST("/api/v1/auth/verify-email", middleware.Auth(), middleware.AuthRateLimiter(), authHandler.VerifyEmail)
	router.POST("/api/v1/auth/resend-verification", middleware.Auth(), middleware.AuthRateLimiter(), authHandler.ResendVerification)

	// Advanced & Security-Hardened Authentication Routes
	router.POST("/api/v1/auth/refresh-session", middleware.RefreshTokenRateLimiter(), authHandler.RefreshSession)
	router.DELETE("/api/v1/auth/account", middleware.Auth(), authHandler.DeleteAccount)
	router.POST("/api/v1/auth/validate-token", middleware.AuthRateLimiter(), authHandler.ValidateToken)
	router.POST("/api/v1/auth/recovery/initiate", middleware.AuthRateLimiter(), authHandler.AccountRecovery)
	router.POST("/api/v1/auth/recovery/finalize", middleware.AuthRateLimiter(), authHandler.RecoverAccount)

	mfaService := authservice.NewMFAService()
	mfaHandler := protected.NewMFAHandler(mfaService, authservice.NewAuthTokenService(), authService)
	router.POST("/api/v1/auth/mfa/setup", middleware.Auth(), mfaHandler.SetupMFA)
	router.POST("/api/v1/auth/mfa/enable", middleware.Auth(), mfaHandler.EnableMFA)
	router.POST("/api/v1/auth/mfa/disable", middleware.Auth(), mfaHandler.DisableMFA)
	router.POST("/api/v1/auth/mfa/verify", middleware.LoginRateLimiter(), mfaHandler.VerifyMFA)

	// Social Authentication (OAuth redirect & callback)
	router.GET("/api/v1/auth/social/:provider", authHandler.SocialLogin)
	router.GET("/api/v1/auth/callback/:provider", authHandler.OAuthCallback)

	// OAuth Provider Management
	router.POST("/api/v1/auth/social/link", middleware.Auth(), authHandler.LinkProvider)
	router.POST("/api/v1/auth/social/unlink", middleware.Auth(), authHandler.UnlinkProvider)
	router.GET("/api/v1/auth/social/accounts", middleware.Auth(), authHandler.GetLinkedAccounts)

	// Guest User
	router.GET("/api/v1/users/guest", protected.GetGuestUser)

	// Paymob Webhook (POST only — GET is a CSRF vector)
	router.POST("/api/v1/payments/paymob/callback", protected.PaymobWebhook)

	// WebSocket (require auth & rate limiting)
	router.GET("/api/v1/ws", middleware.Auth(), middleware.WebSocketRateLimiter(), protected.WSHandler)

	// Public Forum routes
	router.GET("/api/v1/forum/categories", protected.GetForumCategories)
	router.GET("/api/v1/forum/posts", protected.GetForumPosts)
	router.POST("/api/v1/forum/posts", middleware.Auth(), protected.CreateForumPost)
	router.GET("/api/v1/forum/posts/:id", protected.GetForumPost)
	router.POST("/api/v1/forum/posts/:id/view", protected.IncrementForumPostView)
	router.GET("/api/v1/forum/posts/:id/replies", protected.GetForumPostReplies)
	router.POST("/api/v1/forum/posts/:id/replies", middleware.Auth(), protected.CreateForumPostReply)

	// Public community routes
	router.GET("/api/v1/announcements", protected.GetPublicAnnouncements)
	router.POST("/api/v1/announcements", middleware.Auth(), protected.CreatePublicAnnouncement)

	// Community chat routes — session-scoped: the caller's identity comes from
	// the JWT (middleware.Auth), never from a client-supplied userId path
	// segment (IDOR/BOLA). Only the counterpart user id remains a parameter.
	router.GET("/api/v1/chat/conversations", middleware.Auth(), protected.GetChatConversations)
	router.GET("/api/v1/chat/messages/:chatUserId", middleware.Auth(), protected.GetChatMessages)
	router.POST("/api/v1/chat/messages", middleware.Auth(), protected.SendChatMessage)

	// Community user directory (id, name, avatar only) for starting chats.
	// Session-scoped: the caller's own entry is excluded server-side.
	router.GET("/api/v1/community/users", middleware.Auth(), protected.GetCommunityUsers)
	router.GET("/api/v1/community/users/:id", middleware.Auth(), protected.GetCommunityUserProfile)

	// Metrics endpoints (admin auth required for detailed metrics)
	// Auth() must run first so that AdminRequired() has a user_id to inspect.
	router.GET("/api/v1/metrics", middleware.Auth(), middleware.AdminRequired(), protected.GetMetricsEndpoint)

	// Public Library routes
	router.GET("/api/v1/library/categories", protected.GetLibraryCategories)

	// Public Gamification routes
	router.GET("/api/v1/gamification/leaderboard", protected.GetLeaderboard)

	// Navigation / Mega Menu routes
	router.GET("/api/v1/navigation/menu", protected.GetNavigationMenu)
	router.GET("/api/v1/navigation/main", protected.GetMainNavItems)

	// AI Recommendations (optional auth — works without login, returns empty recommendations)
	router.GET("/api/v1/ai/recommendations", middleware.OptionalAuth(), middleware.AIRateLimiter(), protected.GetAIRecommendations)

	// Public Unified Search (courses, resources, teachers, videos).
	// Unauthenticated and fans out to 4 ILIKE-pattern DB queries per call, so
	// it needs its own rate limit rather than running unbounded.
	router.GET("/api/v1/search", middleware.GlobalRateLimiter(60, time.Minute), protected.PublicSearch)

	// Public affiliate redirect — resolves /r/:code/:slug → destination URL,
	// records a click, and returns cookie metadata so the Next.js edge page
	// can set the tracking cookie and 302 the user.
	router.GET("/api/v1/affiliates/redirect/:code/:slug", middleware.GlobalRateLimiter(600, time.Minute), protected.PublicAffiliateRedirect)
}
