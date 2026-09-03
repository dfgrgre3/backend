package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// invalidateRoleMembersCache invalidates the cached role/permissions for every
// user currently assigned to roleID, so permission changes on a custom role
// (via AdminAssignPermissionsToRole/AdminRemovePermissionsFromRole/
// AdminReplaceRolePermissions) take effect immediately instead of waiting
// out the cache TTL.
func invalidateRoleMembersCache(roleID string) {
	var userIDs []string
	if err := db.DB.Model(&models.UserRoleMapping{}).
		Where("role_id = ?", roleID).
		Pluck("user_id", &userIDs).Error; err != nil {
		return
	}
	for _, uid := range userIDs {
		middleware.InvalidateRolePermsCache(uid)
	}
}

// roleUserCount returns the number of users currently assigned to a role via
// the user_roles mapping table. Users are linked to roles by assignment
// (models.UserRoleMapping), not by the User.role enum column.
func roleUserCount(roleID string) int64 {
	var count int64
	db.DB.Model(&models.UserRoleMapping{}).Where("role_id = ?", roleID).Count(&count)
	return count
}

// rolePermissionIDs returns the permission ids assigned to a role from the
// role_permissions join table.
func rolePermissionIDs(roleID string) []string {
	ids := make([]string, 0)
	db.DB.Model(&models.RolePermission{}).Where("role_id = ?", roleID).Pluck("permission_id", &ids)
	return ids
}

// ─────────────────────────────────────────────
//  Roles Management
// ─────────────────────────────────────────────

func AdminListRoles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Role{})
	if search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var roles []models.Role
	if err := query.Order("level DESC, created_at DESC").Offset(offset).Limit(limit).Find(&roles).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch roles")
		return
	}

	items := make([]gin.H, 0, len(roles))
	for _, r := range roles {
		items = append(items, roleToGin(r, roleUserCount(r.ID), rolePermissionIDs(r.ID)))
	}

	var systemCount, customCount int64
	db.DB.Model(&models.Role{}).Where("is_system = ?", true).Count(&systemCount)
	db.DB.Model(&models.Role{}).Where("is_system = ?", false).Count(&customCount)

	api_response.Success(c, gin.H{
		"roles":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalRoles": total, "systemRoles": systemCount, "customRoles": customCount},
	})
}

func AdminGetRole(c *gin.Context) {
	id := c.Param("id")
	var role models.Role
	if err := db.DB.First(&role, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Role not found")
		return
	}
	api_response.Success(c, roleToGin(role, roleUserCount(role.ID), rolePermissionIDs(role.ID)))
}

func AdminCreateRole(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description *string  `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	role := models.Role{Name: req.Name, Description: req.Description}
	if err := SafeCreate(db.DB, &role); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create role")
		return
	}

	// Persist the initial permission assignments through the same join-table
	// path used by AdminAssignPermissionsToRole.
	for _, permID := range req.Permissions {
		rp := models.RolePermission{
			RoleID:       role.ID,
			PermissionID: permID,
		}
		db.DB.Where("role_id = ? AND permission_id = ?", role.ID, permID).
			FirstOrCreate(&rp)
	}
	invalidateRoleMembersCache(role.ID)

	api_response.Created(c, roleToGin(role, 0, req.Permissions))
}

func AdminUpdateRole(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	// System roles are protected: their name and is_system flag cannot be
	// changed through the admin API. Description and level are still editable
	// so that admins can refine the label / sort order.
	var existing models.Role
	if err := db.DB.Select("id, is_system, name").Where("id = ?", id).First(&existing).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Role not found")
		return
	}
	if existing.IsSystem {
		delete(req, "name")
		delete(req, "is_system")
		delete(req, "isSystem")
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.Role{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update role")
		return
	}
	api_response.Success(c, gin.H{"message": "Role updated"})
}

func AdminDeleteRole(c *gin.Context) {
	id := c.Param("id")
	// System roles cannot be deleted — they are part of the canonical role
	// catalog (SUPER_ADMIN, ADMIN, ...) and removing them would break
	// mergeCustomRolePermissions lookups for any user assigned to them.
	var existing models.Role
	if err := db.DB.Select("is_system").Where("id = ?", id).First(&existing).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Role not found")
		return
	}
	if existing.IsSystem {
		api_response.Error(c, http.StatusForbidden, "System roles cannot be deleted")
		return
	}
	invalidateRoleMembersCache(id)
	if err := db.DB.Delete(&models.Role{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete role")
		return
	}
	// Clean up dangling assignments/grants now that the role no longer exists.
	db.DB.Where("role_id = ?", id).Delete(&models.UserRoleMapping{})
	db.DB.Where("role_id = ?", id).Delete(&models.RolePermission{})
	api_response.Success(c, gin.H{"message": "Role deleted"})
}

func roleToGin(r models.Role, userCount int64, permissionIDs []string) gin.H {
	if permissionIDs == nil {
		permissionIDs = []string{}
	}
	return gin.H{
		"id":          r.ID,
		"name":        r.Name,
		"description": r.Description,
		"permissions": permissionIDs,
		"usersCount":  userCount,
		"isSystem":    r.IsSystem,
		"level":       r.Level,
		"createdAt":   r.CreatedAt,
		"updatedAt":   r.UpdatedAt,
	}
}

// ─────────────────────────────────────────────
//  Permissions Management
// ─────────────────────────────────────────────

func AdminListPermissions(c *gin.Context) {
	module := c.Query("module")

	query := db.DB.Model(&models.Permission{})
	if module != "" {
		query = query.Where("module = ?", module)
	}

	var permissions []models.Permission
	if err := query.Order("module, name").Find(&permissions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch permissions")
		return
	}

	// Group by module
	grouped := make(map[string][]gin.H)
	for _, p := range permissions {
		grouped[p.Module] = append(grouped[p.Module], gin.H{
			"id":          p.ID,
			"name":        p.Name,
			"action":      p.Action,
			"description": p.Description,
		})
	}

	api_response.Success(c, gin.H{
		"permissions": permissions,
		"groups":      grouped,
	})
}

func AdminGetPermissionGroups(c *gin.Context) {
	var modules []string
	if err := db.DB.Model(&models.Permission{}).Distinct("module").Pluck("module", &modules).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch permission groups")
		return
	}

	groups := make([]gin.H, 0, len(modules))
	for _, module := range modules {
		var count int64
		db.DB.Model(&models.Permission{}).Where("module = ?", module).Count(&count)
		groups = append(groups, gin.H{
			"module": module,
			"count":  count,
		})
	}

	api_response.Success(c, gin.H{"groups": groups})
}

// ─────────────────────────────────────────────
//  Role Permissions Management
// ─────────────────────────────────────────────

func AdminGetRolePermissions(c *gin.Context) {
	roleID := c.Param("id")

	var rolePermissions []models.RolePermission
	if err := db.DB.Where("role_id = ?", roleID).Find(&rolePermissions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch role permissions")
		return
	}

	permissionIDs := make([]string, len(rolePermissions))
	for i, rp := range rolePermissions {
		permissionIDs[i] = rp.PermissionID
	}

	var permissions []models.Permission
	if len(permissionIDs) > 0 {
		db.DB.Where("id IN ?", permissionIDs).Find(&permissions)
	}

	api_response.Success(c, gin.H{
		"permissions": permissions,
	})
}

func AdminAssignPermissionsToRole(c *gin.Context) {
	roleID := c.Param("id")
	var req struct {
		PermissionIDs []string `json:"permissionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	for _, permID := range req.PermissionIDs {
		rp := models.RolePermission{
			RoleID:       roleID,
			PermissionID: permID,
		}
		// Use OnConflict to ignore duplicates
		db.DB.Where("role_id = ? AND permission_id = ?", roleID, permID).
			FirstOrCreate(&rp)
	}
	invalidateRoleMembersCache(roleID)

	api_response.Success(c, gin.H{"message": "Permissions assigned to role"})
}

func AdminRemovePermissionsFromRole(c *gin.Context) {
	roleID := c.Param("id")
	var req struct {
		PermissionIDs []string `json:"permissionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.DB.Where("role_id = ? AND permission_id IN ?", roleID, req.PermissionIDs).
		Delete(&models.RolePermission{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove permissions")
		return
	}
	invalidateRoleMembersCache(roleID)

	api_response.Success(c, gin.H{"message": "Permissions removed from role"})
}

func AdminReplaceRolePermissions(c *gin.Context) {
	roleID := c.Param("id")
	var req struct {
		PermissionIDs []string `json:"permissionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Delete existing permissions
	db.DB.Where("role_id = ?", roleID).Delete(&models.RolePermission{})

	// Add new permissions
	for _, permID := range req.PermissionIDs {
		rp := models.RolePermission{
			RoleID:       roleID,
			PermissionID: permID,
		}
		db.DB.Create(&rp)
	}
	invalidateRoleMembersCache(roleID)

	api_response.Success(c, gin.H{"message": "Role permissions replaced"})
}

// ─────────────────────────────────────────────
//  Role Users Management
// ─────────────────────────────────────────────

func AdminGetUsersByRole(c *gin.Context) {
	roleID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.UserRoleMapping{}).Where("role_id = ?", roleID)

	var total int64
	query.Count(&total)

	var mappings []models.UserRoleMapping
	if err := query.Order("assigned_at DESC").Offset(offset).Limit(limit).Find(&mappings).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch role users")
		return
	}

	userIDs := make([]string, len(mappings))
	for i, m := range mappings {
		userIDs[i] = m.UserID
	}

	var users []models.User
	if len(userIDs) > 0 {
		db.DB.Where("id IN ?", userIDs).Find(&users)
	}

	userMap := make(map[string]models.User)
	for _, u := range users {
		userMap[u.ID] = u
	}

	items := make([]gin.H, 0, len(mappings))
	for _, m := range mappings {
		if user, ok := userMap[m.UserID]; ok {
			items = append(items, gin.H{
				"userId":     m.UserID,
				"name":       user.Name,
				"email":      user.Email,
				"role":       user.Role,
				"status":     user.Status,
				"assignedAt": m.AssignedAt,
				"assignedBy": m.AssignedBy,
			})
		}
	}

	api_response.Success(c, gin.H{
		"users":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
	})
}

func AdminAssignUsersToRole(c *gin.Context) {
	roleID := c.Param("id")
	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	for _, userID := range req.UserIDs {
		ur := models.UserRoleMapping{
			UserID: userID,
			RoleID: roleID,
		}
		// Use OnConflict to ignore duplicates
		db.DB.Where("user_id = ? AND role_id = ?", userID, roleID).
			FirstOrCreate(&ur)
		middleware.InvalidateRolePermsCache(userID)
	}

	api_response.Success(c, gin.H{"message": "Users assigned to role"})
}

func AdminRemoveUsersFromRole(c *gin.Context) {
	roleID := c.Param("id")
	var req struct {
		UserIDs []string `json:"userIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.DB.Where("role_id = ? AND user_id IN ?", roleID, req.UserIDs).
		Delete(&models.UserRoleMapping{}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove users from role")
		return
	}
	for _, userID := range req.UserIDs {
		middleware.InvalidateRolePermsCache(userID)
	}

	api_response.Success(c, gin.H{"message": "Users removed from role"})
}
