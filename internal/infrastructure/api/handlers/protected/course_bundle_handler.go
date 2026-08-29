package protected

import (
	"net/http"
	"strconv"
	"strings"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// =============================================================
// Bundle Handler Methods on CourseRESTHandler
// =============================================================

// =============================================================
// Bundle Handler Methods on CourseRESTHandler
// =============================================================

// ListBundles returns a paginated list of bundles
func (h *CourseRESTHandler) ListBundles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	bundles, total, err := h.courseService.ListBundles(page, pageSize)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch bundles", err)
		return
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = (int(total) + pageSize - 1) / pageSize
	}

	api_response.Success(c, gin.H{
		"bundles": bundles,
		"pagination": gin.H{
			"page":       page,
			"pageSize":   pageSize,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

// GetBundle returns a single bundle by ID
func (h *CourseRESTHandler) GetBundle(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	bundle, err := h.courseService.GetBundle(bundleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Bundle not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch bundle", err)
		return
	}

	api_response.Success(c, gin.H{"bundle": bundle})
}

// CreateBundle creates a new bundle
func (h *CourseRESTHandler) CreateBundle(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description *string  `json:"description,omitempty"`
		Price       float64  `json:"price"`
		Currency    string   `json:"currency"`
		CourseIDs   []string `json:"courseIds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Currency == "" {
		req.Currency = "EGP"
	}

	bundle, err := h.courseService.CreateBundle(req.Name, generateBundleSlug(req.Name), req.Price, req.Currency)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to create bundle", err)
		return
	}

	// Add courses to bundle if provided
	for _, courseIDStr := range req.CourseIDs {
		courseID, err := uuid.Parse(courseIDStr)
		if err != nil {
			continue // Skip invalid course IDs
		}
		if err := h.courseService.AddCourseToBundle(bundle.ID, courseID); err != nil {
			continue
		}
	}

	api_response.Created(c, gin.H{"bundle": bundle})
}

// UpdateBundle updates an existing bundle
func (h *CourseRESTHandler) UpdateBundle(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	bundle, err := h.courseService.GetBundle(bundleUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Bundle not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch bundle", err)
		return
	}

	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Price       *float64 `json:"price"`
		Currency    *string  `json:"currency"`
		IsActive    *bool    `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if req.Name != nil && *req.Name != "" {
		bundle.Title = *req.Name
	}
	if req.Description != nil {
		bundle.Description = req.Description
	}
	if req.Price != nil {
		if *req.Price < 0 {
			api_response.Error(c, http.StatusBadRequest, "Price cannot be negative")
			return
		}
		bundle.Price = decimal.NewFromFloat(*req.Price)
	}
	if req.Currency != nil && *req.Currency != "" {
		bundle.CurrencyCode = *req.Currency
	}
	if req.IsActive != nil {
		bundle.IsActive = *req.IsActive
	}

	if err := h.courseService.UpdateBundle(bundle); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update bundle", err)
		return
	}

	api_response.Success(c, gin.H{"bundle": bundle})
}

// DeleteBundle deletes a bundle
func (h *CourseRESTHandler) DeleteBundle(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	if err := h.courseService.DeleteBundle(bundleUUID); err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to delete bundle", err)
		return
	}

	api_response.Success(c, gin.H{"message": "Bundle deleted successfully"})
}

// AddCoursesToBundle adds courses to a bundle
func (h *CourseRESTHandler) AddCoursesToBundle(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	var req struct {
		CourseIDs []string `json:"courseIds" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Verify bundle exists
	if _, err = h.courseService.GetBundle(bundleUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Bundle not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch bundle", err)
		return
	}

	added := []string{}
	for _, courseIDStr := range req.CourseIDs {
		courseID, err := uuid.Parse(courseIDStr)
		if err != nil {
			continue
		}
		if err := h.courseService.AddCourseToBundle(bundleUUID, courseID); err != nil {
			continue
		}
		added = append(added, courseIDStr)
	}

	api_response.Success(c, gin.H{
		"message": "Courses added to bundle",
		"added":   added,
	})
}

// RemoveCoursesFromBundle removes courses from a bundle
func (h *CourseRESTHandler) RemoveCoursesFromBundle(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	var req struct {
		CourseIDs []string `json:"courseIds" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Verify bundle exists
	if _, err = h.courseService.GetBundle(bundleUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Bundle not found")
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch bundle", err)
		return
	}

	removed := []string{}
	for _, courseIDStr := range req.CourseIDs {
		courseID, err := uuid.Parse(courseIDStr)
		if err != nil {
			continue
		}
		if err := h.courseService.RemoveCourseFromBundle(bundleUUID, courseID); err != nil {
			continue
		}
		removed = append(removed, courseIDStr)
	}

	api_response.Success(c, gin.H{
		"message": "Courses removed from bundle",
		"removed": removed,
	})
}

// GetBundleEnrollments returns enrollments for a bundle
func (h *CourseRESTHandler) GetBundleEnrollments(c *gin.Context) {
	bundleUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid bundle ID")
		return
	}

	var enrollments []models.BundleEnrollment
	if err := h.db.WithContext(c.Request.Context()).
		Where("bundle_id = ?", bundleUUID.String()).
		Preload("User").
		Order("created_at DESC").
		Find(&enrollments).Error; err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch enrollments", err)
		return
	}

	api_response.Success(c, gin.H{
		"enrollments": enrollments,
		"total":       len(enrollments),
	})
}

// generateBundleSlug creates a URL-friendly slug from a bundle name.
func generateBundleSlug(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "bundle-" + uuid.NewString()[:8]
	}
	return slug
}
