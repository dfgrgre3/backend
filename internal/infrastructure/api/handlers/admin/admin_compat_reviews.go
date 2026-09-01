package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func AdminBookReviews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	switch c.Request.Method {
	case http.MethodGet:
		// Check which endpoint was called
		isViewsEndpoint := c.FullPath() == "/api/v1/admin/books/views"

		if isViewsEndpoint {
			// Return book view statistics from the Book model
			var books []models.Book
			var total int64
			db.DB.Model(&models.Book{}).Count(&total)
			db.DB.Order("views DESC").Limit(limit).Offset((page - 1) * limit).Find(&books)

			var totalViews int64
			db.DB.Model(&models.Book{}).Select("COALESCE(SUM(views), 0)").Scan(&totalViews)

			items := make([]gin.H, 0, len(books))
			for _, b := range books {
				items = append(items, gin.H{
					"id": b.ID, "title": b.Title, "author": b.Author,
					"views": b.Views, "downloads": b.Downloads,
					"coverUrl": b.CoverUrl, "createdAt": b.CreatedAt,
				})
			}
			api_response.Success(c, gin.H{
				"views": items, "items": items,
				"pagination": gin.H{
					"page": page, "limit": limit, "total": total,
					"totalPages": (total + int64(limit) - 1) / int64(limit),
				},
				"stats": gin.H{"totalViews": totalViews},
			})
		} else {
			// Return course reviews (which also cover books)
			var reviews []models.CourseReview
			var total int64
			db.DB.Model(&models.CourseReview{}).Count(&total)
			db.DB.Preload("User").Order(createdAtDescSort).Limit(limit).Offset((page - 1) * limit).Find(&reviews)

			var avgRating float64
			db.DB.Model(&models.CourseReview{}).Select("COALESCE(AVG(rating), 0)").Scan(&avgRating)

			api_response.Success(c, gin.H{
				"reviews": reviews, "items": reviews,
				"pagination": gin.H{
					"page": page, "limit": limit, "total": total,
					"totalPages": (total + int64(limit) - 1) / int64(limit),
				},
				"stats": gin.H{"totalReviews": total, "avgRating": avgRating},
			})
		}

	case http.MethodDelete:
		var input struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&input); err != nil || input.ID == "" {
			api_response.Error(c, http.StatusBadRequest, shared.MsgIDRequired)
			return
		}
		if err := db.DB.Where(idQuery, input.ID).Delete(&models.CourseReview{}).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to delete review")
			return
		}
		LogAudit(c, "DELETE", "course_review", input.ID, nil)
		api_response.Success(c, nil)

	default:
		api_response.Error(c, http.StatusMethodNotAllowed, shared.MsgMethodNotAllowed)
	}
}
