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

func Contests(c *gin.Context) {
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
		handleContestsGet(c, page, limit)
	case http.MethodPost:
		handleContestsPost(c)
	case http.MethodPatch:
		handleContestsUpdate(c)
	case http.MethodDelete:
		handleContestsDelete(c)
	default:
		api_response.Error(c, http.StatusMethodNotAllowed, shared.MsgMethodNotAllowed)
	}
}

func handleContestsGet(c *gin.Context, page, limit int) {
	var contests []models.Contest
	var total int64

	db.DB.Model(&models.Contest{}).Count(&total)
	if err := db.DB.Limit(limit).Offset((page - 1) * limit).Order("\"createdAt\" DESC").Find(&contests).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch contests")
		return
	}

	items := make([]gin.H, 0, len(contests))
	for _, contest := range contests {
		items = append(items, gin.H{
			"id":                contest.ID,
			"title":             contest.Title,
			"description":       contest.Description,
			"category":          contest.Category,
			"questionsCount":    contest.QuestionsCount,
			"participantsCount": contest.ParticipantsCount,
			"pinCode":           contest.PinCode,
			"status":            contest.Status,
			"createdAt":         contest.CreatedAt,
		})
	}

	pagination := gin.H{
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": (total + int64(limit) - 1) / int64(limit),
	}

	api_response.Success(c, gin.H{
		"contests":   items,
		"items":      items,
		"data":       gin.H{"contests": items, "items": items, "pagination": pagination},
		"pagination": pagination,
		"stats":      gin.H{},
	})
}

func handleContestsPost(c *gin.Context) {
	var input struct {
		Title       string  `json:"title" binding:"required"`
		Description *string `json:"description"`
		Category    *string `json:"category"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	contest := models.Contest{
		Title:       input.Title,
		Description: input.Description,
		Category:    input.Category,
		Status:      "DRAFT",
	}

	if err := SafeCreate(db.DB, &contest); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create contest")
		return
	}

	LogAudit(c, "CREATE", "contest", contest.ID, contest)
	api_response.Created(c, contest)
}

func handleContestsUpdate(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Category    *string `json:"category"`
		Status      *string `json:"status"`
		PinCode     *string `json:"pinCode"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var contest models.Contest
	if err := db.DB.Where(idQuery, id).First(&contest).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Contest not found")
		return
	}

	type contestUpdates struct {
		Title       *string `gorm:"column:title"`
		Description *string `gorm:"column:description"`
		Category    *string `gorm:"column:category"`
		Status      *string `gorm:"column:status"`
		PinCode     *string `gorm:"column:pin_code"`
	}

	updates := contestUpdates{
		Title:       input.Title,
		Description: input.Description,
		Category:    input.Category,
		Status:      input.Status,
		PinCode:     input.PinCode,
	}

	if err := db.DB.Model(&models.Contest{}).Where(idQuery, id).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update contest")
		return
	}

	LogAudit(c, "UPDATE", "contest", id, updates)
	api_response.Success(c, nil)
}

func handleContestsDelete(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Where(idQuery, id).Delete(&models.Contest{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete contest")
		return
	}

	LogAudit(c, "DELETE", "contest", id, nil)
	api_response.Success(c, nil)
}
