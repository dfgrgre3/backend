package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// clerkWebhookEvent is the top-level structure of every Clerk webhook payload.
type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// clerkWebhookUserData contains fields from user.* events.
type clerkWebhookUserData struct {
	ID             string `json:"id"`
	EmailAddresses []struct {
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
	FirstName      *string        `json:"first_name"`
	LastName       *string        `json:"last_name"`
	Username       *string        `json:"username"`
	ImageURL       *string        `json:"image_url"`
	PublicMetadata map[string]any `json:"public_metadata"`
	UnsafeMetadata map[string]any `json:"unsafe_metadata"`
}

// clerkWebhookSessionData contains fields from session.* events.
type clerkWebhookSessionData struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

// HandleClerkWebhook processes inbound webhook events from Clerk.
// It verifies the Svix signature before processing any payload.
//
// Supported events:
//   - user.created / user.updated → upsert user in DB via SyncUserFromClerkResponse
//   - user.deleted                → soft-delete user + invalidate caches
//   - session.ended               → invalidate role/perms cache for the user
//
// Route: POST /api/webhooks/clerk
// This endpoint must be PUBLIC (no Auth() middleware) — Clerk calls it directly.
func HandleClerkWebhook(c *gin.Context) {
	cfg := config.Load()
	if cfg.ClerkWebhookSecret == "" {
		log.Printf("[Clerk Webhook] CLERK_WEBHOOK_SECRET is not configured — rejecting request")
		api_response.Error(c, http.StatusServiceUnavailable, "Webhook not configured")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// Verify Svix signature before processing any payload
	svixID := c.GetHeader("svix-id")
	svixTimestamp := c.GetHeader("svix-timestamp")
	svixSignature := c.GetHeader("svix-signature")

	if !verifyClerkWebhookSignature(body, svixID, svixTimestamp, svixSignature, cfg.ClerkWebhookSecret) {
		log.Printf("[Clerk Webhook] Signature verification failed (svix-id: %s)", svixID)
		api_response.Error(c, http.StatusUnauthorized, "Invalid webhook signature")
		return
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to parse webhook payload")
		return
	}

	log.Printf("[Clerk Webhook] Received event: %s (svix-id: %s)", event.Type, svixID)

	switch event.Type {
	case "user.created", "user.updated":
		handleClerkUserUpsert(c, event.Data, event.Type)
	case "user.deleted":
		handleClerkUserDeleted(c, event.Data)
	case "session.ended", "session.revoked", "session.removed":
		handleClerkSessionEnded(c, event.Data)
	default:
		// Unknown event — acknowledge receipt but take no action
		log.Printf("[Clerk Webhook] Unhandled event type: %s", event.Type)
		c.JSON(http.StatusOK, gin.H{"received": true, "action": "ignored"})
	}
}

// handleClerkUserUpsert creates or updates a user from a Clerk user.* webhook event.
func handleClerkUserUpsert(c *gin.Context, data json.RawMessage, eventType string) {
	var userData clerkWebhookUserData
	if err := json.Unmarshal(data, &userData); err != nil {
		log.Printf("[Clerk Webhook] Failed to parse user data: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Failed to parse user data")
		return
	}

	clerkResp := &services.ClerkUserResponse{
		ID:             userData.ID,
		FirstName:      userData.FirstName,
		LastName:       userData.LastName,
		Username:       userData.Username,
		ImageURL:       userData.ImageURL,
		PublicMetadata: userData.PublicMetadata,
		UnsafeMetadata: userData.UnsafeMetadata,
	}
	for _, ea := range userData.EmailAddresses {
		clerkResp.EmailAddresses = append(clerkResp.EmailAddresses, services.ClerkEmailAddress{
			EmailAddress: ea.EmailAddress,
		})
	}

	user, err := services.SyncUserFromClerkResponse(clerkResp)
	if err != nil {
		log.Printf("[Clerk Webhook] Failed to sync user %s: %v", userData.ID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to sync user")
		return
	}

	// Invalidate caches so updated data propagates immediately
	middleware.InvalidateRolePermsCache(user.ID)

	log.Printf("[Clerk Webhook] %s: synced user %s (%s)", eventType, user.ID, user.Email)
	c.JSON(http.StatusOK, gin.H{"synced": true, "userId": user.ID})
}

// handleClerkUserDeleted soft-deletes a user when Clerk fires user.deleted.
func handleClerkUserDeleted(c *gin.Context, data json.RawMessage) {
	var payload struct {
		ID      string `json:"id"`
		Deleted bool   `json:"deleted"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("[Clerk Webhook] Failed to parse deleted user data: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Failed to parse deleted user data")
		return
	}

	if !payload.Deleted || payload.ID == "" {
		c.JSON(http.StatusOK, gin.H{"action": "noop"})
		return
	}

	if db.DB == nil {
		log.Printf("[Clerk Webhook] DB unavailable — cannot process user.deleted for Clerk ID %s", payload.ID)
		api_response.Error(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Resolve Clerk ID → DB UUID (try exact clerk_id match first)
	dbUserID := services.ClerkIDToUUID(payload.ID)
	var user models.User
	if err := db.DB.Select("id").Where("clerk_id = ? OR id = ?", payload.ID, dbUserID).First(&user).Error; err == nil {
		dbUserID = user.ID
	}

	// Soft-delete: mark status as INACTIVE rather than hard-deleting to preserve audit trails
	if err := db.DB.Model(&models.User{}).Where("id = ?", dbUserID).
		Updates(map[string]any{"status": models.StatusInactive}).Error; err != nil {
		log.Printf("[Clerk Webhook] Failed to deactivate user %s: %v", dbUserID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to deactivate user")
		return
	}

	// Invalidate all caches for this user immediately
	middleware.InvalidateRolePermsCache(dbUserID)

	log.Printf("[Clerk Webhook] user.deleted: deactivated user %s (Clerk ID: %s)", dbUserID, payload.ID)
	c.JSON(http.StatusOK, gin.H{"deactivated": true, "userId": dbUserID})
}

// handleClerkSessionEnded invalidates the role/perms cache when a Clerk session ends.
func handleClerkSessionEnded(c *gin.Context, data json.RawMessage) {
	var session clerkWebhookSessionData
	if err := json.Unmarshal(data, &session); err != nil {
		log.Printf("[Clerk Webhook] Failed to parse session data: %v", err)
		api_response.Error(c, http.StatusBadRequest, "Failed to parse session data")
		return
	}

	if session.UserID == "" {
		c.JSON(http.StatusOK, gin.H{"action": "noop"})
		return
	}

	// Resolve Clerk user ID → DB UUID and invalidate cache
	dbUserID := services.ClerkIDToUUID(session.UserID)
	if db.DB != nil {
		var user models.User
		if err := db.DB.Select("id").Where("clerk_id = ? OR id = ?", session.UserID, dbUserID).First(&user).Error; err == nil {
			dbUserID = user.ID
		}
	}

	middleware.InvalidateRolePermsCache(dbUserID)

	log.Printf("[Clerk Webhook] session.ended: invalidated cache for user %s (session: %s)", dbUserID, session.ID)
	c.JSON(http.StatusOK, gin.H{"invalidated": true, "userId": dbUserID})
}

// verifyClerkWebhookSignature validates the Svix HMAC-SHA256 signature on a Clerk webhook.
// See: https://docs.svix.com/receiving/verifying-payloads/how
func verifyClerkWebhookSignature(body []byte, svixID, svixTimestamp, svixSignature, secret string) bool {
	if svixID == "" || svixTimestamp == "" || svixSignature == "" {
		return false
	}

	// Reject replayed requests with timestamps older than 5 minutes
	ts, err := strconv.ParseInt(svixTimestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 300 {
		log.Printf("[Clerk Webhook] Timestamp too old: %d (now: %d)", ts, time.Now().Unix())
		return false
	}

	// Signed payload format: "<svix-id>.<svix-timestamp>.<body>"
	signedPayload := fmt.Sprintf("%s.%s.%s", svixID, svixTimestamp, string(body))

	// Clerk webhook secrets are prefixed with "whsec_" and base64-standard encoded
	rawSecret := strings.TrimPrefix(secret, "whsec_")
	secretBytes, decErr := base64.StdEncoding.DecodeString(rawSecret)
	if decErr != nil {
		// Try without padding as some secrets omit it
		secretBytes, decErr = base64.RawStdEncoding.DecodeString(rawSecret)
		if decErr != nil {
			log.Printf("[Clerk Webhook] Failed to decode webhook secret: %v", decErr)
			return false
		}
	}

	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedPayload))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// svix-signature may contain multiple signatures: "v1,<sig1> v1,<sig2>"
	for _, part := range strings.Fields(svixSignature) {
		if after, ok := strings.CutPrefix(part, "v1,"); ok {
			if hmac.Equal([]byte(after), []byte(expectedSig)) {
				return true
			}
		}
	}

	return false
}
