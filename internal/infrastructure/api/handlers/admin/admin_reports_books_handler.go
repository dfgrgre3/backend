package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminReportsBooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.Book{}).Count(&total)

	var books []models.Book
	if err := db.DB.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&books).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books report"})
		return
	}

	items := make([]gin.H, 0, len(books))
	for _, book := range books {
		items = append(items, gin.H{
			"id":          book.ID,
			"title":       book.Title,
			"author":      book.Author,
			"price":       book.Price,
			"isFree":      book.IsFree,
			"subjectId":   book.SubjectID,
			"createdAt":   book.CreatedAt,
			"downloadUrl": book.DownloadUrl,
		})
	}

	api_response.Success(c, gin.H{
		"items": items,
		"books": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalCount": total,
			"totalPages": calculateTotalPages(total, limit),
		},
		"stats": gin.H{
			"totalBooks": total,
		},
	})
}
