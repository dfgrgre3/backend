package handlers

import (
	"net/http"
	"strconv"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetLessons returns lessons for a given user
func GetLessons(c *gin.Context) {
	authUserID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	authUserIDStr, _ := authUserID.(string)

	userId := c.Query("userId")
	if userId == "" {
		userId = authUserIDStr
	}

	// Verify ownership or administrative privilege (BOLA/IDOR prevention)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	isAdmin := roleStr == "ADMIN" || roleStr == "SUPER_ADMIN" || roleStr == "MODERATOR"

	if userId != authUserIDStr && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to view these lessons"})
		return
	}

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var lessons []models.Lesson
	if err := readDB.
		Preload("Teacher").
		Where("user_id = ? AND start_time >= ?", userId, time.Now()).
		Order("start_time asc").
		Limit(limit).
		Offset(offset).
		WithContext(c.Request.Context()).
		Find(&lessons).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch lessons"})
		return
	}

	c.JSON(http.StatusOK, lessons)
}

// CreateLesson creates a new scheduled lesson
func CreateLesson(c *gin.Context) {
	userId, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		TeacherID string `json:"teacherId" binding:"required"`
		Title     string `json:"title" binding:"required"`
		Location  string `json:"location" binding:"required"`
		StartTime string `json:"startTime" binding:"required"`
		EndTime   string `json:"endTime" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		// Try alternative format
		startTime, err = time.Parse("2006-01-02T15:04:05Z07:00", input.StartTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid startTime format"})
			return
		}
	}

	endTime, err := time.Parse(time.RFC3339, input.EndTime)
	if err != nil {
		endTime, err = time.Parse("2006-01-02T15:04:05Z07:00", input.EndTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endTime format"})
			return
		}
	}

	// Verify teacher exists and is a teacher
	var teacher models.User
	if err := db.DB.Where("id = ? AND role = ?", input.TeacherID, models.RoleTeacher).First(&teacher).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Teacher not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify teacher"})
		return
	}

	lesson := models.Lesson{
		UserID:    userId,
		TeacherID: input.TeacherID,
		Title:     input.Title,
		Location:  input.Location,
		StartTime: startTime,
		EndTime:   endTime,
	}

	if err := SafeCreate(db.DB, &lesson); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lesson"})
		return
	}

	// Reload with teacher info
	db.DB.Preload("Teacher").First(&lesson, lesson.ID)

	c.JSON(http.StatusCreated, lesson)
}
