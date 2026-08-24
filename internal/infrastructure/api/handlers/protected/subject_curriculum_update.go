package protected

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/cache"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func UpdateCourseCurriculum(c *gin.Context) {
	id := c.Param("id")
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	chaptersRaw, err := extractChaptersRaw(raw)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var chapters []incomingChapter
	if err := json.Unmarshal(chaptersRaw, &chapters); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid curriculum format: "+err.Error())
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		clearSubjectCurriculum(tx, id)
		for i, chapter := range chapters {
			if err := createTopicFromIncoming(tx, id, chapter, i); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save curriculum: "+err.Error())
		return
	}

	getSubjectRepo().InvalidateSubjectCache(id)
	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), id)

	var subject models.Subject
	if err := db.DB.Preload(preloadTopicsSubTopics).First(&subject, idQuery, id).Error; err != nil {
		api_response.Success(c, gin.H{"success": true, "message": "Curriculum updated"})
		return
	}

	api_response.Success(c, gin.H{"curriculum": subject.Topics})
}

type incomingAttachment struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	FileUrl  string  `json:"fileUrl"`
	FileType *string `json:"fileType"`
	FileSize *int64  `json:"fileSize"`
}

type incomingLesson struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	Order       int                  `json:"order"`
	Type        string               `json:"type"`
	VideoUrl    *string              `json:"videoUrl"`
	Duration    int                  `json:"duration"`
	DurationMin int                  `json:"durationMinutes"`
	IsFree      bool                 `json:"isFree"`
	Description *string              `json:"description"`
	Attachments []incomingAttachment `json:"attachments"`
}

type incomingChapter struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Title     string           `json:"title"`
	Order     int              `json:"order"`
	SubTopics []incomingLesson `json:"subTopics"`
}

func extractChaptersRaw(raw map[string]json.RawMessage) (json.RawMessage, error) {
	if v, ok := raw["curriculum"]; ok {
		return v, nil
	}
	if v, ok := raw["topics"]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("missing curriculum or topics field")
}

func clearSubjectCurriculum(tx *gorm.DB, subjectId string) {
	var existingTopics []models.Topic
	if err := tx.Where(subjectIDQuery, subjectId).Find(&existingTopics).Error; err != nil {
		return
	}

	var topicIDs []string
	for _, t := range existingTopics {
		topicIDs = append(topicIDs, t.ID)
	}

	if len(topicIDs) > 0 {
		var existingSubTopics []models.SubTopic
		if err := tx.Where("topic_id IN ?", topicIDs).Find(&existingSubTopics).Error; err == nil {
			var subTopicIDs []string
			for _, st := range existingSubTopics {
				subTopicIDs = append(subTopicIDs, st.ID)
			}
			if len(subTopicIDs) > 0 {
				tx.Unscoped().Where("sub_topic_id IN ?", subTopicIDs).Delete(&models.LessonAttachment{})
			}
		}
		tx.Unscoped().Where("topic_id IN ?", topicIDs).Delete(&models.SubTopic{})
	}
	tx.Unscoped().Where(subjectIDQuery, subjectId).Delete(&models.Topic{})
}

func createTopicFromIncoming(tx *gorm.DB, subjectId string, chapter incomingChapter, order int) error {
	title := chapter.Name
	if title == "" {
		title = chapter.Title
	}
	topic := models.Topic{
		SubjectID: subjectId,
		Title:     title,
		Order:     order,
	}
	if chapter.ID != "" && !strings.HasPrefix(chapter.ID, "new-") {
		topic.ID = chapter.ID
	}

	if err := tx.Create(&topic).Error; err != nil {
		return err
	}

	for j, lesson := range chapter.SubTopics {
		if err := createSubTopicFromIncoming(tx, topic.ID, lesson, j); err != nil {
			return err
		}
	}
	return nil
}

func createSubTopicFromIncoming(tx *gorm.DB, topicId string, lesson incomingLesson, order int) error {
	title := lesson.Name
	if title == "" {
		title = lesson.Title
	}
	duration := lesson.Duration
	if duration == 0 {
		duration = lesson.DurationMin
	}
	lessonType := models.SubTopicVideo
	if lesson.Type != "" {
		lessonType = models.SubTopicType(lesson.Type)
	}

	st := models.SubTopic{
		TopicID:         topicId,
		Title:           title,
		Order:           order,
		Type:            lessonType,
		VideoUrl:        lesson.VideoUrl,
		DurationMinutes: duration,
		IsFree:          lesson.IsFree,
		Description:     lesson.Description,
	}
	if lesson.ID != "" && !strings.HasPrefix(lesson.ID, "new-") {
		st.ID = lesson.ID
	}

	if err := tx.Create(&st).Error; err != nil {
		return err
	}

	for _, att := range lesson.Attachments {
		newAtt := models.LessonAttachment{
			SubTopicID: st.ID,
			Title:      att.Title,
			FileUrl:    att.FileUrl,
		}
		if att.FileType != nil {
			newAtt.FileType = *att.FileType
		}
		if att.FileSize != nil {
			newAtt.FileSize = *att.FileSize
		}
		if att.ID != "" && !strings.HasPrefix(att.ID, "new-") {
			newAtt.ID = att.ID
		}
		if err := tx.Create(&newAtt).Error; err != nil {
			return err
		}
	}

	return nil
}
