package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetWalletBalance(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := db.DB.First(&user, "id = ?", userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	var transactions []models.WalletTransaction
	db.DB.Where("user_id = ?", userId).Order("created_at desc").Limit(20).Find(&transactions)

	api_response.Success(c, gin.H{
		"balance":      user.Balance,
		"currency":     "EGP",
		"transactions": transactions,
		"history":      transactions, // For compatibility with different frontend versions
	})
}
