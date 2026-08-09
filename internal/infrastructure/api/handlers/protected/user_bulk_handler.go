package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func BulkCreateUsers(c *gin.Context) {
	var req struct {
		Users []struct {
			Email    string  `json:"email" binding:"required,email"`
			Name     string  `json:"name" binding:"required"`
			Username *string `json:"username"`
			Password string  `json:"password" binding:"required"`
			Role     string  `json:"role"`
		} `json:"users" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var usersToCreate []models.User
	var credentialsToCreate []models.UserCredential

	for _, userReq := range req.Users {
		role := models.RoleStudent
		if userReq.Role != "" {
			validRoles := map[string]bool{"STUDENT": true, "TEACHER": true, "MODERATOR": true, "ADMIN": true}
			if !validRoles[userReq.Role] {
				continue
			}
			role = models.UserRole(userReq.Role)
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		if err != nil {
			continue
		}

		user := models.User{
			Email:    userReq.Email,
			Name:     &userReq.Name,
			Username: userReq.Username,
			Role:     role,
		}
		usersToCreate = append(usersToCreate, user)

		// Store credential temporarily - will be assigned a user ID after creation
		credentialsToCreate = append(credentialsToCreate, models.UserCredential{
			PasswordHash: string(hashedPassword),
		})
	}

	// Batch insert users and their credentials in chunks of 100
	created := 0
	if len(usersToCreate) > 0 {
		err := db.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(&usersToCreate, 100).Error; err != nil {
				return err
			}
			for i := range credentialsToCreate {
				credentialsToCreate[i].UserID = usersToCreate[i].ID
			}
			return tx.CreateInBatches(&credentialsToCreate, 100).Error
		})
		if err != nil {
			// Fallback: try individual transactions for better error handling
			for i := range usersToCreate {
				if err := db.DB.Transaction(func(tx *gorm.DB) error {
					if err := SafeCreate(tx, &usersToCreate[i]); err != nil {
						return err
					}
					credentialsToCreate[i].UserID = usersToCreate[i].ID
					return SafeCreate(tx, &credentialsToCreate[i])
				}); err == nil {
					created++
				}
			}
		} else {
			created = len(usersToCreate)
		}
	}

	failed := len(req.Users) - created

	LogAudit(c, "BULK_CREATE_USERS", "user", "", gin.H{"created": created, "failed": failed})
	api_response.Success(c, gin.H{"created": created, "failed": failed})
}

// BulkDeleteUsers deletes multiple users
func BulkDeleteUsers(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.UserIDs) == 0 {
		api_response.Success(c, gin.H{"deleted": 0, "failed": 0})
		return
	}

	// Batch delete users in a single query
	result := db.DB.Where("id IN ?", req.UserIDs).Delete(&models.User{})
	deleted := int(result.RowsAffected)
	failed := len(req.UserIDs) - deleted

	// Invalidate caches for all deleted users
	for _, userID := range req.UserIDs {
		middleware.InvalidateRolePermsCache(userID)
		getUserRepo().InvalidateCache(userID)
	}

	LogAudit(c, "BULK_DELETE_USERS", "user", "", gin.H{"deleted": deleted, "failed": failed})
	api_response.Success(c, gin.H{"deleted": deleted, "failed": failed})
}

// BulkSuspendUsers suspends multiple users.
func BulkSuspendUsers(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"userIds" binding:"required,min=1"`
		Reason  string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	success := int64(0)
	failed := int64(0)
	var errors []gin.H
	for _, uid := range req.UserIDs {
		if err := db.DB.Model(&models.User{}).Where(idQuery, uid).Updates(map[string]interface{}{
			"status":        models.StatusSuspended,
			"status_reason": req.Reason,
			"updated_at":    time.Now(),
		}).Error; err != nil {
			failed++
			errors = append(errors, gin.H{"id": uid, "error": err.Error()})
		} else {
			success++
			middleware.InvalidateRolePermsCache(uid)
			getUserRepo().InvalidateCache(uid)
			LogAudit(c, "BULK_SUSPEND", "user", uid, gin.H{"reason": req.Reason})
		}
	}
	api_response.Success(c, gin.H{"success": success, "failed": failed, "errors": errors})
}

// BulkActivateUsers activates multiple users.
func BulkActivateUsers(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"userIds" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	success := int64(0)
	failed := int64(0)
	var errors []gin.H
	for _, uid := range req.UserIDs {
		if err := db.DB.Model(&models.User{}).Where(idQuery, uid).Updates(map[string]interface{}{
			"status":        models.StatusActive,
			"status_reason": nil,
			"updated_at":    time.Now(),
		}).Error; err != nil {
			failed++
			errors = append(errors, gin.H{"id": uid, "error": err.Error()})
		} else {
			success++
			middleware.InvalidateRolePermsCache(uid)
			getUserRepo().InvalidateCache(uid)
			LogAudit(c, "BULK_ACTIVATE", "user", uid, nil)
		}
	}
	api_response.Success(c, gin.H{"success": success, "failed": failed, "errors": errors})
}

// BulkRestoreUsers restores multiple soft-deleted users.
func BulkRestoreUsers(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"userIds" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	success := int64(0)
	failed := int64(0)
	var errors []gin.H
	for _, uid := range req.UserIDs {
		if err := db.DB.Model(&models.User{}).Unscoped().Where("id = ?", uid).Update("deleted_at", nil).Error; err != nil {
			failed++
			errors = append(errors, gin.H{"id": uid, "error": err.Error()})
		} else {
			success++
			middleware.InvalidateRolePermsCache(uid)
			getUserRepo().InvalidateCache(uid)
			LogAudit(c, "BULK_RESTORE", "user", uid, nil)
		}
	}
	api_response.Success(c, gin.H{"success": success, "failed": failed, "errors": errors})
}

// BulkAssignRole assigns a role to multiple users.
func BulkAssignRole(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"userIds" binding:"required,min=1"`
		Role    string   `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidUserRole(models.UserRole(req.Role)) {
		api_response.Error(c, http.StatusBadRequest, "invalid role")
		return
	}
	success := int64(0)
	failed := int64(0)
	var errors []gin.H
	for _, uid := range req.UserIDs {
		if err := db.DB.Model(&models.User{}).Where(idQuery, uid).Updates(map[string]interface{}{
			"role":       req.Role,
			"updated_at": time.Now(),
		}).Error; err != nil {
			failed++
			errors = append(errors, gin.H{"id": uid, "error": err.Error()})
		} else {
			success++
			middleware.InvalidateRolePermsCache(uid)
			getUserRepo().InvalidateCache(uid)
			LogAudit(c, "BULK_ASSIGN_ROLE", "user", uid, gin.H{"role": req.Role})
		}
	}
	api_response.Success(c, gin.H{"success": success, "failed": failed, "errors": errors})
}

// SendActivationLink sends an activation/verification link to the user.
func SendActivationLink(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	token := uuid.New().String()
	expires := time.Now().Add(48 * time.Hour)
	if err := db.DB.Model(&user).Updates(map[string]interface{}{
		"email_verification_token":   token,
		"email_verification_expires": expires,
		"updated_at":                 time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create activation link")
		return
	}
	LogAudit(c, "SEND_ACTIVATION_LINK", "user", user.ID, nil)
	api_response.Success(c, gin.H{"success": true, "message": "Activation link generated"})
}
