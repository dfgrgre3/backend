package protected

import (
	"log"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	params := userListParams{
		role:               c.Query("role"),
		status:             c.Query("status"),
		search:             c.Query("search"),
		searchType:         c.Query("searchType"),
		sortBy:             c.DefaultQuery("sortBy", "createdAt"),
		sortOrder:          c.DefaultQuery("sortOrder", "desc"),
		emailVerified:      c.Query("emailVerified"),
		twoFactorEnabled:   c.Query("twoFactorEnabled"),
		country:            c.Query("country"),
		city:               c.Query("city"),
		gender:             c.Query("gender"),
		gradeLevel:         c.Query("gradeLevel"),
		createdFrom:        c.Query("createdFrom"),
		createdTo:          c.Query("createdTo"),
		subscriptionStatus: c.Query("subscriptionStatus"),
		includeDeleted:     c.Query("includeDeleted") == "true",
		isNew:              c.Query("isNew") == "true",
	}

	query := buildUserListQuery(params)
	orderClause := buildUserListOrderClause(params.sortBy, params.sortOrder)

	var total int64
	query.Count(&total)

	var users []models.User
	if err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	// Summary stats
	var totalUsers, totalAdmins, powerUsers int64
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL").Count(&totalUsers)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ?", models.RoleAdmin).Count(&totalAdmins)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND level >= ?", 10).Count(&powerUsers)

	// Batch fetch _count data
	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	taskMap, sessionMap, achievementMap, enrollmentMap := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	if len(userIDs) > 0 {
		rows, err := fetchUserAggregateCounts(userIDs)
		if err != nil {
			log.Printf("[GetUsers] failed to fetch aggregate user counts: %v", err)
		} else {
			taskMap, sessionMap, achievementMap, enrollmentMap = aggregateUserCountRows(rows)
		}
	}

	items := buildUserListItems(users, taskMap, sessionMap, achievementMap, enrollmentMap)

	api_response.List(c, items, api_response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"users": items,
		"summary": gin.H{
			"totalUsers":  totalUsers,
			"totalAdmins": totalAdmins,
			"powerUsers":  powerUsers,
		},
	})
}
