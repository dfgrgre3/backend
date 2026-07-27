package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	apiresponse "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	instructorStatusPending     = "PENDING"
	instructorStatusUnderReview = "UNDER_REVIEW"
	instructorStatusApproved    = "APPROVED"
	instructorStatusRejected    = "REJECTED"
	instructorStatusSuspended   = "SUSPENDED"

	defaultPage      = 1
	defaultLimit     = 10
	maxLimit         = 100
	maxExportLimit   = 10000
	maxBulkDeleteIDs = 100

	maxNameLength        = 150
	maxUsernameLength    = 50
	maxEmailLength       = 254
	maxBioLength         = 2000
	maxPhoneLength       = 30
	maxCountryLength     = 80
	maxArrayItems        = 30
	maxArrayItemLength   = 80
	maxExperienceYears   = 100
	maxCommissionPercent = 100
)

type instructorStatusCount struct {
	InstructorStatus string `gorm:"column:instructor_status"`
	Count            int64  `gorm:"column:count"`
}

type instructorUpdateInput struct {
	Name            *string   `json:"name"`
	Email           *string   `json:"email"`
	Username        *string   `json:"username"`
	Bio             *string   `json:"bio"`
	Status          *string   `json:"status"`
	Specialties     *[]string `json:"specialties"`
	Languages       *[]string `json:"languages"`
	CommissionRate  *float64  `json:"commissionRate"`
	Phone           *string   `json:"phone"`
	Country         *string   `json:"country"`
	ExperienceYears *int      `json:"experience"`
}

type instructorReviewInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type instructorViolationInput struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type instructorContractInput struct {
	Title string `json:"title"`
	Notes string `json:"notes"`
}

type instructorNotificationInput struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func validInstructorStatus(status string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case instructorStatusUnderReview:
		return instructorStatusUnderReview, true
	case instructorStatusApproved:
		return instructorStatusApproved, true
	case instructorStatusRejected:
		return instructorStatusRejected, true
	case instructorStatusSuspended:
		return instructorStatusSuspended, true
	case instructorStatusPending, "":
		return instructorStatusPending, true
	default:
		return "", false
	}
}

func normalizeInstructorStatus(status string) string {
	if normalized, ok := validInstructorStatus(status); ok {
		return normalized
	}
	return instructorStatusPending
}

func normalizeInstructorStatusDefault(status string) string {
	return normalizeInstructorStatus(status)
}

func getInstructorStatus(user *models.User) string {
	if user == nil {
		return instructorStatusPending
	}
	return normalizeInstructorStatusDefault(user.InstructorStatus)
}

func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultPage)))
	if page < 1 {
		page = defaultPage
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return page, limit
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(value)
}

func stringPtrToString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func truncateString(value string, max int) string {
	if max <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= max {
		return value
	}

	return string(runes[:max])
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > maxEmailLength {
		return "", errors.New("email is too long")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", errors.New("invalid email")
	}

	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

func normalizeInstructorName(name, fallbackEmail string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.Split(fallbackEmail, "@")[0]
	}
	name = truncateString(name, maxNameLength)
	if strings.TrimSpace(name) == "" {
		name = "Instructor"
	}
	return name
}

func normalizeInstructorOptionalText(value string, maxLen int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxLen {
		return "", errors.New("value is too long")
	}
	return value, nil
}

func normalizeInstructorExperienceYears(value *int) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	experienceYears := *value
	if experienceYears < 0 || experienceYears > maxExperienceYears {
		return 0, false, errors.New("invalid experience years")
	}
	return experienceYears, true, nil
}

func normalizeInstructorCommissionRate(value *float64) (float64, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	commissionRate := *value
	if commissionRate < 0 || commissionRate > maxCommissionPercent {
		return 0, false, errors.New("commission rate must be between 0 and 100")
	}
	return commissionRate, true, nil
}

func buildInstructorUpdateMap(input instructorUpdateInput, _ *models.User) (map[string]interface{}, error) {
	updates := map[string]interface{}{}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("Name cannot be empty")
		}
		if len(name) > maxNameLength {
			return nil, errors.New("Name is too long")
		}
		updates["name"] = name
	}

	if input.Email != nil {
		email, err := normalizeEmail(*input.Email)
		if err != nil {
			return nil, err
		}
		updates["email"] = email
	}

	if input.Username != nil {
		username := normalizeUsername(*input.Username)
		if username == "" {
			return nil, errors.New("username cannot be empty")
		}
		updates["username"] = username
	}

	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)
		if len(bio) > maxBioLength {
			return nil, errors.New("Bio is too long")
		}
		if bio == "" {
			updates["bio"] = nil
		} else {
			updates["bio"] = bio
		}
	}

	if input.Phone != nil {
		phone := strings.TrimSpace(*input.Phone)
		if len(phone) > maxPhoneLength {
			return nil, errors.New("Phone is too long")
		}
		if phone == "" {
			updates["phone"] = nil
		} else {
			updates["phone"] = phone
		}
	}

	if input.Country != nil {
		country := strings.TrimSpace(*input.Country)
		if len(country) > maxCountryLength {
			return nil, errors.New("Country is too long")
		}
		if country == "" {
			updates["country"] = nil
		} else {
			updates["country"] = country
		}
	}

	if input.CommissionRate != nil {
		commissionRate := *input.CommissionRate
		if commissionRate < 0 || commissionRate > maxCommissionPercent {
			return nil, errors.New("Commission rate must be between 0 and 100")
		}
		updates["commission_rate"] = commissionRate
	}

	if input.Specialties != nil {
		specialties, err := validateStringArray(*input.Specialties, maxArrayItems, maxArrayItemLength)
		if err != nil {
			return nil, errors.New("Invalid specialties")
		}
		updates["instructor_specialties"] = models.JSONStringArray(specialties)
	}

	if input.Languages != nil {
		languages, err := validateStringArray(*input.Languages, maxArrayItems, maxArrayItemLength)
		if err != nil {
			return nil, errors.New("Invalid languages")
		}
		updates["instructor_languages"] = models.JSONStringArray(languages)
	}

	if input.ExperienceYears != nil {
		experienceYears := *input.ExperienceYears
		if experienceYears < 0 || experienceYears > maxExperienceYears {
			return nil, errors.New("Invalid experience years")
		}
		updates["experience_years"] = strconv.Itoa(experienceYears)
	}

	return updates, nil
}

func normalizeUsername(username string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	username = strings.ReplaceAll(username, " ", ".")
	username = strings.Trim(username, "._-")
	username = truncateString(username, maxUsernameLength)
	username = strings.Trim(username, "._-")
	return username
}

func validateStringArray(values []string, maxItems, maxItemLength int) ([]string, error) {
	cleaned := make([]string, 0, len(values))

	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if len(item) > maxItemLength {
			return nil, errors.New("array item is too long")
		}
		cleaned = append(cleaned, item)
	}

	if len(cleaned) > maxItems {
		return nil, errors.New("too many array items")
	}

	return cleaned, nil
}

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

func experienceYearsToInt(years *string) int {
	if years == nil {
		return 0
	}

	value, err := strconv.Atoi(*years)
	if err != nil {
		return 0
	}

	return value
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

func notImplemented(c *gin.Context, message string) {
	apiresponse.Error(c, http.StatusNotImplemented, message)
}

func GetInstructors(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))

	statusParam := strings.TrimSpace(c.Query("status"))
	statusFilter := ""
	if statusParam != "" && !strings.EqualFold(statusParam, "all") {
		normalized, ok := validInstructorStatus(statusParam)
		if !ok {
			apiresponse.Error(c, http.StatusBadRequest, "Invalid instructor status")
			return
		}
		statusFilter = normalized
	}

	baseQuery := instructorsBaseQuery(database, search)

	listQuery := baseQuery.Session(&gorm.Session{})
	if statusFilter != "" {
		listQuery = listQuery.Where("instructor_status = ?", statusFilter)
	}

	var total int64
	if err := listQuery.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructors")
		return
	}

	offset := (page - 1) * limit
	var users []models.User
	if err := listQuery.Session(&gorm.Session{}).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructors")
		return
	}

	items := make([]gin.H, 0, len(users))
	for i := range users {
		items = append(items, buildInstructorResponse(&users[i]))
	}

	summary, err := instructorStatusSummary(baseQuery)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructors summary")
		return
	}

	totalPages := 1
	if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	if totalPages < 1 {
		totalPages = 1
	}

	apiresponse.Success(c, gin.H{
		"instructors": items,
		"summary":     summary,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func GetInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}

		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor")
		return
	}

	apiresponse.Success(c, gin.H{
		"instructor":  buildInstructorResponse(&user),
		"documents":   []gin.H{},
		"contracts":   []gin.H{},
		"payouts":     []gin.H{},
		"performance": []gin.H{},
		"violations":  []gin.H{},
	})
}

func CreateInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	var input struct {
		Name            string   `json:"name"`
		Email           string   `json:"email"`
		Username        string   `json:"username"`
		Bio             string   `json:"bio"`
		Status          *string  `json:"status"`
		Specialties     []string `json:"specialties"`
		Languages       []string `json:"languages"`
		CommissionRate  *float64 `json:"commissionRate"`
		Phone           string   `json:"phone"`
		Country         string   `json:"country"`
		ExperienceYears *int     `json:"experience"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Valid email is required")
		return
	}

	name := normalizeInstructorName(input.Name, email)

	usernameBase := strings.TrimSpace(input.Username)
	if usernameBase == "" {
		usernameBase = name
	}

	username, err := ensureUniqueUsername(database, usernameBase, nil)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to generate unique username")
		return
	}

	bio, err := normalizeInstructorOptionalText(input.Bio, maxBioLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Bio is too long")
		return
	}

	phone, err := normalizeInstructorOptionalText(input.Phone, maxPhoneLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Phone is too long")
		return
	}

	country, err := normalizeInstructorOptionalText(input.Country, maxCountryLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Country is too long")
		return
	}

	commissionRate, hasCommissionRate, err := normalizeInstructorCommissionRate(input.CommissionRate)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !hasCommissionRate {
		commissionRate = 0.0
	}

	status := instructorStatusPending
	if input.Status != nil {
		normalized, ok := validInstructorStatus(*input.Status)
		if !ok {
			apiresponse.Error(c, http.StatusBadRequest, "Invalid instructor status")
			return
		}
		status = normalized
	}

	specialties, err := validateStringArray(input.Specialties, maxArrayItems, maxArrayItemLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid specialties")
		return
	}

	languages, err := validateStringArray(input.Languages, maxArrayItems, maxArrayItemLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid languages")
		return
	}

	if _, _, err := normalizeInstructorExperienceYears(input.ExperienceYears); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	emailCount, err := countRecords(database, "email", email, nil)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to validate email")
		return
	}
	if emailCount > 0 {
		apiresponse.Error(c, http.StatusConflict, "Email already exists")
		return
	}

	randomPassword, err := generateRandomPassword()
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor")
		return
	}

	passwordHash, err := hashPassword(randomPassword)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor")
		return
	}

	teacher := models.User{
		Email:                 email,
		Name:                  &name,
		Username:              &username,
		PasswordHash:          passwordHash,
		Role:                  models.RoleTeacher,
		InstructorStatus:      status,
		CommissionRate:        commissionRate,
		InstructorSpecialties: models.JSONStringArray(specialties),
		InstructorLanguages:   models.JSONStringArray(languages),
		EmailVerified:         false,
	}

	if bio != "" {
		teacher.Bio = &bio
	}
	if phone != "" {
		teacher.Phone = &phone
	}
	if country != "" {
		teacher.Country = &country
	}
	if input.ExperienceYears != nil {
		years := strconv.Itoa(*input.ExperienceYears)
		teacher.ExperienceYears = &years
	}

	if err := database.Create(&teacher).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor")
		return
	}

	apiresponse.Success(c, gin.H{"instructor": buildInstructorResponse(&teacher)})
}

func UpdateInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorUpdateInput

	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Status != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Status changes must use dedicated endpoints")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}

		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor")
		return
	}

	updates, err := buildInstructorUpdateMap(input, &user)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if input.Email != nil {
		email, err := normalizeEmail(*input.Email)
		if err != nil {
			apiresponse.Error(c, http.StatusBadRequest, "Valid email is required")
			return
		}

		emailCount, err := countRecords(database, "email", email, user.ID)
		if err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to validate email")
			return
		}
		if emailCount > 0 {
			apiresponse.Error(c, http.StatusConflict, "Email already exists")
			return
		}

		updates["email"] = email
	}

	if input.Username != nil {
		username := normalizeUsername(*input.Username)
		if username == "" {
			apiresponse.Error(c, http.StatusBadRequest, "Username cannot be empty")
			return
		}

		username, err = ensureUniqueUsername(database, username, user.ID)
		if err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to generate unique username")
			return
		}

		updates["username"] = username
	}

	if len(updates) > 0 {
		if err := database.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to update instructor")
			return
		}

		if err := database.Where("id = ?", user.ID).First(&user).Error; err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to reload instructor")
			return
		}
	}

	apiresponse.Success(c, gin.H{"instructor": buildInstructorResponse(&user)})
}

func DeleteInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	result := database.Where("id = ? AND role = ?", id, models.RoleTeacher).Delete(&models.User{})
	if result.Error != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to delete instructor")
		return
	}

	if result.RowsAffected == 0 {
		apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
		return
	}

	apiresponse.Success(c, gin.H{"deleted": true})
}

func ApproveInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusApproved)
}

func RejectInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusRejected)
}

func SuspendInstructor(c *gin.Context) {
	changeInstructorStatus(c, instructorStatusSuspended)
}

func changeInstructorStatus(c *gin.Context, status string) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	normalizedStatus := normalizeInstructorStatusDefault(status)

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}

		apiresponse.Error(c, http.StatusInternalServerError, "Failed to update instructor status")
		return
	}

	if user.InstructorStatus != normalizedStatus {
		if err := database.Model(&user).Update("instructor_status", normalizedStatus).Error; err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to update instructor status")
			return
		}
	}

	apiresponse.Success(c, gin.H{"status": normalizedStatus})
}

func GetInstructorStatistics(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	baseQuery := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher)
	summary, err := instructorStatusSummary(baseQuery)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor statistics")
		return
	}

	apiresponse.Success(c, summary)
}

func GetInstructorDocuments(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor documents")
		return
	}

	documents := []gin.H{
		{"documentId": "identity", "name": "Identity Document", "status": "PENDING"},
		{"documentId": "qualification", "name": "Qualification Certificate", "status": "PENDING"},
		{"documentId": "portfolio", "name": "Teaching Portfolio", "status": "PENDING"},
	}

	apiresponse.Success(c, gin.H{"documents": documents})
}

func ReviewInstructorDocument(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	documentID := strings.TrimSpace(c.Param("documentId"))
	if id == "" || documentID == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id and document id are required")
		return
	}

	var input instructorReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to review instructor document")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "APPROVED"
	}

	logEntry := models.AuditLog{
		UserID:      &user.ID,
		EventType:   "instructor_document_review",
		Action:      "review",
		Resource:    "instructor_document",
		ResourceID:  documentID,
		Changes:     input.Notes,
		Metadata:    `{"status":"` + status + `"}`,
		CreatedAt:   time.Now(),
	}
	if err := SafeCreate(database, &logEntry); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to record document review")
		return
	}

	apiresponse.Success(c, gin.H{
		"documentId": documentID,
		"instructorId": user.ID,
		"status": status,
		"notes": input.Notes,
		"reviewedAt": logEntry.CreatedAt,
	})
}

func GetInstructorContracts(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructor contracts")
		return
	}

	contracts := []gin.H{{
		"id":           "contract-" + user.ID,
		"instructorId": user.ID,
		"title":        "Standard Instructor Contract",
		"status":       "DRAFT",
		"createdAt":    time.Now(),
	}}

	apiresponse.Success(c, gin.H{"contracts": contracts})
}

func CreateInstructorContract(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorContractInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor contract")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Standard Instructor Contract"
	}

	apiresponse.Success(c, gin.H{
		"id":           "contract-" + user.ID,
		"instructorId": user.ID,
		"title":        title,
		"notes":        input.Notes,
		"status":       "DRAFT",
		"createdAt":    time.Now(),
	})
}


func GetInstructorPerformance(c *gin.Context) {
	apiresponse.Success(c, gin.H{"performance": []gin.H{}})
}

func GetInstructorViolations(c *gin.Context) {
	apiresponse.Success(c, gin.H{"violations": []gin.H{}})
}

func CreateInstructorViolation(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorViolationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor violation")
		return
	}

	typeValue := strings.ToLower(strings.TrimSpace(input.Type))
	if typeValue == "" {
		typeValue = "policy"
	}
	severity := strings.ToLower(strings.TrimSpace(input.Severity))
	if severity == "" {
		severity = "medium"
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = "No details provided"
	}

	logEntry := models.AuditLog{
		UserID:      &user.ID,
		EventType:   "instructor_violation",
		Action:      "create",
		Resource:    "instructor_violation",
		ResourceID:  user.ID,
		Changes:     description,
		Metadata:    `{"type":"` + typeValue + `","severity":"` + severity + `"}`,
		CreatedAt:   time.Now(),
	}
	if err := SafeCreate(database, &logEntry); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to record instructor violation")
		return
	}

	apiresponse.Success(c, gin.H{
		"id":           logEntry.ID,
		"instructorId": user.ID,
		"type":         typeValue,
		"description":  description,
		"severity":     severity,
		"status":       "OPEN",
		"createdAt":    logEntry.CreatedAt,
	})
}

func ResolveInstructorViolation(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to resolve instructor violation")
		return
	}

	apiresponse.Success(c, gin.H{
		"instructorId": user.ID,
		"status":       "RESOLVED",
		"resolvedAt":   time.Now(),
	})
}

func SendInstructorNotification(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		apiresponse.Error(c, http.StatusBadRequest, "Instructor id is required")
		return
	}

	var input instructorNotificationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiresponse.Error(c, http.StatusNotFound, "Instructor not found")
			return
		}
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to send instructor notification")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Instructor Update"
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = "You have a new update from the platform."
	}

	notification := models.Notification{
		UserID:    user.ID,
		Title:     title,
		Message:   message,
		Type:      models.NotificationInfo,
		Status:    "sent",
		Channels:  models.StringArray{"in-app"},
		CreatedAt: time.Now(),
	}
	if err := SafeCreate(database, &notification); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor notification")
		return
	}

	apiresponse.Success(c, gin.H{
		"id":           notification.ID,
		"instructorId": user.ID,
		"title":        title,
		"message":      message,
		"createdAt":    notification.CreatedAt,
	})
}

func BulkSendInstructorNotifications(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	var users []models.User
	err := database.Where("role = ?", models.RoleTeacher).Find(&users).Error
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to fetch instructors")
		return
	}

	count := 0
	for _, user := range users {
		notification := models.Notification{
			UserID:    user.ID,
			Title:     "Bulk Instructor Update",
			Message:   "A bulk update has been sent to your account.",
			Type:      models.NotificationInfo,
			Status:    "sent",
			Channels:  models.StringArray{"in-app"},
			CreatedAt: time.Now(),
		}
		if err := SafeCreate(database, &notification); err == nil {
			count++
		}
	}

	apiresponse.Success(c, gin.H{"sent": count, "total": len(users)})
}

func ExportInstructors(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(maxExportLimit)))
	if limit < 1 {
		limit = 1
	}
	if limit > maxExportLimit {
		limit = maxExportLimit
	}

	search := strings.TrimSpace(c.Query("search"))

	statusParam := strings.TrimSpace(c.Query("status"))
	statusFilter := ""
	if statusParam != "" && !strings.EqualFold(statusParam, "all") {
		normalized, ok := validInstructorStatus(statusParam)
		if !ok {
			apiresponse.Error(c, http.StatusBadRequest, "Invalid instructor status")
			return
		}
		statusFilter = normalized
	}

	query := instructorsBaseQuery(database, search)
	if statusFilter != "" {
		query = query.Where("instructor_status = ?", statusFilter)
	}

	var users []models.User
	if err := query.Order("created_at DESC").Limit(limit).Find(&users).Error; err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"id",
		"name",
		"email",
		"username",
		"status",
		"commission_rate",
		"phone",
		"country",
		"created_at",
	}); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	for i := range users {
		user := users[i]

		record := []string{
			fmt.Sprintf("%v", user.ID),
			stringPtrToString(user.Name),
			user.Email,
			stringPtrToString(user.Username),
			getInstructorStatus(&user),
			strconv.FormatFloat(user.CommissionRate, 'f', -1, 64),
			stringPtrToString(user.Phone),
			stringPtrToString(user.Country),
			fmt.Sprintf("%v", user.CreatedAt),
		}

		if err := writer.Write(record); err != nil {
			apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to export instructors")
		return
	}

	filename := fmt.Sprintf("instructors-%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func BulkDeleteInstructors(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	var input struct {
		IDs []string `json:"ids"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(input.IDs) == 0 {
		apiresponse.Error(c, http.StatusBadRequest, "IDs are required")
		return
	}

	if len(input.IDs) > maxBulkDeleteIDs {
		apiresponse.Error(c, http.StatusBadRequest, "Too many IDs")
		return
	}

	result := database.Where("id IN ? AND role = ?", input.IDs, models.RoleTeacher).Delete(&models.User{})
	if result.Error != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to delete instructors")
		return
	}

	apiresponse.Success(c, gin.H{
		"deleted":      result.RowsAffected > 0,
		"deletedCount": result.RowsAffected,
	})
}