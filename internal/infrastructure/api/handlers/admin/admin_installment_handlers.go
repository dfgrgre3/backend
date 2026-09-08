package admin

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AdminListInstallments returns installment plans across all users,
// paginated and filterable by status/search. Backed by "Installment".
func AdminListInstallments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Installment{})
	if status != "" {
		query = query.Where("\"Installment\".status = ?", strings.ToLower(status))
	}
	if search != "" {
		query = query.Joins("LEFT JOIN \"User\" ON \"Installment\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"Installment\".reference ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var installments []models.Installment
	if err := query.
		Preload("User").
		Order("\"Installment\".due_date ASC").
		Offset(offset).
		Limit(limit).
		Find(&installments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch installments")
		return
	}

	items := make([]gin.H, 0, len(installments))
	for _, i := range installments {
		userName := ""
		if i.User.Name != nil {
			userName = *i.User.Name
		}
		items = append(items, gin.H{
			"id":                i.ID,
			"userId":            i.UserID,
			"paymentId":         i.PaymentID,
			"installmentNumber": i.InstallmentNumber,
			"totalInstallments": i.TotalInstallments,
			"amount":            i.Amount,
			"currency":          i.Currency,
			"status":            i.Status,
			"dueDate":           i.DueDate,
			"paidAt":            i.PaidAt,
			"method":            i.Method,
			"reference":         i.Reference,
			"notes":             i.Notes,
			"createdAt":         i.CreatedAt,
			"user": gin.H{
				"id":    i.User.ID,
				"name":  userName,
				"email": i.User.Email,
			},
		})
	}

	var pendingCount, paidCount, overdueCount int64
	var pendingTotal float64
	db.DB.Model(&models.Installment{}).Where("status = ?", models.InstallmentPending).Count(&pendingCount)
	db.DB.Model(&models.Installment{}).Where("status = ?", models.InstallmentPaid).Count(&paidCount)
	db.DB.Model(&models.Installment{}).Where("status = ?", models.InstallmentOverdue).Count(&overdueCount)
	db.DB.Model(&models.Installment{}).Where("status IN ?", []models.InstallmentStatus{models.InstallmentPending, models.InstallmentOverdue}).
		Select("COALESCE(SUM(amount), 0)").Scan(&pendingTotal)

	api_response.Success(c, gin.H{
		"installments": items,
		"summary": gin.H{
			"pendingCount":  pendingCount,
			"paidCount":     paidCount,
			"overdueCount":  overdueCount,
			"pendingAmount": pendingTotal,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// AdminCreateInstallmentPlan splits an amount into N scheduled installments
// for a user, optionally linked to an existing Payment.
func AdminCreateInstallmentPlan(c *gin.Context) {
	var req struct {
		UserID       string  `json:"userId" binding:"required,uuid"`
		PaymentID    string  `json:"paymentId" binding:"omitempty,uuid"`
		TotalAmount  float64 `json:"totalAmount" binding:"required,gt=0"`
		Installments int     `json:"installments" binding:"required,gt=0,lte=36"`
		Currency     string  `json:"currency"`
		FirstDueDate string  `json:"firstDueDate" binding:"required"` // YYYY-MM-DD
		IntervalDays int     `json:"intervalDays"`                    // days between installments, default 30
		Notes        string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid installment plan payload")
		return
	}

	firstDue, err := time.Parse("2006-01-02", req.FirstDueDate)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid firstDueDate, expected YYYY-MM-DD")
		return
	}
	if req.IntervalDays <= 0 {
		req.IntervalDays = 30
	}
	currency := req.Currency
	if currency == "" {
		currency = "EGP"
	}

	var paymentID *string
	if req.PaymentID != "" {
		paymentID = &req.PaymentID
	}

	perInstallment := decimal.NewFromFloat(req.TotalAmount).Div(decimal.NewFromInt(int64(req.Installments))).Round(2)

	plans := make([]models.Installment, 0, req.Installments)
	for n := 1; n <= req.Installments; n++ {
		plans = append(plans, models.Installment{
			ID:                uuid.New().String(),
			UserID:            req.UserID,
			PaymentID:         paymentID,
			InstallmentNumber: n,
			TotalInstallments: req.Installments,
			Amount:            perInstallment,
			Currency:          currency,
			Status:            models.InstallmentPending,
			DueDate:           firstDue.AddDate(0, 0, req.IntervalDays*(n-1)),
			Notes:             req.Notes,
		})
	}

	if err := db.DB.Create(&plans).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create installment plan")
		return
	}

	api_response.Success(c, gin.H{"message": "Installment plan created", "count": len(plans)})
}

// AdminMarkInstallmentPaid marks a single installment as paid.
func AdminMarkInstallmentPaid(c *gin.Context) {
	id := c.Param("id")
	now := time.Now()
	result := db.DB.Model(&models.Installment{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":  models.InstallmentPaid,
		"paid_at": now,
	})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update installment")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Installment not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Installment marked as paid"})
}

// AdminCancelInstallment cancels a pending installment.
func AdminCancelInstallment(c *gin.Context) {
	id := c.Param("id")
	result := db.DB.Model(&models.Installment{}).Where("id = ?", id).Update("status", models.InstallmentCancelled)
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to cancel installment")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Installment not found")
		return
	}
	api_response.Success(c, gin.H{"message": "Installment cancelled"})
}
