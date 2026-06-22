package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"
	"time"

	"github.com/gin-gonic/gin"
)

type SessionResponseRecord struct {
	ID           string  `json:"id"`
	UserAgent    string  `json:"userAgent"`
	IP           string  `json:"ip"`
	DeviceInfo   *string `json:"deviceInfo"`
	CreatedAt    string  `json:"createdAt"`
	LastAccessed string  `json:"lastAccessed"`
	ExpiresAt    string  `json:"expiresAt"`
	IsCurrent    bool    `json:"isCurrent"`
	IsTrusted    bool    `json:"isTrusted"`
	Location     *string `json:"location"`
}

func GetAuthSessions(c *gin.Context) {
	dbUserID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	dbUserIDStr, ok := dbUserID.(string)
	if !ok || dbUserIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	var user models.User
	if err := db.DB.Select("clerk_id").Where("id = ?", dbUserIDStr).First(&user).Error; err != nil || user.ClerkID == nil || *user.ClerkID == "" {
		c.JSON(http.StatusOK, gin.H{"sessions": []SessionResponseRecord{}, "success": true})
		return
	}

	clerkSessions, err := services.FetchUserSessionsFromClerk(*user.ClerkID)
	if err != nil {
		log.Printf("[Session Handler] Failed to fetch sessions from Clerk for user %s: %v", dbUserIDStr, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch active sessions"})
		return
	}

	currentJTI := c.GetString("jti")

	responseRecords := make([]SessionResponseRecord, 0, len(clerkSessions))
	for _, cs := range clerkSessions {
		ip := "unknown"
		ua := "Unknown"
		deviceType := "desktop"
		var locationStr *string

		if cs.LatestActivity != nil {
			if cs.LatestActivity.IPAddress != "" {
				ip = cs.LatestActivity.IPAddress
			}
			if cs.LatestActivity.UserAgent != "" {
				ua = cs.LatestActivity.UserAgent
			}
			if cs.LatestActivity.DeviceType != "" {
				deviceType = cs.LatestActivity.DeviceType
			}
			locParts := []string{}
			if cs.LatestActivity.City != "" {
				locParts = append(locParts, cs.LatestActivity.City)
			}
			if cs.LatestActivity.Country != "" {
				locParts = append(locParts, cs.LatestActivity.Country)
			}
			if len(locParts) > 0 {
				str := strings.Join(locParts, ", ")
				locationStr = &str
			}
		}

		deviceInfoMap := map[string]interface{}{
			"deviceType": deviceType,
			"trusted":    true,
		}
		if cs.LatestActivity != nil {
			if cs.LatestActivity.BrowserName != "" {
				deviceInfoMap["browser"] = cs.LatestActivity.BrowserName
			}
		}
		deviceInfoBytes, _ := json.Marshal(deviceInfoMap)
		deviceInfoStr := string(deviceInfoBytes)

		createdAtStr := time.UnixMilli(cs.CreatedAt).Format(time.RFC3339)
		lastActiveStr := time.UnixMilli(cs.LastActiveAt).Format(time.RFC3339)
		expireAtStr := time.UnixMilli(cs.ExpireAt).Format(time.RFC3339)

		responseRecords = append(responseRecords, SessionResponseRecord{
			ID:           cs.ID,
			UserAgent:    ua,
			IP:           ip,
			DeviceInfo:   &deviceInfoStr,
			CreatedAt:    createdAtStr,
			LastAccessed: lastActiveStr,
			ExpiresAt:    expireAtStr,
			IsCurrent:    cs.ID == currentJTI,
			IsTrusted:    true,
			Location:     locationStr,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": responseRecords,
		"success":  true,
	})
}

func DeleteAuthSession(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		var body struct {
			SessionID string `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			sessionID = body.SessionID
		}
	}

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	if err := services.RevokeClerkSession(sessionID); err != nil {
		log.Printf("[Session Handler] Failed to revoke session %s: %v", sessionID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateAuthSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}
