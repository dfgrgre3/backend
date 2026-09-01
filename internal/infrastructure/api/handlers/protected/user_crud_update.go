package protected

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func UpdateUser(c *gin.Context) {
	var req struct {
		UserID        string   `json:"userId"`
		ID            string   `json:"id"`
		Permissions   []string `json:"permissions"`
		Role          string   `json:"role"`
		Name          *string  `json:"name"`
		Username      *string  `json:"username"`
		Email         *string  `json:"email"`
		Phone         *string  `json:"phone"`
		Bio           *string  `json:"bio"`
		GradeLevel    *string  `json:"gradeLevel"`
		EducationType *string  `json:"educationType"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	userID := req.UserID
	if userID == "" {
		userID = req.ID
	}
	if userID == "" {
		userID = c.Param("id")
	}
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "userId is required")
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Authorization checks to prevent privilege escalation
	currentUserID, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	currentUserIDStr, _ := currentUserID.(string)
	currentUserRole, _ := c.Get("role")
	currentUserRoleStr, _ := currentUserRole.(string)

	isAdmin := currentUserRoleStr == "ADMIN" || currentUserRoleStr == "SUPER_ADMIN"
	isSelf := currentUserIDStr == user.ID

	if !isAdmin && !isSelf {
		api_response.Error(c, http.StatusForbidden, "You are not authorized to update this user")
		return
	}

	if isSelf && !isAdmin {
		if req.Role != "" || req.Permissions != nil {
			api_response.Error(c, http.StatusForbidden, "You are not authorized to update role or permissions")
			return
		}
	}

	if req.Role != "" {
		newRole := models.UserRole(req.Role)
		if !models.IsValidUserRole(newRole) {
			api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("Invalid role: %q", req.Role))
			return
		}

		// Privilege escalation guard: an actor may only grant a role at or
		// below their own hierarchy level. Without this, any ADMIN could set
		// their own (or anyone else's) role to SUPER_ADMIN via this endpoint,
		// which implicitly grants admin:bypass — full privilege escalation.
		currentUserRoleTyped := models.UserRole(currentUserRoleStr)
		actorLevel := models.GetRoleLevel(currentUserRoleTyped)
		targetLevel := models.GetRoleLevel(newRole)
		if targetLevel > actorLevel {
			api_response.Error(c, http.StatusForbidden,
				fmt.Sprintf("You cannot grant a role higher than your own: %s", req.Role))
			return
		}
		// An actor editing their own row must not be able to escalate
		// themselves, even sideways into another admin-level role with a
		// different permission set than they currently hold.
		if isSelf && newRole != currentUserRoleTyped && targetLevel >= models.RoleLevelAdmin {
			api_response.Error(c, http.StatusForbidden, "You cannot change your own role to another admin-level role")
			return
		}
	}

	if req.Permissions != nil {
		// Validate shape only. The frontend matrix legitimately offers keys that
		// are not in AllPermissions() (e.g. roles:*, taxes:*, learning_paths:*),
		// so a strict whitelist would reject valid saves. A malformed string is
		// still rejected because it can never match and only obscures intent.
		for _, p := range req.Permissions {
			if p == "" || !strings.Contains(p, ":") {
				api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("Invalid permission format: %q", p))
				return
			}
			if p == models.PermPermissionsCustom {
				// Legacy sentinel: never persist it. GetEffectivePermissions
				// strips it on read, so storing it only creates confusion.
				api_response.Error(c, http.StatusBadRequest, "permissions:custom is not a grantable permission")
				return
			}
		}

		// Privilege escalation guard: an administrator may only grant permissions
		// they themselves effectively hold. Without this, any account able to edit
		// users could award itself (or a confederate) `admin:bypass`.
		actorPermsRaw, _ := c.Get("permissions")
		actorPerms, _ := actorPermsRaw.([]string)
		for _, requested := range req.Permissions {
			granted := false
			for _, actorGrant := range actorPerms {
				if models.PermissionGrantMatches(actorGrant, requested) {
					granted = true
					break
				}
			}
			if !granted {
				api_response.Error(c, http.StatusForbidden,
					fmt.Sprintf("You cannot grant a permission you do not hold: %s", requested))
				return
			}
		}

		// An admin editing their own row must not be able to widen it.
		if isSelf {
			existing := make(map[string]struct{})
			for _, e := range user.GetEffectivePermissions() {
				existing[e] = struct{}{}
			}
			for _, requested := range req.Permissions {
				if _, held := existing[requested]; !held {
					api_response.Error(c, http.StatusForbidden,
						"You cannot grant yourself additional permissions")
					return
				}
			}
		}
	}

	type userUpdates struct {
		Role          *string                `gorm:"column:role"`
		Name          *string                `gorm:"column:name"`
		Username      *string                `gorm:"column:username"`
		Email         *string                `gorm:"column:email"`
		Phone         *string                `gorm:"column:phone"`
		Bio           *string                `gorm:"column:bio"`
		GradeLevel    *string                `gorm:"column:grade_level"`
		EducationType *string                `gorm:"column:education_type"`
		Permissions   models.JSONStringArray `gorm:"column:permissions"`
	}

	updates := userUpdates{
		Name:          req.Name,
		Username:      req.Username,
		Email:         req.Email,
		Phone:         req.Phone,
		Bio:           req.Bio,
		GradeLevel:    req.GradeLevel,
		EducationType: req.EducationType,
	}

	if req.Role != "" {
		updates.Role = &req.Role
	}
	if req.Permissions != nil {
		// An empty list is an intentional deny-all list. Do not add role defaults
		// or a sentinel marker; authorization reads this persisted list verbatim.
		updates.Permissions = models.JSONStringArray(req.Permissions)
	}

	if err := db.DB.Model(&models.User{}).Where(idQuery, user.ID).
		Updates(&updates).Error; err != nil {
		// Log the underlying error — a bare 500 here previously hid the actual
		// cause (e.g. the User.BeforeUpdate hook rejecting empty-model role
		// updates), making this endpoint impossible to debug from logs alone.
		log.Printf("[ERROR] UpdateUser: failed to persist user %s: %v", user.ID, err)
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update user", err)
		return
	}

	db.DB.First(&user, idQuery, user.ID)
	if err := getUserRepo().Update(&user); err != nil {
		log.Printf("[WARN] Failed to sync user cache after admin update: %v", err)
	}

	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "UPDATE", "user", user.ID, updates)
	api_response.Success(c, gin.H{"user": user})
}
