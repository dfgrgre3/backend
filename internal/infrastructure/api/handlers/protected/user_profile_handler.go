package protected

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetUserProfile returns the authenticated user's profile details.
// @Summary Get user profile
// @Description Get the detailed profile of the currently authenticated user.
// @Tags users
// @Produce json
// @Success 200 {object} authdto.UserProfileEnvelope
// @Failure 401 {object} map[string]interface{}
// @Router /api/users/profile [get]
// including whether 2FA is configured (never the backup codes themselves -
// even hashed, they have no legitimate use on the client and must not be
// exposed in an API response).
func GetUserProfile(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := db.DB.WithContext(c.Request.Context()).First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var settings models.TwoFactorSettings
	mfaEnabled := false
	if err := db.DB.WithContext(c.Request.Context()).First(&settings, userIDQuery, userId).Error; err == nil {
		mfaEnabled = settings.IsEnabled
	}

	api_response.Success(c, gin.H{
		"id":               user.ID,
		"email":            user.Email,
		"username":         user.Username,
		"name":             user.Name,
		"avatar":           user.Avatar,
		"phone":            user.Phone,
		"phoneVerified":    user.PhoneVerified,
		"emailVerified":    user.EmailVerified,
		"gradeLevel":       user.GradeLevel,
		"educationType":    user.EducationType,
		"section":          user.Section,
		"bio":              user.Bio,
		"country":          user.Country,
		"city":             user.City,
		"gender":           user.Gender,
		"school":           user.School,
		"alternativePhone": user.AlternativePhone,
		"dateOfBirth":      user.DateOfBirth,
		"studyGoal":        user.StudyGoal,
		"subjectsTaught":   user.SubjectsTaught,
		"experienceYears":  user.ExperienceYears,
		"mfaEnabled":       mfaEnabled,
	})
}

// UpdateProfile updates the authenticated user's profile details.
// @Summary Update user profile
// @Description Update one or more fields in the currently authenticated user's profile.
// @Tags users
// @Accept json
// @Produce json
// @Param request body authdto.UserProfileUpdateRequest true "Profile fields to update"
// @Success 200 {object} authdto.UserProfileUpdateEnvelope
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/users/profile [patch]
func UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req authdto.UserProfileUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}

	if err := validateProfileUpdate(req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.GradeLevel != nil {
		updates["grade_level"] = *req.GradeLevel
	}
	if req.EducationType != nil {
		updates["education_type"] = *req.EducationType
	}
	if req.Section != nil {
		updates["section"] = *req.Section
	}
	if req.Country != nil {
		updates["country"] = *req.Country
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.AlternativePhone != nil {
		updates["alternative_phone"] = *req.AlternativePhone
	}
	if req.BirthDate != nil {
		if *req.BirthDate == "" {
			updates["date_of_birth"] = nil
		} else {
			parsed, err := time.Parse("2006-01-02", *req.BirthDate)
			if err != nil {
				api_response.Error(c, http.StatusBadRequest, "Invalid birthDate format, expected YYYY-MM-DD")
				return
			}
			updates["date_of_birth"] = parsed
		}
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.City != nil {
		updates["city"] = *req.City
	}
	if req.School != nil {
		updates["school"] = *req.School
	}
	if req.StudyGoal != nil {
		updates["study_goal"] = *req.StudyGoal
	}
	if req.SubjectsTaught != nil {
		updates["subjects_taught"] = *req.SubjectsTaught
	}
	if req.ExperienceYears != nil {
		updates["experience_years"] = *req.ExperienceYears
	}

	if len(updates) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No fields to update")
		return
	}

	database := db.DB.WithContext(c.Request.Context())
	if req.Username != nil {
		var count int64
		if err := database.Model(&models.User{}).
			Where("username = ? AND id <> ?", *req.Username, userID).
			Count(&count).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to validate username")
			return
		}
		if count > 0 {
			api_response.Error(c, http.StatusConflict, "Username is already taken")
			return
		}
	}

	if err := database.Model(&models.User{}).Where(idQuery, userID).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	// Return the canonical profile after the write. This keeps API consumers
	// synchronized even when a database trigger normalizes a value.
	var updated models.User
	if err := database.First(&updated, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Profile updated but could not be reloaded")
		return
	}
	var twoFactorSettings models.TwoFactorSettings
	mfaEnabled := database.First(&twoFactorSettings, userIDQuery, userID).Error == nil && twoFactorSettings.IsEnabled
	api_response.Success(c, gin.H{
		"message": "Profile updated successfully",
		"profile": profileResponse(updated, mfaEnabled),
	})
}

var profileUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.]{3,30}$`)
var profilePhonePattern = regexp.MustCompile(`^\+?[0-9\s-]{7,20}$`)

func validateProfileUpdate(req authdto.UserProfileUpdateRequest) error {
	if req.Name != nil && (len([]rune(strings.TrimSpace(*req.Name))) < 2 || len([]rune(*req.Name)) > 80) {
		return fmt.Errorf("name must be between 2 and 80 characters")
	}
	if req.Username != nil && *req.Username != "" && !profileUsernamePattern.MatchString(*req.Username) {
		return fmt.Errorf("username must contain only English letters, numbers, underscores, or dots")
	}
	if req.Bio != nil && len([]rune(*req.Bio)) > 300 {
		return fmt.Errorf("bio must not exceed 300 characters")
	}
	if req.City != nil && len([]rune(*req.City)) > 60 {
		return fmt.Errorf("city must not exceed 60 characters")
	}
	if req.School != nil && len([]rune(*req.School)) > 120 {
		return fmt.Errorf("school must not exceed 120 characters")
	}
	if req.StudyGoal != nil && len([]rune(*req.StudyGoal)) > 200 {
		return fmt.Errorf("study goal must not exceed 200 characters")
	}
	if req.Phone != nil && *req.Phone != "" && !profilePhonePattern.MatchString(*req.Phone) {
		return fmt.Errorf("invalid phone number")
	}
	if req.AlternativePhone != nil && *req.AlternativePhone != "" && !profilePhonePattern.MatchString(*req.AlternativePhone) {
		return fmt.Errorf("invalid alternative phone number")
	}
	return nil
}

func profileResponse(user models.User, mfaEnabled bool) gin.H {
	return gin.H{
		"id": user.ID, "email": user.Email, "username": user.Username, "name": user.Name,
		"avatar": user.Avatar, "phone": user.Phone, "phoneVerified": user.PhoneVerified,
		"emailVerified": user.EmailVerified, "gradeLevel": user.GradeLevel,
		"educationType": user.EducationType, "section": user.Section, "bio": user.Bio,
		"country": user.Country, "city": user.City, "gender": user.Gender, "school": user.School,
		"alternativePhone": user.AlternativePhone, "dateOfBirth": user.DateOfBirth,
		"studyGoal": user.StudyGoal, "subjectsTaught": user.SubjectsTaught,
		"experienceYears": user.ExperienceYears, "mfaEnabled": mfaEnabled,
	}
}
