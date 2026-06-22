package router

import (
	"thanawy-backend/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupWebhookRoutes registers Clerk webhook endpoints.
// These routes are intentionally PUBLIC — no Auth() middleware.
// Security is enforced via Svix HMAC-SHA256 signature verification inside the handler.
func SetupWebhookRoutes(router *gin.Engine) {
	webhooks := router.Group("/api/webhooks")
	{
		// Clerk user lifecycle & session events
		// Configure this URL in the Clerk Dashboard → Webhooks section.
		// Required env: CLERK_WEBHOOK_SECRET=whsec_...
		webhooks.POST("/clerk", handlers.HandleClerkWebhook)
	}
}
