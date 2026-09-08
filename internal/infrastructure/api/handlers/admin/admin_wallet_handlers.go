package admin

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// AdminListWallets returns every user's wallet balance, paginated and
// searchable by name/email. Backed by the real "User".balance column.
func AdminListWallets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	search := strings.TrimSpace(c.Query("search"))
	offset := (page - 1) * limit

	query := db.DB.Model(&models.User{})
	if search != "" {
		query = query.Where("name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var users []models.User
	if err := query.
		Select("id", "name", "email", "avatar", "balance", "created_at").
		Order("balance DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch wallets")
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		name := ""
		if u.Name != nil {
			name = *u.Name
		}
		avatar := ""
		if u.Avatar != nil {
			avatar = *u.Avatar
		}
		items = append(items, gin.H{
			"userId":   u.ID,
			"name":     name,
			"email":    u.Email,
			"avatar":   avatar,
			"balance":  u.Balance,
			"currency": "EGP",
		})
	}

	var totalBalance float64
	db.DB.Model(&models.User{}).Select("COALESCE(SUM(balance), 0)").Scan(&totalBalance)

	api_response.Success(c, gin.H{
		"wallets": items,
		"summary": gin.H{
			"totalWallets": total,
			"totalBalance": totalBalance,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// AdminListWalletTransactions returns wallet transactions across all users,
// paginated/filterable by type and search. Backed by "WalletTransaction".
func AdminListWalletTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	search := strings.TrimSpace(c.Query("search"))
	txType := c.Query("type")
	offset := (page - 1) * limit

	query := db.DB.Model(&models.WalletTransaction{})
	if txType != "" {
		query = query.Where("\"WalletTransaction\".type = ?", strings.ToUpper(txType))
	}
	if search != "" {
		query = query.Joins("LEFT JOIN \"User\" ON \"WalletTransaction\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"WalletTransaction\".description ILIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var transactions []models.WalletTransaction
	if err := query.
		Preload("User").
		Order("\"WalletTransaction\".created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch wallet transactions")
		return
	}

	items := make([]gin.H, 0, len(transactions))
	for _, t := range transactions {
		userName := ""
		if t.User.Name != nil {
			userName = *t.User.Name
		}
		items = append(items, gin.H{
			"id":          t.ID,
			"userId":      t.UserID,
			"type":        t.Type,
			"amount":      t.Amount,
			"currency":    "EGP",
			"walletType":  t.WalletType,
			"description": t.Description,
			"referenceId": t.ReferenceID,
			"status":      t.Status,
			"createdAt":   t.CreatedAt,
			"user": gin.H{
				"id":    t.User.ID,
				"name":  userName,
				"email": t.User.Email,
			},
		})
	}

	api_response.Success(c, gin.H{
		"transactions": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}
