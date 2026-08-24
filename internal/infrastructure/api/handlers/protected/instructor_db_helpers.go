package protected

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func randomToken(byteLength int) (string, error) {
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateRandomPassword() (string, error) {
	return randomToken(18)
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func countRecords(database *gorm.DB, column string, value string, excludeID interface{}) (int64, error) {
	query := database.Model(&models.User{}).Where(column+" = ?", value)
	if excludeID != nil {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func ensureUniqueUsername(database *gorm.DB, baseUsername string, excludeID interface{}) (string, error) {
	baseUsername = normalizeUsername(baseUsername)
	if baseUsername == "" {
		baseUsername = "instructor"
	}

	baseUsername = truncateString(baseUsername, maxUsernameLength-10)
	baseUsername = strings.Trim(baseUsername, "._-")
	if baseUsername == "" {
		baseUsername = "instructor"
	}

	candidate := baseUsername
	for i := 0; i < 5; i++ {
		count, err := countRecords(database, "username", candidate, excludeID)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}

		suffix, err := randomToken(4)
		if err != nil {
			return "", err
		}

		candidate = baseUsername + "-" + strings.ToLower(suffix)
	}

	return "", errors.New("could not generate unique username")
}

func buildInstructorResponse(user *models.User) gin.H {
	if user == nil {
		return gin.H{}
	}

	name := stringPtrToString(user.Name)
	if strings.TrimSpace(name) == "" {
		name = user.Email
	}

	username := stringPtrToString(user.Username)
	if strings.TrimSpace(username) == "" {
		username = user.Email
	}

	specialties := []string{}
	if len(user.InstructorSpecialties) > 0 {
		specialties = []string(user.InstructorSpecialties)
	} else if len(user.SubjectsTaught) > 0 {
		specialties = []string(user.SubjectsTaught)
	}

	languages := []string{}
	if len(user.InstructorLanguages) > 0 {
		languages = []string(user.InstructorLanguages)
	}

	return gin.H{
		"id":             user.ID,
		"name":           name,
		"email":          user.Email,
		"username":       username,
		"avatar":         user.Avatar,
		"phone":          user.Phone,
		"country":        user.Country,
		"status":         getInstructorStatus(user),
		"role":           string(user.Role),
		"specialties":    specialties,
		"languages":      languages,
		"commissionRate": user.CommissionRate,
		"rating":         0,
		"totalStudents":  0,
		"totalCourses":   0,
		"totalRevenue":   0,
		"createdAt":      user.CreatedAt,
		"updatedAt":      user.UpdatedAt,
		"lastActive":     user.LastLogin,
		"bio":            user.Bio,
		"experience":     experienceYearsToInt(user.ExperienceYears),
		"isVerified":     user.EmailVerified,
		"documents":      []gin.H{},
	}
}

func instructorsBaseQuery(database *gorm.DB, search string) *gorm.DB {
	query := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher)

	search = strings.TrimSpace(search)
	if search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		query = query.Where(
			"(LOWER(COALESCE(name, '')) LIKE ? OR LOWER(COALESCE(email, '')) LIKE ? OR LOWER(COALESCE(username, '')) LIKE ?)",
			pattern,
			pattern,
			pattern,
		)
	}

	return query
}

func instructorStatusSummary(baseQuery *gorm.DB) (gin.H, error) {
	var counts []instructorStatusCount

	err := baseQuery.Session(&gorm.Session{}).
		Select("COALESCE(instructor_status, 'PENDING') AS instructor_status, COUNT(*) AS count").
		Group("COALESCE(instructor_status, 'PENDING')").
		Find(&counts).Error
	if err != nil {
		return nil, err
	}

	summary := gin.H{
		"total":       int64(0),
		"pending":     int64(0),
		"approved":    int64(0),
		"rejected":    int64(0),
		"suspended":   int64(0),
		"underReview": int64(0),
	}

	var total int64
	for _, item := range counts {
		total += item.Count

		switch normalizeInstructorStatusDefault(item.InstructorStatus) {
		case instructorStatusPending:
			summary["pending"] = summary["pending"].(int64) + item.Count
		case instructorStatusApproved:
			summary["approved"] = summary["approved"].(int64) + item.Count
		case instructorStatusRejected:
			summary["rejected"] = summary["rejected"].(int64) + item.Count
		case instructorStatusSuspended:
			summary["suspended"] = summary["suspended"].(int64) + item.Count
		case instructorStatusUnderReview:
			summary["underReview"] = summary["underReview"].(int64) + item.Count
		}
	}

	summary["total"] = total
	return summary, nil
}
