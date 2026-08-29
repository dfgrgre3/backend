package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminReportsUsers(c *gin.Context) {
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
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.User{}).Count(&total)

	var users []models.User
	if err := db.DB.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users report"})
		return
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	examCounts := countsByKey(&models.ExamResult{}, "user_id", userIDs)
	sessionCounts := countsByKey(&models.StudySession{}, "user_id", userIDs)

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"username":      user.Username,
			"role":          user.Role,
			"status":        user.Status,
			"createdAt":     user.CreatedAt,
			"lastLogin":     user.LastLogin,
			"monthlyExams":  examCounts[user.ID],
			"studySessions": sessionCounts[user.ID],
		})
	}

	roleCounts := []gin.H{}
	for _, role := range []models.UserRole{models.RoleAdmin, models.RoleTeacher, models.RoleStudent} {
		var count int64
		db.DB.Model(&models.User{}).Where(queryRole, role).Count(&count)
		roleCounts = append(roleCounts, gin.H{"role": role, "count": count})
	}

	api_response.Success(c, gin.H{
		"items": items,
		"users": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalCount": total,
			"totalPages": calculateTotalPages(total, limit),
		},
		"stats": gin.H{
			"totalUsers": total,
			"byRole":     roleCounts,
		},
	})
}
