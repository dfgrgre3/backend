package handlers

import (
	"net/http"
	"strconv"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

type refundCompat struct {
	ID string `json:"id"`
}

func AdminListLessons(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	var lessons []models.LmsLesson
	if err := db.DB.Order("created_at DESC").Offset((page-1)*limit).Limit(limit).Find(&lessons).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch lessons")
		return
	}
	api_response.Success(c, gin.H{"items": lessons, "pagination": gin.H{"page": page, "limit": limit, "total": len(lessons)}})
}

func AdminGetLesson(c *gin.Context) {
	id := c.Param("id")
	var lesson models.LmsLesson
	if err := db.DB.Where("id = ?", id).First(&lesson).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}
	api_response.Success(c, lesson)
}

func AdminCreateLesson(c *gin.Context) {
	var lesson models.LmsLesson
	if err := c.ShouldBindJSON(&lesson); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	lesson.CreatedAt = time.Now()
	lesson.UpdatedAt = lesson.CreatedAt
	if err := db.DB.Create(&lesson).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create lesson")
		return
	}
	api_response.Success(c, lesson)
}

func AdminUpdateLesson(c *gin.Context) {
	id := c.Param("id")
	var lesson models.LmsLesson
	if err := db.DB.Where("id = ?", id).First(&lesson).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Lesson not found")
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&lesson).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update lesson")
		return
	}
	api_response.Success(c, gin.H{"message": "Lesson updated"})
}

func AdminDeleteLesson(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.LmsLesson{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete lesson")
		return
	}
	api_response.Success(c, gin.H{"message": "Lesson deleted"})
}

func AdminListRefunds(c *gin.Context) {
	api_response.Success(c, gin.H{"items": []refundCompat{}, "pagination": gin.H{"page": 1, "limit": 10, "total": 0}})
}

func AdminGetRefund(c *gin.Context) {
	api_response.Error(c, http.StatusNotFound, "Refund not found")
}

func AdminApproveRefund(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Refund approved"})
}

func AdminRejectRefund(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Refund rejected"})
}

func AdminProcessRefund(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Refund processed"})
}

func AdminListTaxRates(c *gin.Context) {
	api_response.Success(c, gin.H{"items": []gin.H{}, "pagination": gin.H{"page": 1, "limit": 10, "total": 0}})
}

func AdminGetTaxRate(c *gin.Context) {
	api_response.Error(c, http.StatusNotFound, "Tax rate not found")
}

func AdminCreateTaxRate(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Tax rate created"})
}

func AdminUpdateTaxRate(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Tax rate updated"})
}

func AdminDeleteTaxRate(c *gin.Context) {
	api_response.Success(c, gin.H{"message": "Tax rate deleted"})
}

// The canonical admin module handlers live in admin_new_modules.go.
// Keep this compatibility file free of duplicate declarations so the backend builds.
