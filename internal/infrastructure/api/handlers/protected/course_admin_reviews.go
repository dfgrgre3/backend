package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminListCourseReviews returns paginated CourseReview rows for a course
// (Subject) — the table real students actually write to via
// CreateCourseReview/GetCourseReviews, unlike the hexagonal LmsReview table.
// Used by the admin panel's course review-moderation page.
func AdminListCourseReviews(c *gin.Context) {
	id := c.Param("id")

	var subject models.Subject
	query := db.DB.Select("id").WithContext(c.Request.Context())
	query = applyIDOrSlugQuery(query, id)
	if err := query.First(&subject).Error; err != nil {
		handleSubjectError(c, id, err, "resolving subject for admin review listing")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int64
	db.DB.WithContext(c.Request.Context()).Model(&models.CourseReview{}).
		Where(subjectIDQuery, subject.ID).Count(&total)

	var reviews []models.CourseReview
	if err := db.DB.WithContext(c.Request.Context()).
		Preload("User").
		Where(subjectIDQuery, subject.ID).
		Order("created_at DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"data": reviews,
		"pagination": gin.H{
			"total":      total,
			"page":       page,
			"limit":      limit,
			"totalPages": totalPages,
		},
	})
}

// AdminSetReviewVisibility toggles a CourseReview's visibility (hide/show
// from students) from the admin panel.
func AdminSetReviewVisibility(c *gin.Context) {
	var body struct {
		ID        string `json:"id" binding:"required"`
		IsVisible bool   `json:"isVisible"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Model(&models.CourseReview{}).
		Where(idQuery, body.ID).
		Update("is_visible", body.IsVisible).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update review visibility")
		return
	}

	api_response.Success(c, gin.H{"updated": true})
}

// AdminDeleteReview soft-deletes a CourseReview from the admin panel.
func AdminDeleteReview(c *gin.Context) {
	var body struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Delete(&models.CourseReview{}, idQuery, body.ID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete review")
		return
	}

	api_response.Success(c, gin.H{"deleted": true})
}
