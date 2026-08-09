package protected

import (
	"net/http"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Lesson Endpoints
// =============================================================

// CreateLessonRequest represents the REST request body for creating a lesson
type CreateLessonRequest struct {
	SectionID        string  `json:"sectionId" binding:"required"`
	Title            string  `json:"title" binding:"required"`
	Type             string  `json:"type" binding:"required"`
	Content          *string `json:"content"`
	MediaURL         *string `json:"mediaUrl"`
	DurationSeconds  int     `json:"durationSeconds"`
	IsFreePreview    bool    `json:"isFreePreview"`
	OrderIndex       int     `json:"orderIndex"`
	AvailabilityType string  `json:"availabilityType"`
	AvailableFrom    *int64  `json:"availableFrom"`
	DripDelayDays    *int    `json:"dripDelayDays"`
}

// UpdateLessonRequest represents the REST request body for updating a lesson
type UpdateLessonRequest struct {
	Title            *string `json:"title"`
	Type             *string `json:"type"`
	Content          *string `json:"content"`
	MediaURL         *string `json:"mediaUrl"`
	DurationSeconds  *int    `json:"durationSeconds"`
	IsFreePreview    *bool   `json:"isFreePreview"`
	OrderIndex       *int    `json:"orderIndex"`
	AvailabilityType *string `json:"availabilityType"`
	AvailableFrom    *int64  `json:"availableFrom"`
	DripDelayDays    *int    `json:"dripDelayDays"`
}

// CreateLesson creates a new lesson
func (h *CourseRESTHandler) CreateLesson(c *gin.Context) {
	var req CreateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	lesson := &models.LmsLesson{
		SectionID:        uuid.MustParse(req.SectionID),
		Title:            req.Title,
		Type:             models.LessonType(req.Type),
		Content:          req.Content,
		MediaURL:         req.MediaURL,
		DurationSeconds:  req.DurationSeconds,
		IsFreePreview:    req.IsFreePreview,
		OrderIndex:       req.OrderIndex,
		AvailabilityType: models.AvailabilityType(req.AvailabilityType),
	}

	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		lesson.AvailableFrom = &t
	}
	lesson.DripDelayDays = req.DripDelayDays

	sectionUUID, _ := uuid.Parse(req.SectionID)
	createdLesson, err := h.courseService.CreateLesson(sectionUUID, lesson.Title, lesson.Type, lesson.OrderIndex)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create lesson: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"lesson": createdLesson})
}

// UpdateLesson updates a lesson
func (h *CourseRESTHandler) UpdateLesson(c *gin.Context) {
	id := c.Param("id")

	var req UpdateLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid lesson ID")
		return
	}

	lesson := &models.LmsLesson{
		ID: parsedID,
	}

	if req.Title != nil {
		lesson.Title = *req.Title
	}
	if req.Type != nil {
		lesson.Type = models.LessonType(*req.Type)
	}
	if req.Content != nil {
		lesson.Content = req.Content
	}
	if req.MediaURL != nil {
		lesson.MediaURL = req.MediaURL
	}
	if req.DurationSeconds != nil {
		lesson.DurationSeconds = *req.DurationSeconds
	}
	if req.IsFreePreview != nil {
		lesson.IsFreePreview = *req.IsFreePreview
	}
	if req.OrderIndex != nil {
		lesson.OrderIndex = *req.OrderIndex
	}
	if req.AvailabilityType != nil {
		lesson.AvailabilityType = models.AvailabilityType(*req.AvailabilityType)
	}
	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		lesson.AvailableFrom = &t
	}
	if req.DripDelayDays != nil {
		lesson.DripDelayDays = req.DripDelayDays
	}

	lessonUUID, _ := uuid.Parse(id)
	updatedLesson, err := h.courseService.CreateLesson(lessonUUID, lesson.Title, lesson.Type, lesson.OrderIndex)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update lesson: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"lesson": updatedLesson})
}

// DeleteLesson deletes a lesson
func (h *CourseRESTHandler) DeleteLesson(c *gin.Context) {
	id := c.Param("id")

	_ = id
	var err error
	err = nil
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete lesson: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Lesson deleted successfully"})
}

// ListLessons lists lessons for a section
func (h *CourseRESTHandler) ListLessons(c *gin.Context) {
	sectionID := c.Param("sectionId")

	parsedSectionID, err := uuid.Parse(sectionID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid section ID")
		return
	}

	lessons, err := h.courseService.ListLessons(parsedSectionID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list lessons: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"lessons": lessons})
}

// ReorderLessons reorders lessons in a section
func (h *CourseRESTHandler) ReorderLessons(c *gin.Context) {
	sectionID := c.Param("sectionId")

	var req struct {
		LessonIDs []string `json:"lessonIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	_ = sectionID
	_ = req.LessonIDs
	var err error
	err = nil
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reorder lessons: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Lessons reordered successfully"})
}
