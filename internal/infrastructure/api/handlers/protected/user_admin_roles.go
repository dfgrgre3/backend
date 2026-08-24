package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

// ChangeUserRole changes a user's role
func ChangeUserRole(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Role   *string `json:"role" binding:"required"`
		Reason *string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	validRoles := map[string]bool{
		"STUDENT": true, "TEACHER": true, "MODERATOR": true,
		"ADMIN": true, "SUPER_ADMIN": true, "SUPPORT": true, "PARENT": true,
	}
	if !validRoles[*req.Role] {
		api_response.Error(c, http.StatusBadRequest, "Invalid role")
		return
	}

	// Privilege escalation guard: an actor may only grant a role at or below
	// their own hierarchy level. Without this, any account with users:manage
	// (which includes MODERATOR/SUPPORT admin roles, not just ADMIN) could
	// set any user's role to SUPER_ADMIN via this endpoint. Mirrors the same
	// check already enforced in UpdateUser (user_crud_update.go).
	newRole := models.UserRole(*req.Role)
	actorRole, _ := c.Get("role")
	actorRoleStr, _ := actorRole.(string)
	actorRoleTyped := models.UserRole(actorRoleStr)
	if actorLevel, targetLevel := models.GetRoleLevel(actorRoleTyped), models.GetRoleLevel(newRole); targetLevel > actorLevel {
		api_response.Error(c, http.StatusForbidden,
			"You cannot grant a role higher than your own")
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	user.Role = newRole

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to change user role")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "CHANGE_ROLE", "user", userID, gin.H{"oldRole": user.Role, "newRole": req.Role, "reason": req.Reason})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// AssignRole assigns a role to a user.
func AssignRole(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Role   string `json:"role" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidUserRole(models.UserRole(req.Role)) {
		api_response.Error(c, http.StatusBadRequest, "invalid role")
		return
	}
	// Privilege escalation guard: mirrors the check in ChangeUserRole / UpdateUser
	// (user_crud_update.go) — an actor may only assign a role at or below their
	// own hierarchy level.
	newRole := models.UserRole(req.Role)
	actorRole, _ := c.Get("role")
	actorRoleStr, _ := actorRole.(string)
	actorRoleTyped := models.UserRole(actorRoleStr)
	if actorLevel, targetLevel := models.GetRoleLevel(actorRoleTyped), models.GetRoleLevel(newRole); targetLevel > actorLevel {
		api_response.Error(c, http.StatusForbidden,
			"You cannot assign a role higher than your own")
		return
	}
	if err := db.DB.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"role":       req.Role,
		"updated_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to assign role")
		return
	}
	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)
	LogAudit(c, "ASSIGN_ROLE", "user", userID, gin.H{"role": req.Role, "reason": req.Reason})
	api_response.Success(c, gin.H{"success": true})
}

// AddPermission adds a permission to a user.
func AddPermission(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Permission string `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	// Privilege escalation guard: an actor cannot grant a permission they do
	// not themselves hold (directly or via a wildcard grant), otherwise any
	// account with users:manage could hand out e.g. system:manage to another
	// user. Super admins bypass via PermAdminBypass, same as elsewhere.
	actorPermsRaw, _ := c.Get("permissions")
	actorPerms, _ := actorPermsRaw.([]string)
	actorHoldsGrant := false
	for _, grant := range actorPerms {
		if grant == models.PermAdminBypass || models.PermissionGrantMatches(grant, req.Permission) {
			actorHoldsGrant = true
			break
		}
	}
	if !actorHoldsGrant {
		api_response.Error(c, http.StatusForbidden,
			"You cannot grant a permission you do not hold")
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	for _, p := range user.GetEffectivePermissions() {
		if p == req.Permission {
			api_response.Success(c, gin.H{"success": true, "message": "permission already exists"})
			return
		}
	}
	user.Permissions = append(user.Permissions, req.Permission)
	if err := db.DB.Model(&user).Update("permissions", user.Permissions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add permission")
		return
	}
	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)
	LogAudit(c, "ADD_PERMISSION", "user", user.ID, gin.H{"permission": req.Permission})
	api_response.Success(c, gin.H{"success": true})
}

// RemovePermission removes a permission from a user.
func RemovePermission(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Permission string `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	updated := make(models.JSONStringArray, 0, len(user.Permissions))
	for _, p := range user.Permissions {
		if p != req.Permission {
			updated = append(updated, p)
		}
	}
	if err := db.DB.Model(&user).Update("permissions", updated).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove permission")
		return
	}
	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)
	LogAudit(c, "REMOVE_PERMISSION", "user", user.ID, gin.H{"permission": req.Permission})
	api_response.Success(c, gin.H{"success": true})
}
