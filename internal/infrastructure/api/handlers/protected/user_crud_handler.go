package protected

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUsers(c *gin.Context) {
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")
	searchType := c.Query("searchType")
	sortBy := c.DefaultQuery("sortBy", "createdAt")
	sortOrder := c.DefaultQuery("sortOrder", "desc")
	emailVerified := c.Query("emailVerified")
	twoFactorEnabled := c.Query("twoFactorEnabled")
	country := c.Query("country")
	city := c.Query("city")
	gender := c.Query("gender")
	gradeLevel := c.Query("gradeLevel")
	createdFrom := c.Query("createdFrom")
	createdTo := c.Query("createdTo")
	subscriptionStatus := c.Query("subscriptionStatus")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var users []models.User
	query := db.DB.Model(&models.User{}).Where("deleted_at IS NULL")

	// Role filter
	if role != "" {
		query = query.Where("role = ?", role)
	}

	// Status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Search filter with search type
	if search != "" {
		switch searchType {
		case "name":
			query = query.Where("name ILIKE ?", "%"+search+"%")
		case "email":
			query = query.Where("email ILIKE ?", "%"+search+"%")
		case "username":
			query = query.Where("username ILIKE ?", "%"+search+"%")
		case "phone":
			query = query.Where("phone ILIKE ?", "%"+search+"%")
		case "userId":
			query = query.Where("id = ?", search)
		default:
			query = query.Where("(email ILIKE ? OR name ILIKE ? OR username ILIKE ? OR phone ILIKE ?)", "%"+search+"%", "%"+search+"%", "%"+search+"%", "%"+search+"%")
		}
	}

	// Email verified filter
	switch emailVerified {
	case "true":
		query = query.Where("email_verified = ?", true)
	case "false":
		query = query.Where("email_verified = ?", false)
	}

	// Two factor filter
	switch twoFactorEnabled {
	case "true":
		query = query.Where("two_factor_enabled = ?", true)
	case "false":
		query = query.Where("two_factor_enabled = ?", false)
	}

	// Country filter
	if country != "" && country != "other" {
		query = query.Where("country = ?", country)
	} else if country == "other" {
		query = query.Where("country IS NULL OR country = ''")
	}

	// City filter
	if city != "" && city != "other" {
		query = query.Where("city = ?", city)
	} else if city == "other" {
		query = query.Where("city IS NULL OR city = ''")
	}

	// Gender filter
	if gender != "" && gender != "other" {
		query = query.Where("gender = ?", gender)
	} else if gender == "other" {
		query = query.Where("gender IS NULL OR gender = ''")
	}

	// Grade level filter
	if gradeLevel != "" {
		query = query.Where("grade_level = ?", gradeLevel)
	}

	// Date range filters
	if createdFrom != "" {
		query = query.Where("created_at >= ?", createdFrom)
	}
	if createdTo != "" {
		query = query.Where("created_at <= ?", createdTo+"T23:59:59Z")
	}

	// Subscription status filter
	if subscriptionStatus != "" {
		now := time.Now()
		switch subscriptionStatus {
		case "active":
			query = query.Where("active_subscription_id IS NOT NULL AND subscription_expires_at > ?", now)
		case "expired":
			query = query.Where("active_subscription_id IS NOT NULL AND subscription_expires_at <= ?", now)
		case "none":
			query = query.Where("active_subscription_id IS NULL")
		}
	}

	// Sorting
	allowedSorts := map[string]string{
		"name":      "name",
		"createdAt": "created_at",
		"lastLogin": "last_login",
		"totalXP":   "total_xp",
		"status":    "status",
	}
	sortColumn, ok := allowedSorts[sortBy]
	if !ok {
		sortColumn = "created_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	orderClause := sortColumn + " " + sortOrder + " NULLS LAST"

	var total int64
	query.Count(&total)

	if err := query.Order(orderClause).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	// Summary stats
	var totalUsers int64
	var totalAdmins int64
	var powerUsers int64
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL").Count(&totalUsers)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND role = ?", models.RoleAdmin).Count(&totalAdmins)
	db.DB.Model(&models.User{}).Where("deleted_at IS NULL AND level >= ?", 10).Count(&powerUsers)

	// Batch fetch _count data for all users in this page using one aggregated query
	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	taskMap, sessionMap, achievementMap, enrollmentMap := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	if len(userIDs) > 0 {
		rows, err := fetchUserAggregateCounts(userIDs)
		if err != nil {
			log.Printf("[GetUsers] failed to fetch aggregate user counts: %v", err)
		} else {
			taskMap, sessionMap, achievementMap, enrollmentMap = aggregateUserCountRows(rows)
		}
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":                    user.ID,
			"email":                 user.Email,
			"name":                  user.Name,
			"username":              user.Username,
			"avatar":                user.Avatar,
			"phone":                 user.Phone,
			"phoneVerified":         user.PhoneVerified,
			"twoFactorEnabled":      user.TwoFactorEnabled,
			"role":                  user.Role,
			"status":                user.Status,
			"permissions":           user.GetEffectivePermissions(),
			"emailVerified":         user.EmailVerified,
			"country":               user.Country,
			"gradeLevel":            user.GradeLevel,
			"createdAt":             user.CreatedAt,
			"updatedAt":             user.UpdatedAt,
			"lastLogin":             user.LastLogin,
			"totalXP":               user.TotalXP,
			"level":                 user.Level,
			"currentStreak":         user.CurrentStreak,
			"activeSubscriptionId":  user.ActiveSubscriptionID,
			"subscriptionExpiresAt": user.SubscriptionExpiresAt,
			"_count": gin.H{
				"tasks":              taskMap[user.ID],
				"studySessions":      sessionMap[user.ID],
				"achievements":       achievementMap[user.ID],
				"subjectEnrollments": enrollmentMap[user.ID],
			},
		})
	}

	api_response.List(c, items, api_response.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: calculateTotalPages(total, limit),
	}, gin.H{
		"users": items,
		"summary": gin.H{
			"totalUsers":  totalUsers,
			"totalAdmins": totalAdmins,
			"powerUsers":  powerUsers,
		},
	})
}

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
		api_response.Error(c, http.StatusInternalServerError, "Failed to update user")
		return
	}

	db.DB.First(&user, idQuery, user.ID)
	if err := getUserRepo().Update(&user); err != nil {
		log.Printf("[WARN] Failed to sync user cache after admin update: %v", err)
	}

	if req.Role != "" || req.Permissions != nil {
		// Only admins can change roles
	}

	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "UPDATE", "user", user.ID, updates)
	api_response.Success(c, gin.H{"user": user})
}

func GetGuestUser(c *gin.Context) {
	api_response.Success(c, gin.H{"id": "guest_" + config.Load().Environment})
}

func GetUserByID(c *gin.Context) {
	id := c.Param("id")

	user, err := getUserRepo().FindByID(id)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	api_response.Success(c, buildUserDetailsPayload(*user))
}

func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		userID = c.Query("userId")
	}
	if userID == "" {
		var input struct {
			ID     string `json:"id"`
			UserID string `json:"userId"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			log.Printf("[WARN] Failed to bind JSON input for user deletion: %v", err)
		}
		if input.UserID != "" {
			userID = input.UserID
		} else {
			userID = input.ID
		}
	}
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "userId is required")
		return
	}

	if err := db.DB.Delete(&models.User{}, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "DELETE", "user", userID, nil)
	api_response.Success(c, nil)
}

func CreateUser(c *gin.Context) {
	var input struct {
		Email       string  `json:"email" binding:"required,email"`
		Password    string  `json:"password" binding:"required,min=8"`
		Name        *string `json:"name"`
		Username    *string `json:"username"`
		Role        string  `json:"role"`
		Phone       *string `json:"phone"`
		Status      string  `json:"status"`
		FirstName   *string `json:"firstName"`
		LastName    *string `json:"lastName"`
		Country     *string `json:"country"`
		City        *string `json:"city"`
		DateOfBirth *string `json:"dateOfBirth"`
		Gender      *string `json:"gender"`
		Language    *string `json:"language"`
		Timezone    *string `json:"timezone"`
		Bio         *string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	role := models.RoleStudent
	if input.Role != "" {
		validRoles := map[string]bool{"STUDENT": true, "TEACHER": true, "MODERATOR": true, "ADMIN": true, "SUPPORT": true, "SUPER_ADMIN": true, "PARENT": true}
		if !validRoles[input.Role] {
			api_response.Error(c, http.StatusBadRequest, "Invalid role")
			return
		}
		role = models.UserRole(input.Role)
	}

	status := models.StatusActive
	if input.Status != "" {
		validStatuses := map[string]bool{"ACTIVE": true, "INACTIVE": true, "SUSPENDED": true, "BANNED": true, "PENDING_VERIFICATION": true}
		if !validStatuses[input.Status] {
			api_response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
		status = models.UserStatus(input.Status)
	}

	// Build name from firstName + lastName if provided, otherwise use input.Name
	name := input.Name
	if input.FirstName != nil || input.LastName != nil {
		firstName := ""
		lastName := ""
		if input.FirstName != nil {
			firstName = *input.FirstName
		}
		if input.LastName != nil {
			lastName = *input.LastName
		}
		fullName := strings.TrimSpace(firstName + " " + lastName)
		if fullName != "" {
			name = &fullName
		}
	}

	// Parse date of birth if provided
	var dateOfBirth *time.Time
	if input.DateOfBirth != nil && *input.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", *input.DateOfBirth)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid date of birth format. Use YYYY-MM-DD")
			return
		}
		dateOfBirth = &parsed
	}

	// Validate gender if provided
	if input.Gender != nil && *input.Gender != "" {
		validGenders := map[string]bool{"male": true, "female": true, "other": true}
		if !validGenders[*input.Gender] {
			api_response.Error(c, http.StatusBadRequest, "Invalid gender. Must be male, female, or other")
			return
		}
	}

	// Local provisioning
	user := models.User{
		Email:       input.Email,
		Name:        name,
		Username:    input.Username,
		Role:        role,
		Status:      status,
		Phone:       input.Phone,
		Country:     input.Country,
		DateOfBirth: dateOfBirth,
		Bio:         input.Bio,
	}

	var existingUser models.User
	if err := db.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "User with this email already exists")
		return
	}

	// Check username uniqueness if provided
	if input.Username != nil && *input.Username != "" {
		var existingUsername models.User
		if err := db.DB.Where("username = ?", *input.Username).First(&existingUsername).Error; err == nil {
			api_response.Error(c, http.StatusConflict, "User with this username already exists")
			return
		}
	}

	// Check phone uniqueness if provided
	if input.Phone != nil && *input.Phone != "" {
		var existingPhone models.User
		if err := db.DB.Where("phone = ?", *input.Phone).First(&existingPhone).Error; err == nil {
			api_response.Error(c, http.StatusConflict, "User with this phone number already exists")
			return
		}
	}

	credential := models.UserCredential{}
	if err := credential.SetPassword(input.Password); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := SafeCreate(tx, &user); err != nil {
			return err
		}
		credential.UserID = user.ID
		return SafeCreate(tx, &credential)
	}); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "User with this email already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	LogAudit(c, "CREATE", "user", user.ID, gin.H{"email": user.Email, "role": user.Role})
	api_response.Created(c, user)
}
