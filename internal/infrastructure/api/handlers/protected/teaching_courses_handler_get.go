package protected

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TeachingGetCourse returns a single course for the instructor.
func TeachingGetCourse(c *gin.Context) {
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

	var subject models.Subject
	if err := database.
		Where("id = ? AND instructor_id = ?", courseID, userID).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC").
				Preload("SubTopics", func(db *gorm.DB) *gorm.DB {
					return db.Order("\"order\" ASC")
				})
		}).
		First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	price, _ := subject.Price.Float64()
	rating, _ := subject.Rating.Float64()

	var chapters []gin.H
	lessonsCount := 0
	for _, t := range subject.Topics {
		var lessons []gin.H
		for _, st := range t.SubTopics {
			lessonsCount++
			lessons = append(lessons, gin.H{
				"id":        st.ID,
				"title":     st.Title,
				"duration":  fmt.Sprintf("%d دقيقة", st.DurationMinutes),
				"type":      strings.ToLower(string(st.Type)),
				"url":       st.VideoUrl,
				"isPreview": st.IsFree,
			})
		}
		chapters = append(chapters, gin.H{
			"id":      t.ID,
			"title":   t.Title,
			"lessons": lessons,
		})
	}

	status := "draft"
	switch subject.Status {
	case models.CourseStatusPublished:
		status = "published"
	case models.CourseStatusArchived:
		status = "archived"
	}

	api_response.Success(c, gin.H{
		"course": gin.H{
			"id":               subject.ID,
			"title":            subject.Name,
			"description":      stringPtrToString(subject.Description),
			"shortDescription": stringPtrToString(subject.ShortDescription),
			"thumbnail":        stringPtrToString(subject.ThumbnailUrl),
			"trailerUrl":       stringPtrToString(subject.TrailerUrl),
			"status":           status,
			"studentsCount":    subject.EnrolledCount,
			"lessonsCount":     lessonsCount,
			"rating":           rating,
			"price":            price,
			"duration":         fmt.Sprintf("%d ساعة", subject.DurationHours),
			"category":         stringPtrToString(subject.CategoryId),
			"level":            string(subject.Level),
			"language":         subject.Language,
			"createdDate":      subject.CreatedAt.Format("2006-01-02"),
			"chapters":         chapters,
			"enrolledCount":    subject.EnrolledCount,
			"completionRate":   0.0,
			"isFeatured":       subject.IsFeatured,
		},
	})
}
