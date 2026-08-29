package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Instructor Payouts Management
// ─────────────────────────────────────────────

func GetInstructorPayouts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.InstructorPayout{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"InstructorPayout\".instructor_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var payouts []models.InstructorPayout
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&payouts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payouts")
		return
	}

	items := make([]gin.H, 0, len(payouts))
	var pendingCount, paidCount int64
	var totalAmount float64
	for _, p := range payouts {
		items = append(items, payoutToGin(p))
		switch p.Status {
		case "PENDING":
			pendingCount++
		case "PAID":
			paidCount++
		}
		totalAmount += p.Amount
	}

	api_response.Success(c, gin.H{
		"payouts":    items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalPayouts": total, "pendingCount": pendingCount, "paidCount": paidCount, "totalAmount": totalAmount},
	})
}

func ApproveInstructorPayout(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := c.Get("userId")
	if err := db.DB.Model(&models.InstructorPayout{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "APPROVED",
		"approved_by": adminID,
		"approved_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to approve payout")
		return
	}
	api_response.Success(c, gin.H{"message": "Payout approved"})
}

func RejectInstructorPayout(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)
	adminID, _ := c.Get("userId")
	if err := db.DB.Model(&models.InstructorPayout{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "REJECTED",
		"approved_by": adminID,
		"approved_at": time.Now(),
		"notes":       req.Reason,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reject payout")
		return
	}
	api_response.Success(c, gin.H{"message": "Payout rejected"})
}

func payoutToGin(p models.InstructorPayout) gin.H {
	instructorName := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", p.InstructorID).First(&user).Error; err == nil {
		if user.Name != nil {
			instructorName = *user.Name
		}
	}
	return gin.H{
		"id":              p.ID,
		"instructorId":    p.InstructorID,
		"instructorName":  instructorName,
		"instructorEmail": user.Email,
		"amount":          p.Amount,
		"currency":        p.Currency,
		"status":          p.Status,
		"paymentMethod":   p.PaymentMethod,
		"transactionId":   p.TransactionID,
		"notes":           p.Notes,
		"requestedAt":     p.CreatedAt,
		"paidAt":          p.PaidAt,
	}
}
