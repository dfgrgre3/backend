package protected

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
)

// GetInvoice returns invoice data for a specific payment
func GetInvoice(c *gin.Context) {
	userId, _ := c.Get("userId")

	invoiceID := c.Param("id")
	if invoiceID == "" {
		invoiceID = c.Query("id")
	}

	var invoice models.Invoice
	if err := db.DB.Preload("Payment").Preload("Payment.Plan").Where("user_id = ?", userId).First(&invoice, idQuery, invoiceID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Invoice not found")
		return
	}

	api_response.Success(c, invoice)
}
