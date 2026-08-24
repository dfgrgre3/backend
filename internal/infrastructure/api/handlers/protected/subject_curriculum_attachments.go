package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func AddLessonAttachment(c *gin.Context) {
	lessonId := c.Param("id")
	var attachment models.LessonAttachment
	if err := c.ShouldBindJSON(&attachment); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	attachment.SubTopicID = lessonId
	if err := SafeCreate(db.DB, &attachment); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add attachment")
		return
	}

	// Invalidate parent subject cache
	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, idQuery, lessonId).Error; err == nil {
		var topic models.Topic
		if err := db.DB.First(&topic, idQuery, subTopic.TopicID).Error; err == nil && topic.SubjectID != "" {
			getSubjectRepo().InvalidateSubjectCache(topic.SubjectID)
			cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), topic.SubjectID)
		}
	}

	api_response.Created(c, attachment)
}

func DeleteLessonAttachment(c *gin.Context) {
	attachmentId := c.Param("attachmentId")

	var attachment models.LessonAttachment
	if err := db.DB.First(&attachment, idQuery, attachmentId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Attachment not found")
		return
	}

	if err := db.DB.Delete(&attachment).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete attachment")
		return
	}

	// Invalidate parent subject cache
	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, idQuery, attachment.SubTopicID).Error; err == nil {
		var topic models.Topic
		if err := db.DB.First(&topic, idQuery, subTopic.TopicID).Error; err == nil && topic.SubjectID != "" {
			getSubjectRepo().InvalidateSubjectCache(topic.SubjectID)
			cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), topic.SubjectID)
		}
	}

	api_response.Success(c, gin.H{"success": true})
}
