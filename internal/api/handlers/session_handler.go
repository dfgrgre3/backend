package handlers

import (
	"net/http"

	"thanawy-backend/internal/api/response"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	authService services.AuthService
}

func NewSessionHandler(authService services.AuthService) *SessionHandler {
	return &SessionHandler{
		authService: authService,
	}
}

func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessions, err := h.authService.GetUserSessions(c.Request.Context(), userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, sessions)
}

func (h *SessionHandler) RevokeSession(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		response.Error(c, http.StatusBadRequest, "Session ID is required")
		return
	}

	err := h.authService.RevokeSession(c.Request.Context(), userID.(string), sessionID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Session revoked successfully"})
}

func (h *SessionHandler) RevokeAllSessions(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	err := h.authService.RevokeAllSessions(c.Request.Context(), userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "All sessions revoked successfully"})
}
