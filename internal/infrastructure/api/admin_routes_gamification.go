package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminGamificationRoutes registers the general CRUD / gamification
// resources: Achievements, Rewards, Seasons, Coupons, Challenges, Blog,
// Events, Automations, Campaigns, AB Testing, Forum, Books, Learning Paths,
// Bank Questions, Resources, Media, Upload, Landing Page and Affiliates.
func registerAdminGamificationRoutes(admin, sensitive *gin.RouterGroup) {
	// Achievements
	admin.GET("/achievements", handlers.AdminGetAchievements)
	admin.POST("/achievements", handlers.AdminCreateAchievement)
	admin.PATCH("/achievements/:id", handlers.AdminUpdateAchievement)
	admin.DELETE("/achievements/:id", handlers.AdminDeleteAchievement)

	// Rewards
	admin.GET("/rewards", handlers.AdminGetRewards)
	admin.POST("/rewards", handlers.AdminCreateReward)
	admin.PATCH("/rewards/:id", handlers.AdminUpdateReward)
	admin.DELETE("/rewards/:id", handlers.AdminDeleteReward)

	// Seasons
	admin.GET("/seasons", handlers.AdminGetSeasons)
	admin.POST("/seasons", handlers.AdminCreateSeason)
	admin.PATCH("/seasons/:id", handlers.AdminUpdateSeason)
	admin.DELETE("/seasons/:id", handlers.AdminDeleteSeason)

	// Coupons
	admin.GET("/coupons", handlers.AdminGetCoupons)
	admin.POST("/coupons", handlers.AdminCreateCoupon)
	admin.PATCH("/coupons/:id", handlers.AdminUpdateCoupon)
	admin.DELETE("/coupons/:id", handlers.AdminDeleteCoupon)

	// Orders (Cart checkout history) — the admin panel's Orders page
	// predates this route; see cart.go / admin_orders.go.
	admin.GET("/orders", handlers.AdminListOrders)
	admin.PATCH("/orders", handlers.AdminUpdateOrderStatus)

	// Challenges
	admin.GET("/challenges", handlers.AdminGetChallenges)
	admin.POST("/challenges", handlers.AdminCreateChallenge)
	admin.PATCH("/challenges/:id", handlers.AdminUpdateChallenge)
	admin.DELETE("/challenges/:id", handlers.AdminDeleteChallenge)

	// Blog
	admin.GET("/blog", handlers.AdminGetBlog)
	admin.POST("/blog", handlers.AdminCreateBlogPost)
	admin.PATCH("/blog/:id", handlers.AdminUpdateBlogPost)
	admin.DELETE("/blog/:id", handlers.AdminDeleteBlogPost)

	// Events
	admin.GET("/events", handlers.AdminGetEvents)
	admin.POST("/events", handlers.AdminCreateEvent)
	admin.PATCH("/events", handlers.AdminUpdateEvent)
	admin.DELETE("/events", handlers.AdminDeleteEvent)

	// Automations
	admin.GET("/automations", handlers.AdminGetAutomations)
	admin.POST("/automations", handlers.AdminCreateAutomation)
	admin.PATCH("/automations/:id", handlers.AdminUpdateAutomation)
	admin.DELETE("/automations/:id", handlers.AdminDeleteAutomation)

	// Campaigns
	admin.GET("/marketing/campaigns", handlers.AdminGetCampaigns)
	admin.POST("/marketing/campaigns", handlers.AdminCreateCampaign)
	admin.PATCH("/marketing/campaigns/:id", handlers.AdminUpdateCampaign)
	admin.DELETE("/marketing/campaigns/:id", handlers.AdminDeleteCampaign)

	// AB Testing
	admin.GET("/ab-testing", handlers.AdminGetABTests)
	admin.POST("/ab-testing", handlers.AdminCreateABTest)
	admin.PATCH("/ab-testing/:id", handlers.AdminUpdateABTest)
	admin.DELETE("/ab-testing/:id", handlers.AdminDeleteABTest)
	admin.GET("/ab-testing/:id/variant", handlers.AdminGetABVariant)
	admin.POST("/ab-testing/:id/track", handlers.AdminTrackABEvent)

	// Forum Categories
	admin.GET("/forum", handlers.AdminGetForum)
	admin.GET("/forum-categories", handlers.AdminGetForumCategories)
	admin.POST("/forum-categories", handlers.AdminCreateForumCategory)

	// Books
	admin.GET("/books", handlers.AdminGetBooks)

	// Learning Paths
	admin.GET("/learning-paths", admindelivery.AdminListLearningPaths)
	admin.POST("/learning-paths", admindelivery.AdminCreateLearningPath)
	admin.GET("/learning-paths/:id", admindelivery.AdminGetLearningPath)
	admin.PATCH("/learning-paths/:id", admindelivery.AdminUpdateLearningPath)
	admin.DELETE("/learning-paths/:id", admindelivery.AdminDeleteLearningPath)

	// Bank Questions
	admin.GET("/bank-questions", admindelivery.AdminListBankQuestions)
	admin.GET("/bank-questions/:id", admindelivery.AdminGetBankQuestion)
	admin.POST("/bank-questions", admindelivery.AdminCreateBankQuestion)
	admin.PATCH("/bank-questions/:id", admindelivery.AdminUpdateBankQuestion)
	admin.DELETE("/bank-questions/:id", admindelivery.AdminDeleteBankQuestion)

	// Resources
	admin.GET("/resources", handlers.AdminGetResources)
	admin.POST("/resources", handlers.AdminCreateResource)
	admin.PATCH("/resources", handlers.AdminUpdateResource)
	admin.DELETE("/resources", handlers.AdminDeleteResource)

	// Media Assets
	admin.GET("/media", admindelivery.AdminListMedia)
	admin.POST("/media", admindelivery.AdminCreateMedia)
	admin.GET("/media/tags", admindelivery.AdminGetMediaTags)

	// Upload
	admin.POST("/upload/presign", handlers.PresignUpload)
	admin.POST("/upload", handlers.Upload)
	admin.DELETE("/upload", handlers.DeleteUpload)
	admin.POST("/upload/chunked", handlers.UploadChunked)
	admin.PUT("/upload/chunked", handlers.UploadChunked)
	admin.PATCH("/upload/chunked", handlers.UploadChunked)
	admin.GET("/upload/chunked/:uploadId/status", handlers.GetUploadStatus)

	// Landing Page
	admin.GET("/landing", admindelivery.AdminListLandingSections)
	admin.POST("/landing", admindelivery.AdminUpsertLandingSection)

	// Affiliates
	admin.GET("/affiliates", handlers.AdminGetAffiliates)
	admin.POST("/affiliates", handlers.AdminCreateAffiliate)
	admin.GET("/affiliates/:id", handlers.AdminGetAffiliate)
	admin.PATCH("/affiliates/:id", handlers.AdminUpdateAffiliate)
	admin.DELETE("/affiliates/:id", handlers.AdminDeleteAffiliate)
	admin.GET("/affiliates/:id/referrals", handlers.AdminGetAffiliateReferrals)
	admin.POST("/affiliates/:id/pay", handlers.AdminPayAffiliate)
	admin.POST("/books", handlers.AdminCreateBook)
	admin.PATCH("/books/:id", handlers.AdminUpdateBook)
	admin.DELETE("/books/:id", handlers.AdminDeleteBook)
	admin.GET("/books/views", admindelivery.AdminBookReviews)
	admin.GET("/books/reviews", admindelivery.AdminBookReviews)
	admin.DELETE("/books/reviews", admindelivery.AdminBookReviews)
}
