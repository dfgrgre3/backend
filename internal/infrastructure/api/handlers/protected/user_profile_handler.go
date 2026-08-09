package protected

import (
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// GetProfile returns current user profile
func GetProfile(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID")
		return
	}

	profilePtr, err := getUserRepo().FindByID(userIdStr)
	if err != nil {
		emailVal, _ := c.Get("user_email")
		emailStr, _ := emailVal.(string)
		if emailStr == "" {
			emailStr = userIdStr + "@external-auth.user"
		}
		if createErr := EnsureUserExists(userIdStr, emailStr); createErr != nil {
			log.Printf("[Auth] Failed to auto-create user %s: %v", userIdStr, createErr)
			api_response.Error(c, http.StatusNotFound, errUserNotFound)
			return
		}
		profilePtr, err = getUserRepo().FindByID(userIdStr)
		if err != nil {
			api_response.Error(c, http.StatusNotFound, errUserNotFound)
			return
		}
	}
	profile := *profilePtr

	// Expose effective permissions (role defaults + DB overrides) so the client matches PermissionRequired.
	profile.Permissions = models.JSONStringArray(profile.GetEffectivePermissions())

	// Add hydration status to profile so frontend knows auth context is ready
	role, _ := c.Get("role")
	perms, _ := c.Get("permissions")
	if role != nil {
		profile.Role = models.UserRole(role.(string))
	}
	if perms != nil {
		profile.Permissions = models.JSONStringArray(perms.([]string))
	}

	api_response.Success(c, gin.H{
		"user":                 &profile,
		"hydratedRole":         role,
		"hydratedPerms":        perms,
		"accessTokenExpiresAt": c.GetInt64("accessTokenExpiresAt"),
	})
}

func UpdateProfile(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Name          string `json:"name"`
		Username      string `json:"username"`
		Phone         string `json:"phone"`
		Bio           string `json:"bio"`
		GradeLevel    string `json:"gradeLevel"`
		EducationType string `json:"educationType"`
		Section       string `json:"section"`
		Country       string `json:"country"`
		City          string `json:"city"`
		Avatar        string `json:"avatar"`
		BirthDate     string `json:"birthDate"`
		Gender        string `json:"gender"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	type profileUpdates struct {
		Name          *string `gorm:"column:name"`
		Username      *string `gorm:"column:username"`
		Phone         *string `gorm:"column:phone"`
		Bio           *string `gorm:"column:bio"`
		GradeLevel    *string `gorm:"column:grade_level"`
		EducationType *string `gorm:"column:education_type"`
		Section       *string `gorm:"column:section"`
		Country       *string `gorm:"column:country"`
		Avatar        *string `gorm:"column:avatar"`
		Gender        *string `gorm:"column:gender"`
	}

	updates := profileUpdates{}
	if req.Name != "" {
		updates.Name = &req.Name
	}
	if req.Username != "" {
		updates.Username = &req.Username
	}
	if req.Phone != "" {
		updates.Phone = &req.Phone
	}
	if req.Bio != "" {
		updates.Bio = &req.Bio
	}
	if req.GradeLevel != "" {
		updates.GradeLevel = &req.GradeLevel
	}
	if req.EducationType != "" {
		updates.EducationType = &req.EducationType
	}
	if req.Section != "" {
		updates.Section = &req.Section
	}
	if req.Country != "" {
		updates.Country = &req.Country
	}
	if req.Avatar != "" {
		updates.Avatar = &req.Avatar
	}
	if req.Gender != "" {
		updates.Gender = &req.Gender
	}

	if err := db.DB.Model(&models.User{}).Where(idQuery, user.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	db.DB.First(&user, idQuery, user.ID)
	if err := getUserRepo().Update(&user); err != nil {
		log.Printf("[WARN] Failed to sync user cache after profile update: %v", err)
	}

	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)

	api_response.Success(c, gin.H{"success": true, "user": user})
}
