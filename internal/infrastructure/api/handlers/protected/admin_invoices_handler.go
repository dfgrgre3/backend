package protected

import (
	"math"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetAdminInvoices returns paginated invoices with summary stats.
// Registered at GET /api/admin/invoices (admin panel: الاشتراك والفواتير).
func GetAdminInvoices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Base query joins Payment to allow filtering by payment status and
	// to expose amount/currency/method on each invoice row.
	query := db.DB.Model(&models.Invoice{}).
		Joins("LEFT JOIN \"Payment\" ON \"Payment\".id = \"Invoice\".payment_id")

	if status != "" {
		query = query.Where("\"Payment\".status = ?", status)
	}

	if search != "" {
		query = query.Joins("LEFT JOIN \"User\" ON \"User\".id = \"Invoice\".user_id").
			Where("\"Invoice\".invoice_number ILIKE ? OR \"User\".name ILIKE ? OR \"User\".email ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var invoices []models.Invoice
	if err := query.
		Preload("Payment").
		Preload("Payment.Subject").
		Order("\"Invoice\".created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&invoices).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch invoices")
		return
	}

	items := make([]gin.H, 0, len(invoices))
	for _, inv := range invoices {
		var user models.User
		db.DB.Select("id", "name", "email", "avatar").Where("id = ?", inv.UserID).First(&user)

		userName := ""
		if user.Name != nil {
			userName = *user.Name
		}
		userAvatar := ""
		if user.Avatar != nil {
			userAvatar = *user.Avatar
		}

		planName := ""
		if inv.Payment.Subject != nil {
			if inv.Payment.Subject.NameAr != nil && *inv.Payment.Subject.NameAr != "" {
				planName = *inv.Payment.Subject.NameAr
			} else {
				planName = inv.Payment.Subject.Name
			}
		}

		items = append(items, gin.H{
			"id":            inv.ID,
			"invoiceNumber": inv.InvoiceNumber,
			"pdfUrl":        inv.PdfUrl,
			"createdAt":     inv.CreatedAt,
			"updatedAt":     inv.UpdatedAt,
			"payment": gin.H{
				"id":        inv.Payment.ID,
				"amount":    inv.Payment.Amount,
				"currency":  inv.Payment.Currency,
				"status":    inv.Payment.Status,
				"method":    inv.Payment.Method,
				"reference": inv.Payment.Reference,
			},
			"planName": planName,
			"user": gin.H{
				"id":     user.ID,
				"name":   userName,
				"email":  user.Email,
				"avatar": userAvatar,
			},
		})
	}

	var totalAmount float64
	db.DB.Model(&models.Invoice{}).
		Joins("LEFT JOIN \"Payment\" ON \"Payment\".id = \"Invoice\".payment_id").
		Where("\"Payment\".status = ?", models.PaymentCompleted).
		Select(revenueSumQuery).Scan(&totalAmount)

	var paidCount, pendingCount, failedCount int64
	countByStatus := func(s models.PaymentStatus) int64 {
		var n int64
		db.DB.Model(&models.Invoice{}).
			Joins("LEFT JOIN \"Payment\" ON \"Payment\".id = \"Invoice\".payment_id").
			Where("\"Payment\".status = ?", s).Count(&n)
		return n
	}
	paidCount = countByStatus(models.PaymentCompleted)
	pendingCount = countByStatus(models.PaymentPending)
	failedCount = countByStatus(models.PaymentFailed)

	api_response.Success(c, gin.H{
		"invoices": items,
		"summary": gin.H{
			"totalInvoices": total,
			"totalAmount":   totalAmount,
			"paidCount":     paidCount,
			"pendingCount":  pendingCount,
			"failedCount":   failedCount,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// GetAdminInvoice returns a single invoice by id with full payment/user details.
// Registered at GET /api/admin/invoices/:id.
func GetAdminInvoice(c *gin.Context) {
	id := c.Param("id")

	var invoice models.Invoice
	if err := db.DB.
		Preload("Payment").
		Preload("Payment.Subject").
		Where("id = ?", id).
		First(&invoice).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Invoice not found")
		return
	}

	var user models.User
	db.DB.Select("id", "name", "email", "avatar", "phone").Where("id = ?", invoice.UserID).First(&user)

	userName := ""
	if user.Name != nil {
		userName = *user.Name
	}
	userAvatar := ""
	if user.Avatar != nil {
		userAvatar = *user.Avatar
	}

	planName := ""
	if invoice.Payment.Subject != nil {
		if invoice.Payment.Subject.NameAr != nil && *invoice.Payment.Subject.NameAr != "" {
			planName = *invoice.Payment.Subject.NameAr
		} else {
			planName = invoice.Payment.Subject.Name
		}
	}

	api_response.Success(c, gin.H{
		"id":            invoice.ID,
		"invoiceNumber": invoice.InvoiceNumber,
		"pdfUrl":        invoice.PdfUrl,
		"createdAt":     invoice.CreatedAt,
		"updatedAt":     invoice.UpdatedAt,
		"payment": gin.H{
			"id":            invoice.Payment.ID,
			"amount":        invoice.Payment.Amount,
			"currency":      invoice.Payment.Currency,
			"status":        invoice.Payment.Status,
			"method":        invoice.Payment.Method,
			"reference":     invoice.Payment.Reference,
			"completedAt":   invoice.Payment.CompletedAt,
			"externalTxnId": invoice.Payment.ExternalTxnID,
		},
		"planName": planName,
		"user": gin.H{
			"id":     user.ID,
			"name":   userName,
			"email":  user.Email,
			"avatar": userAvatar,
		},
	})
}
