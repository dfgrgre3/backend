package protected

import (
	"errors"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
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
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Thumbnail   *string  `json:"thumbnail"`
		Price       *float64 `json:"price"`
		Status      *string  `json:"status"`
		Level       *string  `json:"level"`
		Language    *string  `json:"language"`
		TrailerUrl  *string  `json:"trailerUrl"`
		ShortDesc   *string  `json:"shortDescription"`
		LongDesc    *string  `json:"longDescription"`
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
	}

	api_response.Success(c, gin.H{"message": "Course updated successfully"})
}
