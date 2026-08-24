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
		Chapters    []struct {
			Title   string `json:"title"`
			Lessons []struct {
				Title    string `json:"title"`
				Duration string `json:"duration"`
				Type     string `json:"type"`
				URL      string `json:"url"`
				Preview  bool   `json:"isPreview"`
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

	// Create chapters and lessons if provided. Any failure here is reported
	// back to the caller instead of being silently swallowed — previously a
	// failed topic/lesson insert left the course created with fewer chapters
	// than submitted while still returning 201 Created, so the instructor had
	// no way to know part of their content was lost.
	var creationWarnings []string
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

		for j, les := range ch.Lessons {
			durationMinutes := 15
			if les.Duration != "" {
				if n, err := strconv.Atoi(strings.TrimSuffix(les.Duration, " دقيقة")); err == nil {
					durationMinutes = n
				}
			}
			subTopic := models.SubTopic{
				TopicID:         topic.ID,
				Title:           les.Title,
				Type:            models.SubTopicType(strings.ToUpper(les.Type)),
				VideoUrl:        strPtr(les.URL),
				IsFree:          les.Preview,
				Order:           j + 1,
				DurationMinutes: durationMinutes,
			}
			if err := database.Create(&subTopic).Error; err != nil {
				creationWarnings = append(creationWarnings, fmt.Sprintf("lesson %q was not saved: %v", les.Title, err))
			}
		}
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
			"createdDate":   subject.CreatedAt.Format("2006-01-02"),
			"chapters":      []interface{}{},
		},
	}
	// Surface partial-failure warnings (see comment above) instead of
	// silently returning 201 as if every chapter/lesson was saved.
	if len(creationWarnings) > 0 {
		response["warnings"] = creationWarnings
	}
	api_response.Created(c, response)
}
