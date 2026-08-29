package protected

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"

	authservice "thanawy-backend/internal/domain/auth/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	authService authservice.AuthService
}

func NewSessionHandler(authService authservice.AuthService) *SessionHandler {
	return &SessionHandler{
		authService: authService,
	}
}

// sessionResponse enriches UserSession with isCurrent so the client can
// mark/exclude the caller's own session without guessing from user agent.
type sessionResponse struct {
	*models.UserSession
	IsCurrent bool `json:"isCurrent"`
}

// currentSessionRefreshHash identifies which UserSession row belongs to the
// refresh-token cookie on this request, so callers can flag/exclude "this
// device" without heuristics on user agent/IP (which can collide across
// devices). Returns "" if there is no (valid) refresh token cookie.
func currentSessionRefreshHash(c *gin.Context) string {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		return ""
	}
	return models.ComputeRefreshTokenHash(refreshToken)
}

func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessions, err := h.authService.GetUserSessions(c.Request.Context(), userID.(string))
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch sessions", err)
		return
	}

	currentHash := currentSessionRefreshHash(c)

	response := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		response = append(response, sessionResponse{
			UserSession: s,
			IsCurrent:   currentHash != "" && s.RefreshTokenHash == currentHash,
		})
	}

	api_response.Success(c, response)
}

func (h *SessionHandler) RevokeSession(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		api_response.Error(c, http.StatusBadRequest, "Session ID is required")
		return
	}

	err := h.authService.RevokeSession(c.Request.Context(), userID.(string), sessionID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Session revoked successfully"})
}

// RevokeAllSessions ends every OTHER active session for the caller, keeping
// the session making this very request alive. It deliberately does NOT call
// authService.RevokeAllSessions - that method (also used by password reset
// and account deletion, where logging the user out everywhere including
// "here" is correct) has no concept of "except this one" and would log the
// caller out of the device they just used to click "end all other sessions".
func (h *SessionHandler) RevokeAllSessions(c *gin.Context) {
	userIDRaw, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID := userIDRaw.(string)

	sessions, err := h.authService.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch sessions", err)
		return
	}

	currentHash := currentSessionRefreshHash(c)

	revokedCount := 0
	for _, s := range sessions {
		if currentHash != "" && s.RefreshTokenHash == currentHash {
			continue // never revoke the session making this request
		}
		if err := h.authService.RevokeSession(c.Request.Context(), userID, s.ID); err != nil {
			api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to revoke session", err)
			return
		}
		revokedCount++
	}

	api_response.Success(c, gin.H{
		"message":      "All other sessions revoked successfully",
		"revokedCount": revokedCount,
	})
}
