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
// Section Endpoints
// =============================================================

// CreateSectionRequest represents the REST request body for creating a section.
// CourseID is optional in the body — the route is always nested under
// /courses/:id/sections, so the URL param is the source of truth; the body
// field is accepted for backward compatibility only.
type CreateSectionRequest struct {
	CourseID      string `json:"courseId"`
	Title         string `json:"title" binding:"required"`
	OrderIndex    int    `json:"orderIndex"`
	AvailableFrom *int64 `json:"availableFrom"`
	DripDelayDays *int   `json:"dripDelayDays"`
}

// UpdateSectionRequest represents the REST request body for updating a section
type UpdateSectionRequest struct {
	Title         *string `json:"title"`
	OrderIndex    *int    `json:"orderIndex"`
	AvailableFrom *int64  `json:"availableFrom"`
	DripDelayDays *int    `json:"dripDelayDays"`
}

// CreateSection creates a new section
func (h *CourseRESTHandler) CreateSection(c *gin.Context) {
	var req CreateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	courseID := c.Param("id")
	if courseID == "" {
		courseID = req.CourseID
	}
	courseUUID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	section := &models.LmsSection{
		CourseID:   courseUUID,
		Title:      req.Title,
		OrderIndex: req.OrderIndex,
	}

	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		section.AvailableFrom = &t
	}
	section.DripDelayDays = req.DripDelayDays

	createdSection, err := h.courseService.CreateSection(courseUUID, section.Title, section.OrderIndex)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to create section", err)
		return
	}

	api_response.Created(c, gin.H{"section": createdSection})
}

// UpdateSection updates a section
func (h *CourseRESTHandler) UpdateSection(c *gin.Context) {
	id := c.Param("sectionId")

	var req UpdateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid section ID")
		return
	}

	section, err := h.courseService.GetSection(parsedID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Section not found")
		return
	}

	if req.Title != nil {
		section.Title = *req.Title
	}
	if req.OrderIndex != nil {
		section.OrderIndex = *req.OrderIndex
	}
	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		section.AvailableFrom = &t
	}
	if req.DripDelayDays != nil {
		section.DripDelayDays = req.DripDelayDays
	}

	updatedSection, err := h.courseService.UpdateSection(section)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update section", err)
		return
	}

	api_response.Success(c, gin.H{"section": updatedSection})
}

// DeleteSection deletes a section
func (h *CourseRESTHandler) DeleteSection(c *gin.Context) {
	id := c.Param("sectionId")

	sectionUUID, err := uuid.Parse(id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid section ID")
		return
	}

	if err := h.courseService.DeleteSection(sectionUUID); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to delete section", err)
		return
	}

	api_response.Success(c, gin.H{"message": "Section deleted successfully"})
}

// ListSections lists sections for a course
func (h *CourseRESTHandler) ListSections(c *gin.Context) {
	courseID := c.Param("id")

	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	sections, err := h.courseService.ListSections(parsedCourseID)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to list sections", err)
		return
	}

	api_response.Success(c, gin.H{"sections": sections})
}

// ReorderSections reorders sections in a course
func (h *CourseRESTHandler) ReorderSections(c *gin.Context) {
	courseID := c.Param("id")

	var req struct {
		SectionIDs []string `json:"sectionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	sectionUUIDs := make([]uuid.UUID, 0, len(req.SectionIDs))
	for _, id := range req.SectionIDs {
		parsed, err := uuid.Parse(id)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid section ID: "+id)
			return
		}
		sectionUUIDs = append(sectionUUIDs, parsed)
	}

	if err := h.courseService.ReorderSections(parsedCourseID, sectionUUIDs); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to reorder sections", err)
		return
	}

	api_response.Success(c, gin.H{"message": "Sections reordered successfully"})
}
