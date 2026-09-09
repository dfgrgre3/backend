package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TeachingUpdateCourse updates an existing course.
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
		Title       *string            `json:"title"`
		Description *string            `json:"description"`
		Thumbnail   *string            `json:"thumbnail"`
		Price       *float64           `json:"price"`
		Status      *string            `json:"status"`
		Level       *string            `json:"level"`
		Language    *string            `json:"language"`
		CategoryID  *string            `json:"categoryId"`
		Quiz        *courseQuizInput   `json:"quiz"`
		Quizzes     []*courseQuizInput `json:"quizzes"`
		TrailerUrl  *string            `json:"trailerUrl"`
		ShortDesc   *string            `json:"shortDescription"`
		LongDesc    *string            `json:"longDescription"`
		Chapters    *[]struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Lessons []struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Description *string `json:"description"`
				Content     *string `json:"content"`
				VideoURL    *string `json:"videoUrl"`
				ExamID      *string `json:"examId"`
				Attachments []struct {
					Title    string `json:"title"`
					FileURL  string `json:"fileUrl"`
					FileType string `json:"fileType"`
					FileSize int64  `json:"fileSize"`
				} `json:"attachments"`
				DurationMinutes *int   `json:"durationMinutes"`
				Duration        string `json:"duration"` // legacy clients
				Type            string `json:"type"`
				Preview         bool   `json:"isFree"`
			} `json:"lessons"`
		} `json:"chapters"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
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
	if input.Status != nil {
		status := strings.ToUpper(*input.Status)
		switch models.CourseStatus(status) {
		case models.CourseStatusPublished:
			updates["status"] = models.CourseStatusPublished
			updates["is_published"] = true
			updates["published_at"] = time.Now()
		case models.CourseStatusDraft:
			updates["status"] = models.CourseStatusDraft
			updates["is_published"] = false
		case models.CourseStatusArchived:
			updates["status"] = models.CourseStatusArchived
			updates["is_published"] = false
			updates["archived_at"] = time.Now()
		default:
			api_response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
	}

	if len(updates) > 0 {
		if err := database.Model(&subject).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update course")
			return
		}
		getSubjectRepo().InvalidateSubjectCache(subject.ID)
	}

	var responseChapters []gin.H
	// Curriculum is replaced atomically when supplied. Omitting `chapters`
	// leaves the existing curriculum untouched; sending [] intentionally clears it.
	if input.Chapters != nil {
		responseChapters = make([]gin.H, 0, len(*input.Chapters))
		err := database.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("subject_id = ?", subject.ID).Delete(&models.Topic{}).Error; err != nil {
				return err
			}
			for chapterIndex, chapter := range *input.Chapters {
				topic := models.Topic{SubjectID: subject.ID, Title: strings.TrimSpace(chapter.Title), Order: chapterIndex + 1}
				if err := tx.Create(&topic).Error; err != nil {
					return err
				}
				responseLessons := make([]gin.H, 0, len(chapter.Lessons))
				for lessonIndex, lesson := range chapter.Lessons {
					subTopic := models.SubTopic{
						TopicID:         topic.ID,
						Title:           strings.TrimSpace(lesson.Title),
						Type:            normalizeTeachingLessonType(lesson.Type),
						Description:     lesson.Description,
						Content:         lesson.Content,
						VideoUrl:        lesson.VideoURL,
						ExamID:          lesson.ExamID,
						IsFree:          lesson.Preview,
						Order:           lessonIndex + 1,
						DurationMinutes: lessonDurationMinutes(lesson.DurationMinutes, lesson.Duration),
					}
					if uuid.Validate(strings.TrimSpace(lesson.ID)) == nil {
						subTopic.ID = strings.TrimSpace(lesson.ID)
					}
					if err := tx.Create(&subTopic).Error; err != nil {
						return err
					}
					for _, attachment := range lesson.Attachments {
						if strings.TrimSpace(attachment.Title) == "" || strings.TrimSpace(attachment.FileURL) == "" {
							continue
						}
						if err := tx.Create(&models.LessonAttachment{
							SubTopicID: subTopic.ID,
							Title:      strings.TrimSpace(attachment.Title),
							FileUrl:    strings.TrimSpace(attachment.FileURL),
							FileType:   strings.TrimSpace(attachment.FileType),
							FileSize:   attachment.FileSize,
						}).Error; err != nil {
							return err
						}
					}
					responseLessons = append(responseLessons, gin.H{"id": subTopic.ID, "title": subTopic.Title, "type": string(subTopic.Type), "durationMinutes": subTopic.DurationMinutes, "isPreview": subTopic.IsFree})
				}
				responseChapters = append(responseChapters, gin.H{"id": topic.ID, "title": topic.Title, "lessons": responseLessons})
			}
			if err := persistTeachingQuizzes(tx, subject.ID, userID, input.Quizzes, input.Quiz); err != nil {
				return err
			}
			return nil
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
