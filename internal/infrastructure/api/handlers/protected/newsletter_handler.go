package protected

import (
	"net/http"
	"net/mail"
	"strings"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

type newsletterSubscribeRequest struct {
	Email string `json:"email" binding:"required"`
}

// SubscribeNewsletter adds a visitor to the public newsletter list.
func SubscribeNewsletter(c *gin.Context) {
	var request newsletterSubscribeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		api_response.Error(c, http.StatusBadRequest, "A valid email is required")
		return
	}

	email := strings.ToLower(strings.TrimSpace(request.Email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 {
		api_response.Error(c, http.StatusBadRequest, "A valid email is required")
		return
	}

	result := db.DB.Exec(`
		INSERT INTO newsletter_subscribers (email, subscribed_at)
		VALUES (?, NOW())
		ON CONFLICT (email) DO UPDATE SET subscribed_at = NOW()
	`, email)
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to subscribe")
		return
	}

	api_response.Success(c, gin.H{"subscribed": true})
}
