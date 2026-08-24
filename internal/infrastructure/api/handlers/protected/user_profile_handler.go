package protected

import (
	"encoding/json"
	"net/http"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetUserProfile returns the authenticated user's profile details
// including recovery codes if 2FA is configured.
func GetUserProfile(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	var settings models.TwoFactorSettings
	recoveryCodesJSON := ""
	if err := db.DB.First(&settings, userIDQuery, userId).Error; err == nil {
		if len(settings.BackupCodes) > 0 {
			if codesBytes, err := json.Marshal(settings.BackupCodes); err == nil {
				recoveryCodesJSON = string(codesBytes)
			}
		}
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
		"recoveryCodes":    recoveryCodesJSON,
	})
}

// UpdateProfile updates the authenticated user's profile details.
func UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Name             *string               `json:"name"`
		Username         *string               `json:"username"`
		Bio              *string               `json:"bio"`
		GradeLevel       *string               `json:"gradeLevel"`
		EducationType    *string               `json:"educationType"`
		Section          *string               `json:"section"`
		Country          *string               `json:"country"`
		Avatar           *string               `json:"avatar"`
		Phone            *string               `json:"phone"`
		AlternativePhone *string               `json:"alternativePhone"`
		BirthDate        *string               `json:"birthDate"`
		Gender           *string               `json:"gender"`
		City             *string               `json:"city"`
		School           *string               `json:"school"`
		StudyGoal        *string               `json:"studyGoal"`
		SubjectsTaught   *models.PGStringArray `json:"subjectsTaught"`
		ExperienceYears  *string               `json:"experienceYears"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request payload: "+err.Error())
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

	if err := db.DB.Model(&models.User{}).Where(idQuery, userID).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	api_response.Success(c, gin.H{"message": "Profile updated successfully"})
}
