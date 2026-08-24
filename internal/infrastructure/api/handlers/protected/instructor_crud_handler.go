package protected

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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
			apiresponse.Error(c, http.StatusBadRequest, "invalid instructor status")
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

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
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
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
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
		apiresponse.Error(c, http.StatusBadRequest, "bio is too long")
		return
	}

	phone, err := normalizeInstructorOptionalText(input.Phone, maxPhoneLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "phone is too long")
		return
	}

	country, err := normalizeInstructorOptionalText(input.Country, maxCountryLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "country is too long")
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
			apiresponse.Error(c, http.StatusBadRequest, "invalid instructor status")
			return
		}
		status = normalized
	}

	specialties, err := validateStringArray(input.Specialties, maxArrayItems, maxArrayItemLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid specialties")
		return
	}

	languages, err := validateStringArray(input.Languages, maxArrayItems, maxArrayItemLength)
	if err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid languages")
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

	_, err = hashPassword(randomPassword)
	if err != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to create instructor")
		return
	}

	teacher := models.User{
		Email:                 email,
		Name:                  &name,
		Username:              &username,
		Role:                  models.RoleTeacher,
		InstructorStatus:      status,
		CommissionRate:        decimal.NewFromFloat(commissionRate),
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
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
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

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"instructor": buildInstructorResponse(&user),
	})
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
