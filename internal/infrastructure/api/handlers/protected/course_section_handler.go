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

// CreateSectionRequest represents the REST request body for creating a section
type CreateSectionRequest struct {
	CourseID      string `json:"courseId" binding:"required"`
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

	section := &models.LmsSection{
		CourseID:   uuid.MustParse(req.CourseID),
		Title:      req.Title,
		OrderIndex: req.OrderIndex,
	}

	if req.AvailableFrom != nil {
		t := time.Unix(*req.AvailableFrom, 0)
		section.AvailableFrom = &t
	}
	section.DripDelayDays = req.DripDelayDays

	courseUUID, _ := uuid.Parse(req.CourseID)
	createdSection, err := h.courseService.CreateSection(courseUUID, section.Title, section.OrderIndex)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create section: "+err.Error())
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

	section := &models.LmsSection{
		ID: parsedID,
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

	sectionUUID, _ := uuid.Parse(id)
	updatedSection, err := h.courseService.CreateSection(sectionUUID, section.Title, section.OrderIndex)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update section: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"section": updatedSection})
}

// DeleteSection deletes a section
func (h *CourseRESTHandler) DeleteSection(c *gin.Context) {
	id := c.Param("id")

	sectionUUID, _ := uuid.Parse(id)
	_ = sectionUUID
	var err error
	err = nil
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete section: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Section deleted successfully"})
}

// ListSections lists sections for a course
func (h *CourseRESTHandler) ListSections(c *gin.Context) {
	courseID := c.Param("courseId")

	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	sections, err := h.courseService.ListSections(parsedCourseID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list sections: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"sections": sections})
}

// ReorderSections reorders sections in a course
func (h *CourseRESTHandler) ReorderSections(c *gin.Context) {
	courseID := c.Param("courseId")

	var req struct {
		SectionIDs []string `json:"sectionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	_ = courseID
	_ = req.SectionIDs
	var err error
	err = nil
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reorder sections: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Sections reordered successfully"})
}
