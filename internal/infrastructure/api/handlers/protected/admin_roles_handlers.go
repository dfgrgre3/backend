package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

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
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&roles).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch roles")
		return
	}

	items := make([]gin.H, 0, len(roles))
	for _, r := range roles {
		var userCount int64
		db.DB.Model(&models.User{}).Where("role_id = ?", r.ID).Count(&userCount)
		items = append(items, roleToGin(r, userCount))
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
	var userCount int64
	db.DB.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&userCount)
	api_response.Success(c, roleToGin(role, userCount))
}

func AdminCreateRole(c *gin.Context) {
	var req models.Role
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create role")
		return
	}
	api_response.Created(c, roleToGin(req, 0))
}

func AdminUpdateRole(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
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
	if err := db.DB.Delete(&models.Role{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete role")
		return
	}
	api_response.Success(c, gin.H{"message": "Role deleted"})
}

func roleToGin(r models.Role, userCount int64) gin.H {
	return gin.H{
		"id":          r.ID,
		"name":        r.Name,
		"description": r.Description,
		"permissions": r.Permissions,
		"usersCount":  userCount,
		"isSystem":    r.IsSystem,
		"createdAt":   r.CreatedAt,
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

	api_response.Success(c, gin.H{"message": "Users removed from role"})
}
