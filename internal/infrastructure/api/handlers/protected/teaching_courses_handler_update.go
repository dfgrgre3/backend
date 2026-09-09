package protected

import (
	"errors"
	"net/http"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var _ = authdto.TeachingCourseMutationResponse{}

type teachingAttachmentInput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FileURL  string `json:"fileUrl"`
	FileType string `json:"fileType"`
	FileSize int64  `json:"fileSize"`
}

type teachingLessonInput struct {
	ID              string                    `json:"id"`
	Title           string                    `json:"title"`
	Description     *string                   `json:"description"`
	Content         *string                   `json:"content"`
	VideoURL        *string                   `json:"videoUrl"`
	ExamID          *string                   `json:"examId"`
	Attachments     []teachingAttachmentInput `json:"attachments"`
	DurationMinutes *int                      `json:"durationMinutes"`
	Duration        string                    `json:"duration"`
	Type            string                    `json:"type"`
	Preview         bool                      `json:"isFree"`
}

type teachingChapterInput struct {
	ID      string                `json:"id"`
	Title   string                `json:"title"`
	Lessons []teachingLessonInput `json:"lessons"`
}

// TeachingUpdateCourse updates an existing course.
// @Summary Update instructor course
// @Tags teaching
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param request body map[string]interface{} true "Course payload"
// @Success 200 {object} authdto.TeachingCourseMutationResponse
// @Router /api/v1/teaching/courses/{id} [patch]
func TeachingUpdateCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	var input struct {
		Title       *string                 `json:"title"`
		Description *string                 `json:"description"`
		Thumbnail   *string                 `json:"thumbnail"`
		Price       *float64                `json:"price"`
		Status      *string                 `json:"status"`
		Level       *string                 `json:"level"`
		Language    *string                 `json:"language"`
		CategoryID  *string                 `json:"categoryId"`
		Quiz        *courseQuizInput        `json:"quiz"`
		Quizzes     []*courseQuizInput      `json:"quizzes"`
		TrailerUrl  *string                 `json:"trailerUrl"`
		ShortDesc   *string                 `json:"shortDescription"`
		LongDesc    *string                 `json:"longDescription"`
		Chapters    *[]teachingChapterInput `json:"chapters"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}
	if input.Status != nil {
		api_response.Error(c, http.StatusConflict, "Course status changes must use the review workflow")
		return
	}

	if input.Chapters != nil && subject.Status == models.CourseStatusPublished {
		api_response.Error(c, http.StatusConflict, "Published course curriculum requires a new version before editing")
		return
	}

	updates := map[string]interface{}{}

	if input.Title != nil {
		updates["name"] = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		updates["description"] = &v
	}
	if input.Thumbnail != nil {
		v := strings.TrimSpace(*input.Thumbnail)
		updates["thumbnail_url"] = &v
	}
	if input.Price != nil {
		if *input.Price < 0 {
			api_response.Error(c, http.StatusBadRequest, "Price cannot be negative")
			return
		}
		updates["price"] = decimal.NewFromFloat(*input.Price)
	}
	if input.TrailerUrl != nil {
		v := strings.TrimSpace(*input.TrailerUrl)
		updates["trailer_url"] = &v
	}
	if input.ShortDesc != nil {
		v := strings.TrimSpace(*input.ShortDesc)
		updates["short_description"] = &v
	}
	if input.LongDesc != nil {
		v := strings.TrimSpace(*input.LongDesc)
		updates["long_description"] = &v
	}
	if input.Level != nil {
		updates["level"] = models.Level(strings.ToUpper(*input.Level))
	}
	if input.Language != nil {
		updates["language"] = *input.Language
	}
	if input.CategoryID != nil {
		updates["category_id"] = strings.TrimSpace(*input.CategoryID)
	}
	if len(updates) > 0 {
		if err := database.Model(&subject).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update course")
			return
		}
		getSubjectRepo().InvalidateSubjectCache(subject.ID)
	}

	var responseChapters []gin.H
	// Curriculum updates preserve existing topic and lesson IDs. Missing items
	// are intentionally left untouched; deletion is an explicit operation.
	if input.Chapters != nil {
		var err error
		err = database.Transaction(func(tx *gorm.DB) error {
			responseChapters, err = upsertTeachingCurriculum(tx, subject.ID, *input.Chapters)
			if err != nil {
				return err
			}
			return persistTeachingQuizzes(tx, subject.ID, userID, input.Quizzes, input.Quiz)
		})
		if err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update course curriculum")
			return
		}
		getSubjectRepo().InvalidateSubjectCache(subject.ID)
	}
	if input.Chapters == nil {
		if err := persistTeachingQuizzes(database, subject.ID, userID, input.Quizzes, input.Quiz); err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to save course quiz")
			return
		}
	}

	response := gin.H{"message": "Course updated successfully"}
	if input.Chapters != nil {
		response["course"] = gin.H{"id": subject.ID, "chapters": responseChapters}
	}
	api_response.Success(c, response)
}

func upsertTeachingCurriculum(tx *gorm.DB, subjectID string, chapters []teachingChapterInput) ([]gin.H, error) {
	responseChapters := make([]gin.H, 0, len(chapters))
	for chapterIndex, chapterInput := range chapters {
		topic, err := upsertTeachingTopic(tx, subjectID, chapterInput, chapterIndex)
		if err != nil {
			return nil, err
		}

		responseLessons := make([]gin.H, 0, len(chapterInput.Lessons))
		for lessonIndex, lessonInput := range chapterInput.Lessons {
			subTopic, err := upsertTeachingSubTopic(tx, subjectID, topic.ID, lessonInput, lessonIndex)
			if err != nil {
				return nil, err
			}
			if err := upsertTeachingAttachments(tx, subTopic.ID, lessonInput.Attachments); err != nil {
				return nil, err
			}
			responseLessons = append(responseLessons, gin.H{
				"id":              subTopic.ID,
				"title":           subTopic.Title,
				"type":            string(subTopic.Type),
				"durationMinutes": subTopic.DurationMinutes,
				"isPreview":       subTopic.IsFree,
			})
		}
		responseChapters = append(responseChapters, gin.H{
			"id":      topic.ID,
			"title":   topic.Title,
			"lessons": responseLessons,
		})
	}
	return responseChapters, nil
}

func upsertTeachingTopic(tx *gorm.DB, subjectID string, input teachingChapterInput, order int) (models.Topic, error) {
	var topic models.Topic
	chapterID := strings.TrimSpace(input.ID)
	if uuid.Validate(chapterID) == nil {
		err := tx.Where("id = ? AND subject_id = ?", chapterID, subjectID).First(&topic).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return topic, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var existing models.Topic
			if lookupErr := tx.Where("id = ?", chapterID).First(&existing).Error; lookupErr == nil {
				return topic, errors.New("chapter does not belong to this course")
			}
			topic = models.Topic{ID: chapterID, SubjectID: subjectID}
		}
	} else {
		topic = models.Topic{SubjectID: subjectID}
	}

	topic.Title = strings.TrimSpace(input.Title)
	topic.Order = order + 1
	if topic.ID == "" {
		if err := tx.Create(&topic).Error; err != nil {
			return topic, err
		}
		return topic, nil
	}
	if err := tx.Model(&topic).Updates(map[string]interface{}{"title": topic.Title, "order": topic.Order}).Error; err != nil {
		return topic, err
	}
	return topic, nil
}

func upsertTeachingSubTopic(tx *gorm.DB, subjectID, topicID string, input teachingLessonInput, order int) (models.SubTopic, error) {
	var subTopic models.SubTopic
	lessonID := strings.TrimSpace(input.ID)
	if uuid.Validate(lessonID) == nil {
		err := tx.Where("id = ?", lessonID).First(&subTopic).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return subTopic, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			subTopic = models.SubTopic{ID: lessonID}
		} else {
			var owner models.Topic
			if ownerErr := tx.Where("id = ? AND subject_id = ?", subTopic.TopicID, subjectID).First(&owner).Error; ownerErr != nil {
				return subTopic, errors.New("lesson does not belong to this course")
			}
		}
	} else {
		subTopic = models.SubTopic{}
	}

	subTopic.TopicID = topicID
	subTopic.Title = strings.TrimSpace(input.Title)
	subTopic.Type = normalizeTeachingLessonType(input.Type)
	subTopic.Description = input.Description
	subTopic.Content = input.Content
	subTopic.VideoUrl = input.VideoURL
	subTopic.ExamID = input.ExamID
	subTopic.IsFree = input.Preview
	subTopic.Order = order + 1
	subTopic.DurationMinutes = lessonDurationMinutes(input.DurationMinutes, input.Duration)
	if subTopic.ID == "" {
		if err := tx.Create(&subTopic).Error; err != nil {
			return subTopic, err
		}
		return subTopic, nil
	}
	if err := tx.Model(&subTopic).Updates(map[string]interface{}{
		"topic_id":         subTopic.TopicID,
		"title":            subTopic.Title,
		"type":             subTopic.Type,
		"description":      subTopic.Description,
		"content":          subTopic.Content,
		"video_url":        subTopic.VideoUrl,
		"exam_id":          subTopic.ExamID,
		"is_free":          subTopic.IsFree,
		"order":            subTopic.Order,
		"duration_minutes": subTopic.DurationMinutes,
	}).Error; err != nil {
		return subTopic, err
	}
	return subTopic, nil
}

func upsertTeachingAttachments(tx *gorm.DB, subTopicID string, inputs []teachingAttachmentInput) error {
	for _, input := range inputs {
		title := strings.TrimSpace(input.Title)
		fileURL := strings.TrimSpace(input.FileURL)
		if title == "" || fileURL == "" {
			continue
		}

		attachmentID := strings.TrimSpace(input.ID)
		if uuid.Validate(attachmentID) == nil {
			var attachment models.LessonAttachment
			err := tx.Where("id = ?", attachmentID).First(&attachment).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				attachment = models.LessonAttachment{ID: attachmentID, SubTopicID: subTopicID}
			} else if attachment.SubTopicID != subTopicID {
				return errors.New("attachment does not belong to this lesson")
			}
			attachment.Title = title
			attachment.FileUrl = fileURL
			attachment.FileType = strings.TrimSpace(input.FileType)
			attachment.FileSize = input.FileSize
			if attachment.ID == "" {
				attachment.ID = attachmentID
			}
			if err := tx.Save(&attachment).Error; err != nil {
				return err
			}
			continue
		}

		if err := tx.Create(&models.LessonAttachment{
			SubTopicID: subTopicID,
			Title:      title,
			FileUrl:    fileURL,
			FileType:   strings.TrimSpace(input.FileType),
			FileSize:   input.FileSize,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
