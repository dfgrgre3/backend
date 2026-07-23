package handlers

import (
	"net/http"
	"strings"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminListLiveSessions returns scheduled live classes, optionally for one subject.
func AdminListLiveSessions(c *gin.Context) {
	query := db.DB.Order("scheduled_at ASC")
	if subjectID := c.Query("subjectId"); subjectID != "" {
		parsedID, err := uuid.Parse(subjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "subjectId must be a UUID"})
			return
		}
		query = query.Where("subject_id = ?", parsedID)
	}

	var sessions []models.LiveSession
	if err := query.Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch live sessions"})
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// AdminCreateLiveSession schedules a live class for the admin live-sessions page.
func AdminCreateLiveSession(c *gin.Context) {
	var input struct {
		SubjectID   string  `json:"subjectId"`
		Title       string  `json:"title" binding:"required"`
		Description *string `json:"description"`
		Provider    string  `json:"provider"`
		JoinURL     *string `json:"joinUrl"`
		StartURL    *string `json:"startUrl"`
		HostEmail   string  `json:"hostEmail"`
		ScheduledAt string  `json:"scheduledAt" binding:"required"`
		DurationMin int     `json:"durationMin"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, input.ScheduledAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scheduledAt must be an RFC3339 timestamp"})
		return
	}
	provider := strings.ToUpper(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = "ZOOM"
	}
	if provider != "ZOOM" && provider != "MEET" && provider != "CUSTOM" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be ZOOM, MEET, or CUSTOM"})
		return
	}
	duration := input.DurationMin
	if duration == 0 {
		duration = 60
	}
	if duration < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "durationMin must be positive"})
		return
	}

	session := models.LiveSession{
		Title: input.Title, Description: input.Description, Provider: provider,
		JoinURL: input.JoinURL, StartURL: input.StartURL, HostEmail: input.HostEmail,
		ScheduledAt: scheduledAt, DurationMin: duration, Status: "SCHEDULED",
	}
	if input.SubjectID != "" {
		subjectID, err := uuid.Parse(input.SubjectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "subjectId must be a UUID"})
			return
		}
		session.SubjectID = &subjectID
	}
	if err := SafeCreate(db.DB, &session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create live session"})
		return
	}
	LogAudit(c, "CREATE", "live_session", session.ID.String(), session)
	c.JSON(http.StatusCreated, session)
}
