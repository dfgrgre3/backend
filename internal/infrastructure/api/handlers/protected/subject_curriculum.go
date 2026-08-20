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

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Lesson represents the lesson structure returned to the frontend
type Lesson struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	VideoUrl          string `json:"videoUrl"`
	AudioUrl          string `json:"audioUrl,omitempty"`
	AudioDuration     int    `json:"audioDuration,omitempty"`
	ExternalLinkUrl   string `json:"externalLinkUrl,omitempty"`
	ExternalLinkTitle string `json:"externalLinkTitle,omitempty"`
	Type              string `json:"type"`
	IsFree            bool   `json:"isFree"`
	Order             int    `json:"order"`
	DurationMinutes   int    `json:"durationMinutes"`
	ExamID            string `json:"examId,omitempty"`
	// Advanced fields
	IsDripEnabled      bool   `json:"isDripEnabled,omitempty"`
	DripReleaseDate    string `json:"dripReleaseDate,omitempty"`
	IsContentProtected bool   `json:"isContentProtected,omitempty"`
	HasSubtitles       bool   `json:"hasSubtitles,omitempty"`
	HasChapters        bool   `json:"hasChapters,omitempty"`
	ViewCount          int    `json:"viewCount,omitempty"`
	CompletionCount    int    `json:"completionCount,omitempty"`
}

func GetCourseLessons(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	id := c.Param("id")
	var subject models.Subject

	query := database.Preload(preloadAdvanced)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "fetching course lessons")
		return
	}

	var lessons []Lesson
	for _, topic := range subject.Topics {
		for _, st := range topic.SubTopics {
			l := Lesson{
				ID:                 st.ID,
				Title:              st.Title,
				Description:        stringOrEmpty(st.Description),
				VideoUrl:           stringOrEmpty(st.VideoUrl),
				AudioUrl:           stringOrEmpty(st.AudioUrl),
				AudioDuration:      st.AudioDurationSeconds,
				ExternalLinkUrl:    stringOrEmpty(st.ExternalLinkUrl),
				ExternalLinkTitle:  stringOrEmpty(st.ExternalLinkTitle),
				Type:               string(st.Type),
				IsFree:             st.IsFree,
				Order:              st.Order,
				DurationMinutes:    st.DurationMinutes,
				ExamID:             stringOrEmpty(st.ExamID),
				IsDripEnabled:      st.IsDripEnabled,
				IsContentProtected: st.IsContentProtected,
				ViewCount:          st.ViewCount,
				CompletionCount:    st.CompletionCount,
			}
			if st.DripReleaseDate != nil {
				l.DripReleaseDate = st.DripReleaseDate.Format(time.RFC3339)
			}
			// Check for subtitles and chapters
			if len(st.SubtitleUrls) > 0 {
				l.HasSubtitles = true
			}
			if len(st.VideoChaptersData) > 0 {
				l.HasChapters = true
			}
			lessons = append(lessons, l)
		}
	}

	api_response.Success(c, gin.H{
		"lessons": lessons,
		"course": gin.H{
			"id":        subject.ID,
			"title":     subject.Name,
			"titleAr":   subject.NameAr,
			"status":    subject.Status,
			"type":      string(subject.Type),
			"thumbnail": subject.ThumbnailUrl,
		},
	})
}

func GetSubjectCurriculum(c *gin.Context) {
	id := c.Param("id")
	if id == "" || strings.EqualFold(id, "undefined") || strings.EqualFold(id, "null") {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}
	var subject models.Subject

	query := db.DB.Preload(preloadTopicsSubTopics)
	query = applyIDOrSlugQuery(query, id)

	if err := query.First(&subject).Error; err == nil {
		chaptersCount := len(subject.Topics)
		lessonsCount := 0
		freeLessonsCount := 0
		totalDuration := 0

		for _, topic := range subject.Topics {
			lessonsCount += len(topic.SubTopics)
			for _, subtopic := range topic.SubTopics {
				if subtopic.IsFree {
					freeLessonsCount++
				}
				totalDuration += subtopic.DurationMinutes
			}
		}

		api_response.Success(c, gin.H{
			"stats": gin.H{
				"chaptersCount":        chaptersCount,
				"lessonsCount":         lessonsCount,
				"freeLessonsCount":     freeLessonsCount,
				"totalDurationMinutes": totalDuration,
			},
			"topics": subject.Topics,
		})
		return
	}

	// Fallback: check LmsCourse
	var course models.LmsCourse
	if err := db.DB.Preload("Sections.Lessons").Where("id = ?", id).First(&course).Error; err != nil {
		// Course exists but has no curriculum data yet — return empty instead of 404
		api_response.Success(c, gin.H{
			"stats": gin.H{
				"chaptersCount":        0,
				"lessonsCount":         0,
				"freeLessonsCount":     0,
				"totalDurationMinutes": 0,
			},
			"topics":   []interface{}{},
			"sections": []interface{}{},
		})
		return
	}

	type curriculumSection struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Order   int    `json:"order"`
		Lessons []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Type     string `json:"type"`
			Duration int    `json:"duration"`
			IsFree   bool   `json:"isFree"`
		} `json:"lessons"`
	}

	sections := make([]curriculumSection, 0, len(course.Sections))
	totalLessons := 0
	freeLessons := 0
	totalDuration := 0

	for _, s := range course.Sections {
		ls := make([]struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Type     string `json:"type"`
			Duration int    `json:"duration"`
			IsFree   bool   `json:"isFree"`
		}, 0, len(s.Lessons))
		for _, l := range s.Lessons {
			ls = append(ls, struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Type     string `json:"type"`
				Duration int    `json:"duration"`
				IsFree   bool   `json:"isFree"`
			}{
				ID:       l.ID.String(),
				Title:    l.Title,
				Type:     string(l.Type),
				Duration: l.DurationSeconds / 60,
				IsFree:   l.IsFreePreview,
			})
			if l.IsFreePreview {
				freeLessons++
			}
			totalDuration += l.DurationSeconds / 60
		}
		sections = append(sections, curriculumSection{
			ID:      s.ID.String(),
			Title:   s.Title,
			Order:   s.OrderIndex,
			Lessons: ls,
		})
		totalLessons += len(s.Lessons)
	}

	api_response.Success(c, gin.H{
		"stats": gin.H{
			"chaptersCount":        len(course.Sections),
			"lessonsCount":         totalLessons,
			"freeLessonsCount":     freeLessons,
			"totalDurationMinutes": totalDuration,
		},
		"sections": sections,
		"topics":   []interface{}{},
	})
}

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
