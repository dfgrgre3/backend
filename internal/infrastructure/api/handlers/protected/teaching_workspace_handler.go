package protected

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// These handlers provide the stable API shape consumed by the teaching UI.
// The associated persistence/services can be added independently without
// making the dashboard emit 404s while those features are empty.

func TeachingListConversations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"conversations": []any{}})
}

func TeachingSendConversationMessage(c *gin.Context) {
	var input struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": gin.H{
		"id":   "",
		"text": input.Text,
	}})
}

func TeachingGetAnalytics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"revenueData":       []any{},
		"studentGrowthData": []any{},
		"trafficData":       []any{},
	})
}

func TeachingGetTransactions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"transactions": []any{}})
}

func TeachingGetCalendar(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"events": []any{}})
}

func TeachingCreateCalendarEvent(c *gin.Context) {
	var event map[string]any
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid calendar event"})
		return
	}
	if event["id"] == nil {
		event["id"] = ""
	}
	c.JSON(http.StatusOK, gin.H{"event": event})
}

func TeachingGetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"bio": "", "experience": "", "paypalEmail": "", "apiKey": ""})
}

func TeachingUpdateSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

func TeachingGenerateAPIKey(c *gin.Context) {
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKey": "thw_" + hex.EncodeToString(keyBytes)})
}
