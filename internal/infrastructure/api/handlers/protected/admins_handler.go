package protected

import (
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ListAdmins returns only privileged accounts and all totals needed by the
// administration workspace. Search, filtering, ordering and pagination are
// deliberately executed by PostgreSQL to remain safe for large installations.
func ListAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role IN ?", []models.UserRole{
		models.RoleSuperAdmin, models.RoleAdmin, models.RoleModerator, models.RoleSupport,
	})
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if twoFactor := c.Query("twoFactorEnabled"); twoFactor == "true" || twoFactor == "false" {
		query = query.Where("two_factor_enabled = ?", twoFactor == "true")
	}
	if country := c.Query("country"); country != "" {
		query = query.Where("country = ?", country)
	}
	if online := c.Query("online"); online == "true" {
		query = query.Where("last_login >= ?", time.Now().Add(-15*time.Minute))
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("name ILIKE ? OR username ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR CAST(id AS TEXT) = ?", like, like, like, like, search)
	}
	if from := c.Query("createdFrom"); from != "" {
		query = query.Where("created_at >= ?", from)
	}
	if to := c.Query("createdTo"); to != "" {
		query = query.Where("created_at < ?", to+"T23:59:59Z")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to count admins")
		return
	}
	allowed := map[string]string{"name": "name", "createdAt": "created_at", "lastLogin": "last_login", "role": "role", "status": "status"}
	sort := allowed[c.DefaultQuery("sortBy", "lastLogin")]
	if sort == "" {
		sort = "last_login"
	}
	direction := "DESC"
	if strings.EqualFold(c.Query("sortOrder"), "asc") {
		direction = "ASC"
	}

	var admins []models.User
	if err := query.Order(sort + " " + direction + " NULLS LAST").Offset((page - 1) * limit).Limit(limit).Find(&admins).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch admins")
		return
	}
	items := make([]gin.H, 0, len(admins))
	for _, admin := range admins {
		var sessionCount int64
		db.DB.Model(&models.UserSession{}).Where("user_id = ? AND is_active = ? AND expires_at > ?", admin.ID, true, time.Now()).Count(&sessionCount)
		items = append(items, gin.H{"id": admin.ID, "name": admin.Name, "username": admin.Username, "email": admin.Email, "phone": admin.Phone, "avatar": admin.Avatar, "role": admin.Role, "status": admin.Status, "twoFactorEnabled": admin.TwoFactorEnabled, "permissions": admin.GetEffectivePermissions(), "lastLogin": admin.LastLogin, "createdAt": admin.CreatedAt, "online": sessionCount > 0})
	}

	base := db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role IN ?", []models.UserRole{models.RoleSuperAdmin, models.RoleAdmin, models.RoleModerator, models.RoleSupport})
	var active, suspended, blocked, twoFactor, online, newThisMonth, failedToday int64
	base.Where("status = ?", models.StatusActive).Count(&active)
	base.Where("status = ?", models.StatusSuspended).Count(&suspended)
	base.Where("status = ?", models.StatusBanned).Count(&blocked)
	base.Where("two_factor_enabled = ?", true).Count(&twoFactor)
	db.DB.Model(&models.UserSession{}).Where("is_active = ? AND expires_at > ?", true, time.Now()).Where("user_id IN (?)", base.Select("id")).Count(&online)
	base.Where("created_at >= ?", time.Now().AddDate(0, 0, -time.Now().Day()+1).Truncate(24*time.Hour)).Count(&newThisMonth)
	db.DB.Model(&models.LoginHistory{}).Where("status = ? AND created_at >= ?", "FAILED", time.Now().Truncate(24*time.Hour)).Where("user_id IN (?)", base.Select("id")).Count(&failedToday)

	api_response.Success(c, gin.H{"admins": items, "pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": calculateTotalPages(total, limit)}, "statistics": gin.H{"total": total, "active": active, "suspended": suspended, "blocked": blocked, "online": online, "offline": total - online, "twoFactorEnabled": twoFactor, "failedLoginAttemptsToday": failedToday, "newThisMonth": newThisMonth}})
}
