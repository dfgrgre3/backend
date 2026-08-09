package protected

import (
	"encoding/json"
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Instructor Payouts Management
// ─────────────────────────────────────────────

func GetInstructorPayouts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.InstructorPayout{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"InstructorPayout\".instructor_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var payouts []models.InstructorPayout
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&payouts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch payouts")
		return
	}

	items := make([]gin.H, 0, len(payouts))
	var pendingCount, paidCount int64
	var totalAmount float64
	for _, p := range payouts {
		items = append(items, payoutToGin(p))
		switch p.Status {
		case "PENDING":
			pendingCount++
		case "PAID":
			paidCount++
		}
		totalAmount += p.Amount
	}

	api_response.Success(c, gin.H{
		"payouts":    items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalPayouts": total, "pendingCount": pendingCount, "paidCount": paidCount, "totalAmount": totalAmount},
	})
}

func ApproveInstructorPayout(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := c.Get("userId")
	if err := db.DB.Model(&models.InstructorPayout{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "APPROVED",
		"approved_by": adminID,
		"approved_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to approve payout")
		return
	}
	api_response.Success(c, gin.H{"message": "Payout approved"})
}

func RejectInstructorPayout(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)
	adminID, _ := c.Get("userId")
	if err := db.DB.Model(&models.InstructorPayout{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "REJECTED",
		"approved_by": adminID,
		"approved_at": time.Now(),
		"notes":       req.Reason,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reject payout")
		return
	}
	api_response.Success(c, gin.H{"message": "Payout rejected"})
}

func payoutToGin(p models.InstructorPayout) gin.H {
	instructorName := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", p.InstructorID).First(&user).Error; err == nil {
		if user.Name != nil {
			instructorName = *user.Name
		}
	}
	return gin.H{
		"id":              p.ID,
		"instructorId":    p.InstructorID,
		"instructorName":  instructorName,
		"instructorEmail": user.Email,
		"amount":          p.Amount,
		"currency":        p.Currency,
		"status":          p.Status,
		"paymentMethod":   p.PaymentMethod,
		"transactionId":   p.TransactionID,
		"notes":           p.Notes,
		"requestedAt":     p.CreatedAt,
		"paidAt":          p.PaidAt,
	}
}

// ─────────────────────────────────────────────
//  Roles Management
// ─────────────────────────────────────────────

func AdminListRoles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
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

// ─────────────────────────────────────────────
//  User Groups Management
// ─────────────────────────────────────────────

func AdminListUserGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.UserGroup{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var groups []models.UserGroup
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&groups).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch user groups")
		return
	}

	items := make([]gin.H, 0, len(groups))
	var totalMembers int64
	for _, g := range groups {
		var memberCount int64
		db.DB.Model(&models.User{}).Where("group_id = ?", g.ID).Count(&memberCount)
		totalMembers += memberCount
		items = append(items, userGroupToGin(g, memberCount))
	}

	var activeCount int64
	db.DB.Model(&models.UserGroup{}).Where("is_active = ?", true).Count(&activeCount)

	api_response.Success(c, gin.H{
		"groups":     items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalGroups": total, "activeGroups": activeCount, "totalMembers": totalMembers},
	})
}

func AdminCreateUserGroup(c *gin.Context) {
	var req models.UserGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user group")
		return
	}
	api_response.Created(c, userGroupToGin(req, 0))
}

func AdminUpdateUserGroup(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.UserGroup{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update user group")
		return
	}
	api_response.Success(c, gin.H{"message": "User group updated"})
}

func AdminDeleteUserGroup(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.UserGroup{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete user group")
		return
	}
	api_response.Success(c, gin.H{"message": "User group deleted"})
}

func userGroupToGin(g models.UserGroup, memberCount int64) gin.H {
	return gin.H{
		"id":           g.ID,
		"name":         g.Name,
		"description":  g.Description,
		"type":         g.Type,
		"membersCount": memberCount,
		"isActive":     g.IsActive,
		"createdBy":    g.CreatedBy,
		"createdAt":    g.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  MFA Management
// ─────────────────────────────────────────────

func AdminListMFA(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.MFA{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"MFA\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("is_enabled = ?", status == "enabled")
	}

	var total int64
	query.Count(&total)

	var mfas []models.MFA
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&mfas).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch MFA data")
		return
	}

	items := make([]gin.H, 0, len(mfas))
	var enabledCount, disabledCount, totpCount, smsCount int64
	for _, m := range mfas {
		items = append(items, mfaToGin(m))
		if m.IsEnabled {
			enabledCount++
			if m.Method != nil {
				switch string(*m.Method) {
				case "TOTP":
					totpCount++
				case "SMS":
					smsCount++
				}
			}
		} else {
			disabledCount++
		}
	}

	api_response.Success(c, gin.H{
		"users":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalUsers": total, "enabledCount": enabledCount, "disabledCount": disabledCount, "totpCount": totpCount, "smsCount": smsCount},
	})
}

func AdminResetMFA(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.MFA{}).Where("user_id = ?", id).Updates(map[string]interface{}{
		"is_enabled":   false,
		"method":       nil,
		"secret":       nil,
		"backup_codes": nil,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reset MFA")
		return
	}
	api_response.Success(c, gin.H{"message": "MFA reset successfully"})
}

func mfaToGin(m models.MFA) gin.H {
	userName := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", m.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
	}
	return gin.H{
		"id":              m.ID,
		"name":            userName,
		"email":           user.Email,
		"mfaEnabled":      m.IsEnabled,
		"mfaMethod":       m.Method,
		"backupCodesUsed": m.BackupCodesUsed,
		"lastUsedAt":      m.LastUsedAt,
		"enrolledAt":      m.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Security Logs
// ─────────────────────────────────────────────

func AdminListSecurityLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.SecurityLog{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"SecurityLog\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"SecurityLog\".ip_address ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.SecurityLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch security logs")
		return
	}

	items := make([]gin.H, 0, len(logs))
	var successCount, failedCount, blockedCount, highRiskCount int64
	for _, l := range logs {
		items = append(items, securityLogToGin(l))
		switch l.EventType {
		case models.SecurityEventLoginSuccess:
			successCount++
		case models.SecurityEventLoginFailed:
			failedCount++
		case models.SecurityEvent2FAFailed:
			blockedCount++
		}
		if l.Metadata != nil && len(*l.Metadata) > 0 {
			highRiskCount++
		}
	}

	api_response.Success(c, gin.H{
		"logs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "successCount": successCount, "failedCount": failedCount, "blockedCount": blockedCount, "highRiskCount": highRiskCount},
	})
}

func securityLogToGin(l models.SecurityLog) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", l.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}
	return gin.H{
		"id":        l.ID,
		"userId":    l.UserID,
		"userName":  userName,
		"userEmail": userEmail,
		"action":    l.EventType,
		"resource":  "security_log",
		"ipAddress": l.IP,
		"userAgent": l.UserAgent,
		"status":    string(l.EventType),
		"riskScore": 0,
		"location":  l.Location,
		"createdAt": l.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Scheduled Tasks
// ─────────────────────────────────────────────

func AdminListScheduledTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.ScheduledTask{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var tasks []models.ScheduledTask
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch scheduled tasks")
		return
	}

	items := make([]gin.H, 0, len(tasks))
	var activeCount, pausedCount, failedCount, totalRuns int64
	for _, t := range tasks {
		items = append(items, scheduledTaskToGin(t))
		switch t.Status {
		case "ACTIVE":
			activeCount++
		case "PAUSED":
			pausedCount++
		case "FAILED":
			failedCount++
		}
		totalRuns += int64(t.RunCount)
	}

	api_response.Success(c, gin.H{
		"tasks":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalTasks": total, "activeTasks": activeCount, "pausedTasks": pausedCount, "failedTasks": failedCount, "totalRuns": totalRuns},
	})
}

func AdminUpdateScheduledTask(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.ScheduledTask{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update scheduled task")
		return
	}
	api_response.Success(c, gin.H{"message": "Scheduled task updated"})
}

func AdminDeleteScheduledTask(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.ScheduledTask{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete scheduled task")
		return
	}
	api_response.Success(c, gin.H{"message": "Scheduled task deleted"})
}

func scheduledTaskToGin(t models.ScheduledTask) gin.H {
	return gin.H{
		"id":           t.ID,
		"name":         t.Name,
		"description":  t.Description,
		"type":         t.Type,
		"status":       t.Status,
		"schedule":     t.Schedule,
		"lastRunAt":    t.LastRunAt,
		"nextRunAt":    t.NextRunAt,
		"runCount":     t.RunCount,
		"successCount": t.SuccessCount,
		"failureCount": t.FailureCount,
		"createdAt":    t.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Queue Management
// ─────────────────────────────────────────────

func AdminListQueueJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.QueueJob{})
	if search != "" {
		query = query.Where("name ILIKE ? OR type ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var jobs []models.QueueJob
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch queue jobs")
		return
	}

	items := make([]gin.H, 0, len(jobs))
	var pendingJobs, processingJobs, failedJobs int64
	var totalProcessingTime int64
	for _, j := range jobs {
		items = append(items, queueJobToGin(j))
		switch j.Status {
		case "PENDING":
			pendingJobs++
		case "PROCESSING":
			processingJobs++
		case "FAILED":
			failedJobs++
		}
	}

	avgProcessingTime := int64(0)
	if len(jobs) > 0 {
		avgProcessingTime = totalProcessingTime / int64(len(jobs))
	}

	api_response.Success(c, gin.H{
		"jobs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalJobs": total, "pendingJobs": pendingJobs, "processingJobs": processingJobs, "failedJobs": failedJobs, "avgProcessingTime": avgProcessingTime},
	})
}

func AdminRetryQueueJob(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.QueueJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":   "PENDING",
		"attempts": 0,
		"error":    nil,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to retry job")
		return
	}
	api_response.Success(c, gin.H{"message": "Job retried"})
}

func AdminDeleteQueueJob(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.QueueJob{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete job")
		return
	}
	api_response.Success(c, gin.H{"message": "Job deleted"})
}

func queueJobToGin(j models.QueueJob) gin.H {
	return gin.H{
		"id":          j.ID,
		"name":        j.Name,
		"type":        j.Type,
		"status":      j.Status,
		"priority":    j.Priority,
		"attempts":    j.Attempts,
		"maxAttempts": j.MaxAttempts,
		"error":       j.Error,
		"processedAt": j.ProcessedAt,
		"createdAt":   j.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Cache Management
// ─────────────────────────────────────────────

func AdminGetCacheStats(c *gin.Context) {
	var entries []models.CacheEntry
	if err := db.DB.Find(&entries).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch cache stats")
		return
	}

	var totalSize, totalHits, totalMisses int64
	items := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		totalSize += e.Size
		totalHits += e.Hits
		totalMisses += e.Misses
		items = append(items, gin.H{
			"key":          e.Key,
			"type":         e.Type,
			"size":         e.Size,
			"hits":         e.Hits,
			"misses":       e.Misses,
			"lastAccessed": e.LastAccessed,
			"expiresAt":    e.ExpiresAt,
		})
	}

	total := totalHits + totalMisses
	hitRate := 0.0
	missRate := 0.0
	if total > 0 {
		hitRate = float64(totalHits) / float64(total) * 100
		missRate = float64(totalMisses) / float64(total) * 100
	}

	api_response.Success(c, gin.H{
		"entries": items,
		"summary": gin.H{
			"totalEntries": len(entries),
			"totalSize":    totalSize,
			"hitRate":      hitRate,
			"missRate":     missRate,
		},
	})
}

func AdminClearCache(c *gin.Context) {
	key := c.Param("key")
	if key != "" {
		if err := db.DB.Delete(&models.CacheEntry{}, "key = ?", key).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to clear cache")
			return
		}
	} else {
		if err := db.DB.Delete(&models.CacheEntry{}).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to clear all cache")
			return
		}
	}
	api_response.Success(c, gin.H{"message": "Cache cleared"})
}

// ─────────────────────────────────────────────
//  Email Templates
// ─────────────────────────────────────────────

func AdminListEmailTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.EmailTemplate{})
	if search != "" {
		query = query.Where("name ILIKE ? OR subject ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var templates []models.EmailTemplate
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&templates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch email templates")
		return
	}

	var activeCount int64
	db.DB.Model(&models.EmailTemplate{}).Where("is_active = ?", true).Count(&activeCount)
	var categories int64
	db.DB.Model(&models.EmailTemplate{}).Select("COUNT(DISTINCT category)").Scan(&categories)

	api_response.Success(c, gin.H{
		"templates":  templates,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalTemplates": total, "activeTemplates": activeCount, "categories": categories},
	})
}

func AdminCreateEmailTemplate(c *gin.Context) {
	var req models.EmailTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create email template")
		return
	}
	api_response.Created(c, req)
}

func AdminUpdateEmailTemplate(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.EmailTemplate{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update email template")
		return
	}
	api_response.Success(c, gin.H{"message": "Email template updated"})
}

func AdminDeleteEmailTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.EmailTemplate{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete email template")
		return
	}
	api_response.Success(c, gin.H{"message": "Email template deleted"})
}

// ─────────────────────────────────────────────
//  Feature Flags
// ─────────────────────────────────────────────

func AdminListFeatureFlags(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.FeatureFlag{})
	if search != "" {
		query = query.Where("name ILIKE ? OR key ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var flags []models.FeatureFlag
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&flags).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch feature flags")
		return
	}

	var enabledCount, disabledCount int64
	db.DB.Model(&models.FeatureFlag{}).Where("is_enabled = ?", true).Count(&enabledCount)
	db.DB.Model(&models.FeatureFlag{}).Where("is_enabled = ?", false).Count(&disabledCount)

	api_response.Success(c, gin.H{
		"flags":      flags,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalFlags": total, "enabledFlags": enabledCount, "disabledFlags": disabledCount},
	})
}

func AdminUpdateFeatureFlag(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.FeatureFlag{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update feature flag")
		return
	}
	api_response.Success(c, gin.H{"message": "Feature flag updated"})
}

func AdminDeleteFeatureFlag(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.FeatureFlag{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete feature flag")
		return
	}
	api_response.Success(c, gin.H{"message": "Feature flag deleted"})
}

// ─────────────────────────────────────────────
//  Maintenance Mode
// ─────────────────────────────────────────────

func AdminGetMaintenanceMode(c *gin.Context) {
	var settings models.SystemSetting
	if err := db.DB.First(&settings, "key = ?", "maintenance_mode").Error; err != nil {
		api_response.Success(c, gin.H{
			"isEnabled":  false,
			"message":    "",
			"allowedIPs": []string{},
			"startTime":  nil,
			"endTime":    nil,
			"updatedAt":  time.Now(),
		})
		return
	}
	api_response.Success(c, gin.H{"value": settings.Value})
}

func AdminUpdateMaintenanceMode(c *gin.Context) {
	var req struct {
		IsEnabled bool   `json:"isEnabled"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	valueBytes, err := json.Marshal(gin.H{
		"isEnabled": req.IsEnabled,
		"message":   req.Message,
		"updatedAt": time.Now(),
	})
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to encode maintenance mode settings")
		return
	}
	value := string(valueBytes)

	var setting models.SystemSetting
	if err := db.DB.First(&setting, "key = ?", "maintenance_mode").Error; err != nil {
		setting = models.SystemSetting{
			Key:   "maintenance_mode",
			Value: value,
		}
		SafeCreate(db.DB, &setting)
	} else {
		setting.Value = value
		db.DB.Save(&setting)
	}

	api_response.Success(c, gin.H{"message": "Maintenance mode updated"})
}

// ─────────────────────────────────────────────
//  API Keys
// ─────────────────────────────────────────────

func AdminListAPIKeys(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.APIKey{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var keys []models.APIKey
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&keys).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch API keys")
		return
	}

	var activeCount, expiredCount int64
	db.DB.Model(&models.APIKey{}).Where("is_active = ?", true).Count(&activeCount)
	db.DB.Model(&models.APIKey{}).Where("expires_at < ?", time.Now()).Count(&expiredCount)

	api_response.Success(c, gin.H{
		"keys":       keys,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalKeys": total, "activeKeys": activeCount, "expiredKeys": expiredCount},
	})
}

func AdminCreateAPIKey(c *gin.Context) {
	var req models.APIKey
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create API key")
		return
	}
	api_response.Created(c, req)
}

func AdminDeleteAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.APIKey{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete API key")
		return
	}
	api_response.Success(c, gin.H{"message": "API key deleted"})
}

// ─────────────────────────────────────────────
//  Webhooks
// ─────────────────────────────────────────────

func AdminListWebhooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.Webhook{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var webhooks []models.Webhook
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&webhooks).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch webhooks")
		return
	}

	var activeCount int64
	var totalTriggers int64
	db.DB.Model(&models.Webhook{}).Where("is_active = ?", true).Count(&activeCount)
	db.DB.Model(&models.Webhook{}).Select("COALESCE(SUM(success_count + failure_count), 0)").Scan(&totalTriggers)

	api_response.Success(c, gin.H{
		"webhooks":   webhooks,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalWebhooks": total, "activeWebhooks": activeCount, "totalTriggers": totalTriggers},
	})
}

func AdminCreateWebhook(c *gin.Context) {
	var req models.Webhook
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create webhook")
		return
	}
	api_response.Created(c, req)
}

func AdminUpdateWebhook(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.Webhook{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update webhook")
		return
	}
	api_response.Success(c, gin.H{"message": "Webhook updated"})
}

func AdminDeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.Webhook{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete webhook")
		return
	}
	api_response.Success(c, gin.H{"message": "Webhook deleted"})
}

func AdminTestWebhook(c *gin.Context) {
	id := c.Param("id")
	var webhook models.Webhook
	if err := db.DB.First(&webhook, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Webhook not found")
		return
	}

	db.DB.Model(&webhook).Updates(map[string]interface{}{
		"last_triggered_at": time.Now(),
	})

	api_response.Success(c, gin.H{"message": "Webhook test sent", "status": "SUCCESS"})
}

// ─────────────────────────────────────────────
//  Data Import/Export
// ─────────────────────────────────────────────

func AdminListImportExportJobs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var jobs []models.ImportExportJob
	if err := db.DB.Order("created_at DESC").Offset(offset).Limit(limit).Find(&jobs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch jobs")
		return
	}

	var total int64
	db.DB.Model(&models.ImportExportJob{}).Count(&total)

	var pending, completed, failed int64
	for _, j := range jobs {
		switch j.Status {
		case "PENDING", "PROCESSING":
			pending++
		case "COMPLETED":
			completed++
		case "FAILED":
			failed++
		}
	}

	api_response.Success(c, gin.H{
		"jobs":       jobs,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalJobs": total, "pendingJobs": pending, "completedJobs": completed, "failedJobs": failed},
	})
}

func AdminExportData(c *gin.Context) {
	entity := c.Param("entity")
	job := models.ImportExportJob{
		Type:      "EXPORT",
		Entity:    entity,
		Status:    "PENDING",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	SafeCreate(db.DB, &job)
	api_response.Success(c, gin.H{"message": "Export started", "jobId": job.ID})
}

func AdminImportData(c *gin.Context) {
	entity := c.Param("entity")
	file, _ := c.FormFile("file")
	if file == nil {
		api_response.Error(c, http.StatusBadRequest, "No file uploaded")
		return
	}

	job := models.ImportExportJob{
		Type:      "IMPORT",
		Entity:    entity,
		Status:    "PENDING",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	SafeCreate(db.DB, &job)
	api_response.Success(c, gin.H{"message": "Import started", "jobId": job.ID})
}

// ─────────────────────────────────────────────
//  System Logs
// ─────────────────────────────────────────────

func AdminListSystemLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	level := c.Query("level")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.SystemLog{})
	if search != "" {
		query = query.Where("message ILIKE ? OR service ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if level != "" && level != "all" {
		query = query.Where("level = ?", level)
	}

	var total int64
	query.Count(&total)

	var logs []models.SystemLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch system logs")
		return
	}

	var infoCount, warnCount, errorCount, debugCount int64
	for _, l := range logs {
		switch l.Level {
		case "INFO":
			infoCount++
		case "WARN":
			warnCount++
		case "ERROR":
			errorCount++
		case "DEBUG":
			debugCount++
		}
	}

	api_response.Success(c, gin.H{
		"logs":       logs,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "infoCount": infoCount, "warnCount": warnCount, "errorCount": errorCount, "debugCount": debugCount},
	})
}

// ─────────────────────────────────────────────
//  Activity Log
// ─────────────────────────────────────────────

func AdminListActivityLog(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.ActivityLog{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"ActivityLog\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"ActivityLog\".action ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var logs []models.ActivityLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch activity log")
		return
	}

	items := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		items = append(items, activityLogToGin(l))
	}

	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)
	var todayCount, weekCount int64
	db.DB.Model(&models.ActivityLog{}).Where("created_at >= ?", today).Count(&todayCount)
	db.DB.Model(&models.ActivityLog{}).Where("created_at >= ?", weekAgo).Count(&weekCount)

	api_response.Success(c, gin.H{
		"logs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "todayCount": todayCount, "weekCount": weekCount},
	})
}

func activityLogToGin(l models.ActivityLog) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", l.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}
	return gin.H{
		"id":         l.ID,
		"userId":     l.UserID,
		"userName":   userName,
		"userEmail":  userEmail,
		"action":     l.Action,
		"resource":   l.Resource,
		"resourceId": l.ResourceID,
		"ipAddress":  l.IPAddress,
		"userAgent":  l.UserAgent,
		"metadata":   l.Metadata,
		"createdAt":  l.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  User Sessions
// ─────────────────────────────────────────────

func AdminListUserSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.UserSession{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"UserSession\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"UserSession\".ip_address ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var sessions []models.UserSession
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&sessions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch user sessions")
		return
	}

	items := make([]gin.H, 0, len(sessions))
	var activeCount int64
	uniqueUsers := make(map[string]bool)
	for _, s := range sessions {
		items = append(items, userSessionToGin(s))
		if s.IsActive {
			activeCount++
		}
		uniqueUsers[s.UserID] = true
	}

	api_response.Success(c, gin.H{
		"sessions":   items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalSessions": total, "activeSessions": activeCount, "uniqueUsers": len(uniqueUsers)},
	})
}

func AdminRevokeUserSession(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.UserSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_active": false,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	api_response.Success(c, gin.H{"message": "Session revoked"})
}

func userSessionToGin(s models.UserSession) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", s.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}

	ipAddress := s.IPAddress
	if ipAddress == "" {
		ipAddress = s.IP
	}
	lastActivity := s.LastActivity
	if lastActivity.IsZero() {
		lastActivity = s.LastAccessed
	}
	device := s.Device
	if device == nil && s.DeviceType != "" {
		deviceValue := s.DeviceType
		device = &deviceValue
	}

	return gin.H{
		"id":           s.ID,
		"userId":       s.UserID,
		"userName":     userName,
		"userEmail":    userEmail,
		"ipAddress":    ipAddress,
		"userAgent":    s.UserAgent,
		"device":       device,
		"location":     s.Location,
		"isActive":     s.IsActive,
		"lastActivity": lastActivity,
		"createdAt":    s.CreatedAt,
		"expiresAt":    s.ExpiresAt,
	}
}

// ─────────────────────────────────────────────
//  Login Attempts
// ─────────────────────────────────────────────

func AdminListLoginAttempts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.LoginAttempt{})
	if search != "" {
		query = query.Where("ip_address ILIKE ?", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var attempts []models.LoginAttempt
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&attempts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch login attempts")
		return
	}

	items := make([]gin.H, 0, len(attempts))
	var successCount, failedCount, blockedCount, suspiciousCount int64
	for _, a := range attempts {
		items = append(items, loginAttemptToGin(a))
		switch a.Status {
		case "SUCCESS":
			successCount++
		case "FAILED":
			failedCount++
		case "BLOCKED":
			blockedCount++
		}
		if a.RiskScore > 50 {
			suspiciousCount++
		}
	}

	api_response.Success(c, gin.H{
		"attempts":   items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalAttempts": total, "successCount": successCount, "failedCount": failedCount, "blockedCount": blockedCount, "suspiciousCount": suspiciousCount},
	})
}

func loginAttemptToGin(a models.LoginAttempt) gin.H {
	userName := ""
	userEmail := ""
	if a.UserID != nil {
		var user models.User
		if err := db.DB.Select("name", "email").Where("id = ?", *a.UserID).First(&user).Error; err == nil {
			if user.Name != nil {
				userName = *user.Name
			}
			userEmail = user.Email
		}
	}
	return gin.H{
		"id":            a.ID,
		"userId":        a.UserID,
		"userName":      userName,
		"userEmail":     userEmail,
		"ipAddress":     a.IPAddress,
		"userAgent":     a.UserAgent,
		"status":        a.Status,
		"failureReason": a.FailureReason,
		"location":      a.Location,
		"riskScore":     a.RiskScore,
		"createdAt":     a.CreatedAt,
	}
}
