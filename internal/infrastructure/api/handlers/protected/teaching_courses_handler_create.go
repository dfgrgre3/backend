package protected

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// TeachingCreateCourse creates a new course for the authenticated instructor.
func TeachingCreateCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		Title       string  `json:"title" binding:"required"`
		Description string  `json:"description"`
		Thumbnail   string  `json:"thumbnail"`
		Price       float64 `json:"price"`
		Status      string  `json:"status"`
		Level       string  `json:"level"`
		Language    string  `json:"language"`
		CategoryID  *string `json:"categoryId"`
		Chapters    []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Lessons []struct {
				ID              string `json:"id"`
				Title           string `json:"title"`
				DurationMinutes *int   `json:"durationMinutes"`
				Duration        string `json:"duration"` // legacy clients
				Type            string `json:"type"`
				URL             string `json:"url"`
				Preview         bool   `json:"isPreview"`
			} `json:"lessons"`
		} `json:"chapters"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if input.Level == "" {
		input.Level = "INTERMEDIATE"
	}
	if input.Language == "" {
		input.Language = "ar"
	}

	status := models.CourseStatusDraft
	if input.Status == "published" || input.Status == "PUBLISHED" {
		status = models.CourseStatusPublished
	}

	thumbnail := strings.TrimSpace(input.Thumbnail)
	description := strings.TrimSpace(input.Description)

	subject := models.Subject{
		Name:          input.Title,
		Price:         decimal.NewFromFloat(input.Price),
		ThumbnailUrl:  strPtr(thumbnail),
		Description:   strPtr(description),
		Level:         models.Level(input.Level),
		Language:      input.Language,
		CategoryId:    input.CategoryID,
		Status:        status,
		InstructorId:  &userID,
		EnrolledCount: 0,
		Rating:        decimal.Zero,
		IsActive:      true,
		IsPublished:   status == models.CourseStatusPublished,
	}

	if err := database.Create(&subject).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create course")
		return
	}
	// The public catalog is backed by Redis list caches. Invalidate them so a
	// newly-published teaching course is available without waiting for TTL.
	getSubjectRepo().InvalidateSubjectCache(subject.ID)

	// Create chapters and lessons if provided. Any failure here is reported
	// back to the caller instead of being silently swallowed — previously a
	// failed topic/lesson insert left the course created with fewer chapters
	// than submitted while still returning 201 Created, so the instructor had
	// no way to know part of their content was lost.
	var creationWarnings []string
	responseChapters := make([]gin.H, 0, len(input.Chapters))
	for i, ch := range input.Chapters {
		topic := models.Topic{
			SubjectID: subject.ID,
			Title:     ch.Title,
			Order:     i + 1,
		}
		if err := database.Create(&topic).Error; err != nil {
			creationWarnings = append(creationWarnings, fmt.Sprintf("chapter %q was not saved: %v", ch.Title, err))
			continue
		}
		responseLessons := make([]gin.H, 0, len(ch.Lessons))

		for j, les := range ch.Lessons {
			durationMinutes := lessonDurationMinutes(les.DurationMinutes, les.Duration)
			subTopic := models.SubTopic{
				TopicID:         topic.ID,
				Title:           les.Title,
				Type:            normalizeTeachingLessonType(les.Type),
				VideoUrl:        strPtr(les.URL),
				IsFree:          les.Preview,
				Order:           j + 1,
				DurationMinutes: durationMinutes,
			}
			if err := database.Create(&subTopic).Error; err != nil {
				creationWarnings = append(creationWarnings, fmt.Sprintf("lesson %q was not saved: %v", les.Title, err))
				continue
			}
			responseLessons = append(responseLessons, gin.H{"id": subTopic.ID, "title": subTopic.Title, "type": string(subTopic.Type), "durationMinutes": subTopic.DurationMinutes, "isPreview": subTopic.IsFree})
		}
		responseChapters = append(responseChapters, gin.H{"id": topic.ID, "title": topic.Title, "lessons": responseLessons})
	}

	// Count created lessons
	var lessonsCount int64
	database.Model(&models.SubTopic{}).
		Joins("JOIN topic ON topic.id = sub_topic.topic_id").
		Where("topic.subject_id = ?", subject.ID).
		Count(&lessonsCount)

	response := gin.H{
		"course": gin.H{
			"id":            subject.ID,
			"title":         subject.Name,
			"description":   stringPtrToString(subject.Description),
			"thumbnail":     stringPtrToString(subject.ThumbnailUrl),
			"status":        strings.ToLower(string(subject.Status)),
			"studentsCount": 0,
			"lessonsCount":  lessonsCount,
			"rating":        0.0,
			"price":         input.Price,
			"duration":      "0 ساعة",
			"category":      "",
			"categoryId":    subject.CategoryId,
			"createdDate":   subject.CreatedAt.Format("2006-01-02"),
			"chapters":      responseChapters,
		},
	}
	// Surface partial-failure warnings (see comment above) instead of
	// silently returning 201 as if every chapter/lesson was saved.
	if len(creationWarnings) > 0 {
		response["warnings"] = creationWarnings
	}
	api_response.Created(c, response)
}

func lessonDurationMinutes(value *int, legacy string) int {
	if value != nil && *value >= 0 {
		return *value
	}
	legacy = strings.TrimSpace(legacy)
	if legacy != "" {
		if n, err := strconv.Atoi(strings.Fields(legacy)[0]); err == nil && n >= 0 {
			return n
		}
	}
	return 0
}

func normalizeTeachingLessonType(value string) models.SubTopicType {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "VIDEO":
		return models.SubTopicVideo
	case "PDF", "ARTICLE", "DOCUMENT":
		return models.SubTopicArticle
	case "QUIZ":
		return models.SubTopicQuiz
	case "ASSIGNMENT":
		return models.SubTopicAssignment
	default:
		return models.SubTopicVideo
	}
}
