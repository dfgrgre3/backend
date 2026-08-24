package protected

import (
	"errors"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	courseservice "thanawy-backend/internal/domain/course/service"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetLessons returns lessons for a given user
func GetLessons(c *gin.Context) {
	authUserID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
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
		api_response.Error(c, http.StatusForbidden, "You are not authorized to view these lessons")
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

	var lessons []models.ScheduledLesson
	if err := readDB.
		Preload("Teacher").
		Where("user_id = ? AND start_time >= ?", userId, time.Now()).
		Order("start_time asc").
		Limit(limit).
		Offset(offset).
		WithContext(c.Request.Context()).
		Find(&lessons).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch lessons")
		return
	}

	api_response.Success(c, lessons)
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
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		// Try alternative format
		startTime, err = time.Parse("2006-01-02T15:04:05Z07:00", input.StartTime)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid startTime format")
			return
		}
	}

	endTime, err := time.Parse(time.RFC3339, input.EndTime)
	if err != nil {
		endTime, err = time.Parse("2006-01-02T15:04:05Z07:00", input.EndTime)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid endTime format")
			return
		}
	}

	// Verify teacher exists and is a teacher
	var teacher models.User
	if err := db.DB.Where("id = ? AND role = ?", input.TeacherID, models.RoleTeacher).First(&teacher).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Teacher not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify teacher")
		return
	}

	lesson := models.ScheduledLesson{
		UserID:    userId,
		TeacherID: input.TeacherID,
		Title:     input.Title,
		Location:  input.Location,
		StartTime: startTime,
		EndTime:   endTime,
	}

	if err := SafeCreate(db.DB, &lesson); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create lesson")
		return
	}

	// Reload with teacher info
	db.DB.Preload("Teacher").First(&lesson, lesson.ID)

	api_response.Success(c, lesson)
}

// TrackLessonView updates the authenticated user's view statistics for a lesson.
func TrackLessonView(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		WatchTimeSeconds    int  `json:"watchTimeSeconds"`
		LastPositionSeconds int  `json:"lastPositionSeconds"`
		Completed           bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}
	if input.WatchTimeSeconds < 0 || input.LastPositionSeconds < 0 {
		api_response.Error(c, http.StatusBadRequest, "View times cannot be negative")
		return
	}

	lessonID := c.Param("id")
	var lesson models.SubTopic
	if err := db.DB.WithContext(c.Request.Context()).First(&lesson, "id = ?", lessonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Lesson not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify lesson")
		return
	}

	if err := courseservice.NewLessonService().UpdateLessonViewStats(
		userID,
		lessonID,
		input.WatchTimeSeconds,
		input.LastPositionSeconds,
		input.Completed,
	); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update lesson view")
		return
	}

	api_response.Success(c, gin.H{"message": "Lesson view updated successfully"})
}

// GetLessonSubtitles returns subtitle tracks for a lesson.
func GetLessonSubtitles(c *gin.Context) {
	lessonID := c.Param("lessonId")
	if !lessonContentAccessible(c, lessonID) {
		return
	}

	subtitles := make([]models.LessonSubtitle, 0)
	if err := db.DB.WithContext(c.Request.Context()).
		Where("sub_topic_id = ?", lessonID).
		Order("is_default DESC, language ASC").
		Find(&subtitles).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch lesson subtitles")
		return
	}

	api_response.Success(c, gin.H{"subtitles": subtitles})
}

// GetVideoChapters returns active video chapters for a lesson.
func GetVideoChapters(c *gin.Context) {
	lessonID := c.Param("lessonId")
	if !lessonContentAccessible(c, lessonID) {
		return
	}

	chapters := make([]courseservice.VideoChapter, 0)
	if err := db.DB.WithContext(c.Request.Context()).
		Where("sub_topic_id = ? AND is_active = ?", lessonID, true).
		Order("sort_order ASC").
		Find(&chapters).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch video chapters")
		return
	}

	api_response.Success(c, gin.H{"chapters": chapters})
}

// lessonContentAccessible reports whether the caller may view a lesson's
// supplementary content (subtitles, chapter markers). Writes the appropriate
// error response and returns false if not. A lesson is accessible if it is
// marked free, or the caller is enrolled in its parent course — the same
// rule GetAvailableLessons applies to the lesson's video itself. Without
// this check, GetLessonSubtitles/GetVideoChapters (both public,
// unauthenticated routes) would leak paid-course content — e.g. full
// subtitle transcripts — to anyone who knows or enumerates a lesson ID.
func lessonContentAccessible(c *gin.Context, lessonID string) bool {
	var lesson models.SubTopic
	if err := db.DB.WithContext(c.Request.Context()).First(&lesson, "id = ?", lessonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Lesson not found")
		} else {
			api_response.Error(c, http.StatusInternalServerError, "Failed to verify lesson")
		}
		return false
	}

	if lesson.IsFree {
		return true
	}

	var topic models.Topic
	if err := db.DB.WithContext(c.Request.Context()).Select("subject_id").First(&topic, "id = ?", lesson.TopicID).Error; err != nil {
		api_response.Error(c, http.StatusForbidden, "This lesson requires enrollment")
		return false
	}

	userIDVal, exists := c.Get("userId")
	userID, _ := userIDVal.(string)
	if !exists || userID == "" {
		api_response.Error(c, http.StatusForbidden, "This lesson requires enrollment")
		return false
	}

	isEnrolled := db.DB.WithContext(c.Request.Context()).
		Where("user_id = ? AND subject_id = ?", userID, topic.SubjectID).
		First(&models.Enrollment{}).Error == nil
	if !isEnrolled {
		api_response.Error(c, http.StatusForbidden, "This lesson requires enrollment")
		return false
	}

	return true
}
