package protected

import (
	"encoding/json"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	courseservice "thanawy-backend/internal/domain/course/service"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ProtectedLessonContent returns full lesson content for enrolled users
// with drip scheduling and content protection logic applied
// GET /api/courses/:courseId/lessons/:lessonId
func ProtectedLessonContent(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := c.Param("courseId")
	lessonID := c.Param("lessonId")

	// Check eligibility using the service
	lessonService := courseservice.NewLessonService()
	eligibility, err := lessonService.CheckLessonEligibility(userID, lessonID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	if !eligibility.Eligible {
		api_response.Error(c, http.StatusForbidden, eligibility.Reason)
		return
	}

	// Get lesson details
	var subTopic models.SubTopic
	if err := db.DB.Preload("Topic").First(&subTopic, "id = ?", lessonID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}

	// Verify lesson belongs to the course
	if subTopic.Topic.SubjectID != courseID {
		api_response.Error(c, http.StatusBadRequest, "Lesson does not belong to this course")
		return
	}

	// Get attachments
	var attachments []models.LessonAttachment
	db.DB.Where("sub_topic_id = ?", lessonID).Find(&attachments)

	// Get subtitles (if any)
	var subtitles []models.LessonSubtitle
	db.DB.Where("sub_topic_id = ?", lessonID).
		Order("is_default DESC, language ASC").
		Find(&subtitles)

	// Get video chapters
	var chapters []courseservice.VideoChapter
	db.DB.Where("sub_topic_id = ? AND is_active = ?", lessonID, true).
		Order("sort_order ASC").
		Find(&chapters)

	// Get user's progress for this lesson
	var progress models.LessonProgress
	hasProgress := db.DB.Where("user_id = ? AND sub_topic_id = ?", userID, lessonID).First(&progress).Error == nil

	// Get view stats
	var viewStat models.LessonViewStat
	hasViewStat := db.DB.Where("user_id = ? AND sub_topic_id = ?", userID, lessonID).First(&viewStat).Error == nil

	// Build response - only expose content for eligible users
	response := gin.H{
		"lesson": gin.H{
			"id":              subTopic.ID,
			"title":           subTopic.Title,
			"description":     subTopic.Description,
			"type":            subTopic.Type,
			"order":           subTopic.Order,
			"durationMinutes": subTopic.DurationMinutes,
			"isFree":          subTopic.IsFree,
			"topicId":         subTopic.TopicID,
			"attachments":     attachments,
			"subtitles":       subtitles,
			"chapters":        chapters,
			"progress":        hasProgress,
			"lastPosition":    hasViewStat,
		},
		"eligibility": eligibility,
	}

	// Conditionally include content based on type
	switch subTopic.Type {
	case models.SubTopicVideo:
		response["lesson"].(gin.H)["videoUrl"] = subTopic.VideoUrl
		response["lesson"].(gin.H)["subtitleUrls"] = parseSubtitleUrls(subTopic.SubtitleUrls)
		if hasViewStat {
			response["lesson"].(gin.H)["lastWatchPosition"] = viewStat.LastPositionSeconds
			response["lesson"].(gin.H)["watchTimeSeconds"] = viewStat.WatchTimeSeconds
		}
	case models.SubTopicAudio:
		response["lesson"].(gin.H)["audioUrl"] = subTopic.AudioUrl
		response["lesson"].(gin.H)["audioDurationSeconds"] = subTopic.AudioDurationSeconds
	case models.SubTopicLink:
		response["lesson"].(gin.H)["externalLinkUrl"] = subTopic.ExternalLinkUrl
		response["lesson"].(gin.H)["externalLinkTitle"] = subTopic.ExternalLinkTitle
	case models.SubTopicArticle:
		response["lesson"].(gin.H)["content"] = subTopic.Content
	case models.SubTopicQuiz, models.SubTopicAssignment:
		response["lesson"].(gin.H)["examId"] = subTopic.ExamID
	}

	api_response.Success(c, response)
}

// GetLessonEligibility returns whether the current user can access a lesson
// GET /api/courses/:courseId/lessons/:lessonId/eligibility
func GetLessonEligibility(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	lessonID := c.Param("lessonId")
	lessonService := courseservice.NewLessonService()

	eligibility, err := lessonService.CheckLessonEligibility(userID, lessonID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, err.Error())
		return
	}

	api_response.Success(c, gin.H{"eligibility": eligibility})
}

// GetCourseLessonsWithAccess returns lessons with individual access flags
// GET /api/courses/:id/lessons/access
func GetCourseLessonsWithAccess(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := c.Param("id")
	lessonService := courseservice.NewLessonService()

	lessons, err := lessonService.GetAvailableLessons(userID, courseID)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch lessons", err)
		return
	}

	api_response.Success(c, gin.H{"lessons": lessons})
}

// parseSubtitleUrls parses JSONB subtitle data
func parseSubtitleUrls(data []byte) map[string]string {
	if len(data) == 0 {
		return nil
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}
