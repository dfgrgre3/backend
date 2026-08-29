package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminListOrders returns paginated orders plus a status-breakdown summary,
// matching the shape the admin panel's Orders page
// (d:/admin .../admin/orders/page.tsx) already expects — that UI predates
// any backend route serving it.
func AdminListOrders(c *gin.Context) {
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

	q := db.DB.WithContext(c.Request.Context()).Model(&models.Order{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if search := c.Query("search"); search != "" {
		search = sanitizeSearchTerm(search)
		if search != "" {
			q = q.Where("order_number ILIKE ?", "%"+search+"%")
		}
	}

	var total int64
	q.Count(&total)

	var orders []models.Order
	if err := q.
		Preload("User").
		Preload("Items").
		Order("created_at DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&orders).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch orders")
		return
	}

	var completedCount, pendingCount, cancelledCount int64
	var totalRevenue decimal.NullDecimal
	db.DB.Model(&models.Order{}).Where("status = ?", models.OrderCompleted).Count(&completedCount)
	db.DB.Model(&models.Order{}).Where("status = ?", models.OrderPending).Count(&pendingCount)
	db.DB.Model(&models.Order{}).Where("status = ?", models.OrderCancelled).Count(&cancelledCount)
	db.DB.Model(&models.Order{}).Where("status = ?", models.OrderCompleted).Select("SUM(total)").Scan(&totalRevenue)

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	revenue := decimal.Zero
	if totalRevenue.Valid {
		revenue = totalRevenue.Decimal
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"orders": orders,
			"summary": gin.H{
				"totalOrders":    total,
				"completedCount": completedCount,
				"pendingCount":   pendingCount,
				"cancelledCount": cancelledCount,
				"totalRevenue":   revenue,
			},
			"pagination": gin.H{
				"page":       page,
				"limit":      limit,
				"total":      total,
				"totalPages": totalPages,
			},
		},
	})
}

// AdminUpdateOrderStatus lets an admin change an order's status (e.g. mark
// a manual refund).
func AdminUpdateOrderStatus(c *gin.Context) {
	var body struct {
		ID     string `json:"id" binding:"required"`
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api_response.Error(c, http.StatusBadRequest, msgInvalidInput)
		return
	}

	if err := db.DB.WithContext(c.Request.Context()).
		Model(&models.Order{}).
		Where(idQuery, body.ID).
		Update("status", body.Status).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update order")
		return
	}

	api_response.Success(c, gin.H{"updated": true})
}
