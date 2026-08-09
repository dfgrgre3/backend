package protected

import (
	"net/http"

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

func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessions, err := h.authService.GetUserSessions(c.Request.Context(), userID.(string))
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	api_response.Success(c, sessions)
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

func (h *SessionHandler) RevokeAllSessions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	err := h.authService.RevokeAllSessions(c.Request.Context(), userID.(string))
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "All sessions revoked successfully"})
}
