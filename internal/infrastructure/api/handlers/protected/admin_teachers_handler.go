package protected

import (
	models "thanawy-backend/internal/domain/common"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func GetTeachers(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	var teachers []models.User
	if err := database.Where("role = ?", models.RoleTeacher).Find(&teachers).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch teachers")
		return
	}

	api_response.Success(c, teachers)
}

func GetTeachersForAdmin(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}
	var teachers []models.User
	if err := database.Where("role = ?", models.RoleTeacher).Find(&teachers).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch teachers")
		return
	}

	items := make([]gin.H, 0, len(teachers))
	for _, teacher := range teachers {
		items = append(items, gin.H{
			"id":        teacher.ID,
			"name":      firstNonEmpty(stringOrEmpty(teacher.Name), stringOrEmpty(teacher.Username), teacher.Email),
			"subjectId": "",
			"onlineUrl": nil,
			"rating":    0,
			"notes":     teacher.Bio,
			"createdAt": teacher.CreatedAt,
			"subject": gin.H{
				"name":   "",
				"nameAr": nil,
				"color":  nil,
			},
		})
	}

	api_response.Success(c, gin.H{
		"items":    items,
		"teachers": items,
	})
}

func CreateTeacher(c *gin.Context) {
	var input struct {
		Name      string  `json:"name" binding:"required"`
		SubjectID string  `json:"subjectId"`
		OnlineURL *string `json:"onlineUrl"`
		Notes     *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	teacherName := input.Name
	email := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input.Name), " ", ".")) + "@thanawy.local"

	var existingUser models.User
	if err := db.DB.Where("email = ?", email).First(&existingUser).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "Teacher with this name already exists")
		return
	}

	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	randomPassword := hex.EncodeToString(randomBytes)
	_, err := bcrypt.GenerateFromPassword([]byte(randomPassword), 12)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to generate password")
		return
	}

	teacher := models.User{
		Email:    email,
		Name:     &teacherName,
		Username: &teacherName,
		Role:     models.RoleTeacher,
		Bio:      input.Notes,
	}

	if err := SafeCreate(db.DB, &teacher); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "Teacher with this email already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create teacher")
		return
	}

	apiresponse.Created(c, gin.H{"teacher": teacher})
}

func UpdateTeacher(c *gin.Context) {
	var input struct {
		ID        string  `json:"id" binding:"required"`
		Name      string  `json:"name"`
		OnlineURL *string `json:"onlineUrl"`
		Notes     *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var teacher models.User
	if err := db.DB.Where("id = ? AND role = ?", input.ID, models.RoleTeacher).First(&teacher).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Teacher not found")
		return
	}

	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
		updates["username"] = input.Name
	}
	if input.Notes != nil {
		updates["bio"] = *input.Notes
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&models.User{}).Where(queryID, teacher.ID).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update teacher")
			return
		}
	}

	api_response.Success(c, nil)
}

func DeleteTeacher(c *gin.Context) {
	var input struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.DB.Delete(&models.User{}, "id = ? AND role = ?", input.ID, models.RoleTeacher).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete teacher")
		return
	}

	api_response.Success(c, nil)
}

func GetTeacherApplications(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 {
		limit = 10
	}
	search := strings.TrimSpace(c.Query("search"))
	status := normalizeInstructorStatus(c.Query("status"))
	if strings.EqualFold(c.Query("status"), "all") || strings.TrimSpace(c.Query("status")) == "" {
		status = ""
	}

	query := database.Where("role = ?", models.RoleTeacher)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(username) LIKE ?", strings.ToLower(searchPattern), strings.ToLower(searchPattern), strings.ToLower(searchPattern))
	}
	if status != "" {
		query = query.Where("instructor_status = ?", status)
	}

	var total int64
	if err := query.Model(&models.User{}).Count(&total).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch teacher applications")
		return
	}

	offset := (page - 1) * limit
	var users []models.User
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch teacher applications")
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":          user.ID,
			"name":        firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
			"email":       user.Email,
			"phone":       user.Phone,
			"country":     user.Country,
			"status":      getInstructorStatus(&user),
			"specialties": []string(user.InstructorSpecialties),
			"languages":   []string(user.InstructorLanguages),
			"bio":         user.Bio,
			"createdAt":   user.CreatedAt,
			"updatedAt":   user.UpdatedAt,
		})
	}

	totalPages := 1
	if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	if totalPages < 1 {
		totalPages = 1
	}

	var pendingCount int64
	var approvedCount int64
	var rejectedCount int64
	var suspendedCount int64
	var underReviewCount int64
	if err := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher).Where("instructor_status = ?", "PENDING").Count(&pendingCount).Error; err != nil {
		pendingCount = 0
	}
	if err := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher).Where("instructor_status = ?", "APPROVED").Count(&approvedCount).Error; err != nil {
		approvedCount = 0
	}
	if err := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher).Where("instructor_status = ?", "REJECTED").Count(&rejectedCount).Error; err != nil {
		rejectedCount = 0
	}
	if err := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher).Where("instructor_status = ?", "SUSPENDED").Count(&suspendedCount).Error; err != nil {
		suspendedCount = 0
	}
	if err := database.Model(&models.User{}).Where("role = ?", models.RoleTeacher).Where("instructor_status = ?", "UNDER_REVIEW").Count(&underReviewCount).Error; err != nil {
		underReviewCount = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"applications": items,
		"summary": gin.H{
			"total":       total,
			"pending":     pendingCount,
			"approved":    approvedCount,
			"rejected":    rejectedCount,
			"suspended":   suspendedCount,
			"underReview": underReviewCount,
		},
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func ReviewTeacherApplication(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	id := c.Query("id")
	approve := c.Query("approve")
	if strings.TrimSpace(id) == "" {
		api_response.Error(c, http.StatusBadRequest, "Application id is required")
		return
	}

	var user models.User
	if err := database.Where("id = ? AND role = ?", id, models.RoleTeacher).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Application not found")
		return
	}

	newStatus := "REJECTED"
	if strings.EqualFold(approve, "true") || strings.EqualFold(approve, "1") || strings.EqualFold(approve, "yes") {
		newStatus = "APPROVED"
	}

	if err := database.Model(&models.User{}).Where("id = ?", user.ID).Update("instructor_status", newStatus).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update application status")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  newStatus,
		"message": fmt.Sprintf("Application %s successfully", strings.ToLower(newStatus)),
	})
}
