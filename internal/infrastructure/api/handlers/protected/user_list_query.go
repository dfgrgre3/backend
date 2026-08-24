package protected

import (
	"time"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/gorm"
)

type userListParams struct {
	role               string
	status             string
	search             string
	searchType         string
	sortBy             string
	sortOrder          string
	emailVerified      string
	twoFactorEnabled   string
	country            string
	city               string
	gender             string
	gradeLevel         string
	createdFrom        string
	createdTo          string
	subscriptionStatus string
}

func buildUserListQuery(p userListParams) *gorm.DB {
	query := db.DB.Model(&models.User{}).Where("deleted_at IS NULL")

	if p.role != "" {
		query = query.Where("role = ?", p.role)
	}
	if p.status != "" {
		query = query.Where("status = ?", p.status)
	}

	if p.search != "" {
		switch p.searchType {
		case "name":
			query = query.Where("name ILIKE ?", "%"+p.search+"%")
		case "email":
			query = query.Where("email ILIKE ?", "%"+p.search+"%")
		case "username":
			query = query.Where("username ILIKE ?", "%"+p.search+"%")
		case "phone":
			query = query.Where("phone ILIKE ?", "%"+p.search+"%")
		case "userId":
			query = query.Where("id = ?", p.search)
		default:
			query = query.Where(
				"(email ILIKE ? OR name ILIKE ? OR username ILIKE ? OR phone ILIKE ?)",
				"%"+p.search+"%", "%"+p.search+"%", "%"+p.search+"%", "%"+p.search+"%",
			)
		}
	}

	switch p.emailVerified {
	case "true":
		query = query.Where("email_verified = ?", true)
	case "false":
		query = query.Where("email_verified = ?", false)
	}

	switch p.twoFactorEnabled {
	case "true":
		query = query.Where("two_factor_enabled = ?", true)
	case "false":
		query = query.Where("two_factor_enabled = ?", false)
	}

	if p.country != "" && p.country != "other" {
		query = query.Where("country = ?", p.country)
	} else if p.country == "other" {
		query = query.Where("country IS NULL OR country = ''")
	}

	if p.city != "" && p.city != "other" {
		query = query.Where("city = ?", p.city)
	} else if p.city == "other" {
		query = query.Where("city IS NULL OR city = ''")
	}

	if p.gender != "" && p.gender != "other" {
		query = query.Where("gender = ?", p.gender)
	} else if p.gender == "other" {
		query = query.Where("gender IS NULL OR gender = ''")
	}

	if p.gradeLevel != "" {
		query = query.Where("grade_level = ?", p.gradeLevel)
	}
	if p.createdFrom != "" {
		query = query.Where("created_at >= ?", p.createdFrom)
	}
	if p.createdTo != "" {
		query = query.Where("created_at <= ?", p.createdTo+"T23:59:59Z")
	}

	if p.subscriptionStatus != "" {
		now := time.Now()
		switch p.subscriptionStatus {
		case "active":
			query = query.Where("active_subscription_id IS NOT NULL AND subscription_expires_at > ?", now)
		case "expired":
			query = query.Where("active_subscription_id IS NOT NULL AND subscription_expires_at <= ?", now)
		case "none":
			query = query.Where("active_subscription_id IS NULL")
		}
	}

	return query
}

func buildUserListOrderClause(sortBy, sortOrder string) string {
	allowedSorts := map[string]string{
		"name":      "name",
		"createdAt": "created_at",
		"lastLogin": "last_login",
		"totalXP":   "total_xp",
		"status":    "status",
	}
	col, ok := allowedSorts[sortBy]
	if !ok {
		col = "created_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	return col + " " + sortOrder + " NULLS LAST"
}
