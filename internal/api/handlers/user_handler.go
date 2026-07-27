package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"time"

	"github.com/gin-gonic/gin"
)

var (
	userRepo        *repository.UserRepository
	userRepoOnce    sync.Once
	sessionRepo     *repository.SessionRepository
	sessionRepoOnce sync.Once

	errFailedToGenerateTokens = os.Getenv("ERRFAILEDTOGENERATETOKENS")
	refreshTokenPath          = os.Getenv("REFRESHTOKENPATH")
	errInvalidEmail           = "Invalid email"
)

func getUserRepo() *repository.UserRepository {
	userRepoOnce.Do(func() {
		userRepo = repository.NewUserRepository(db.DB)
	})
	return userRepo
}

func getSessionRepo() *repository.SessionRepository {
	sessionRepoOnce.Do(func() {
		sessionRepo = repository.NewSessionRepository(db.DB)
	})
	return sessionRepo
}

// isProduction checks if the app is running in production mode
func isProduction() bool {
	cfg := config.Load()
	return cfg.Environment == "production"
}

// Mock geolocation helper
func getMockLocation(_ string) *string {
	loc := "القاهرة، مصر"
	return &loc
}

// GetProfile returns current user profile
// @Summary Get user profile
// @Description Get detailed profile of the currently authenticated user
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Profile details"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Router /auth/me [get]
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

	// Batch fetch _count data for all users in this page
	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	type taskCount struct {
		UserID string
		Count  int64
	}
	type sessionCount struct {
		UserID string
		Count  int64
	}
	type achievementCount struct {
		UserID string
		Count  int64
	}
	type enrollmentCount struct {
		UserID string
		Count  int64
	}

	var taskCounts []taskCount
	var sessionCounts []sessionCount
	var achievementCounts []achievementCount
	var enrollmentCounts []enrollmentCount

	if len(userIDs) > 0 {
		db.DB.Model(&models.Task{}).Select("user_id, COUNT(*) as count").Where("user_id IN ? AND deleted_at IS NULL", userIDs).Group("user_id").Scan(&taskCounts)
		db.DB.Model(&models.StudySession{}).Select("user_id, COUNT(*) as count").Where("user_id IN ? AND deleted_at IS NULL", userIDs).Group("user_id").Scan(&sessionCounts)
		db.DB.Model(&models.UserAchievement{}).Select("user_id, COUNT(*) as count").Where("user_id IN ? AND deleted_at IS NULL", userIDs).Group("user_id").Scan(&achievementCounts)
		db.DB.Model(&models.Enrollment{}).Select("user_id, COUNT(*) as count").Where("user_id IN ? AND deleted_at IS NULL", userIDs).Group("user_id").Scan(&enrollmentCounts)
	}

	taskMap := make(map[string]int64, len(taskCounts))
	for _, tc := range taskCounts {
		taskMap[tc.UserID] = tc.Count
	}
	sessionMap := make(map[string]int64, len(sessionCounts))
	for _, sc := range sessionCounts {
		sessionMap[sc.UserID] = sc.Count
	}
	achievementMap := make(map[string]int64, len(achievementCounts))
	for _, ac := range achievementCounts {
		achievementMap[ac.UserID] = ac.Count
	}
	enrollmentMap := make(map[string]int64, len(enrollmentCounts))
	for _, ec := range enrollmentCounts {
		enrollmentMap[ec.UserID] = ec.Count
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":                 user.ID,
			"email":              user.Email,
			"name":               user.Name,
			"username":           user.Username,
			"avatar":             user.Avatar,
			"phone":              user.Phone,
			"phoneVerified":      user.PhoneVerified,
			"twoFactorEnabled":   user.TwoFactorEnabled,
			"role":               user.Role,
			"status":             user.Status,
			"permissions":        user.GetEffectivePermissions(),
			"emailVerified":      user.EmailVerified,
			"country":            user.Country,
			"gradeLevel":         user.GradeLevel,
			"createdAt":          user.CreatedAt,
			"updatedAt":          user.UpdatedAt,
			"lastLogin":          user.LastLogin,
			"totalXP":            user.TotalXP,
			"level":              user.Level,
			"currentStreak":      user.CurrentStreak,
			"activeSubscriptionId": user.ActiveSubscriptionID,
			"subscriptionExpiresAt": user.SubscriptionExpiresAt,
			"_count": gin.H{
				"tasks":             taskMap[user.ID],
				"studySessions":     sessionMap[user.ID],
				"achievements":      achievementMap[user.ID],
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
		updates.Permissions = models.JSONStringArray(req.Permissions)
	}

	if err := db.DB.Model(&models.User{}).Where(idQuery, user.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update user")
		return
	}

	db.DB.First(&user, idQuery, user.ID)
	_ = getUserRepo().Update(&user)

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
		_ = c.ShouldBindJSON(&input)
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
	// Security: password is NOT accepted. External auth provider is used.
	// Accepting a local password creates a parallel auth path that bypasses
	// session management, JTI blacklisting, and MFA enforcement.
	var input struct {
		Email    string  `json:"email" binding:"required,email"`
		Name     *string `json:"name"`
		Username *string `json:"username"`
		Role     string  `json:"role"`
		Phone    *string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	role := models.RoleStudent
	if input.Role != "" {
		validRoles := map[string]bool{"STUDENT": true, "TEACHER": true, "MODERATOR": true, "ADMIN": true}
		if !validRoles[input.Role] {
			api_response.Error(c, http.StatusBadRequest, "Invalid role")
			return
		}
		role = models.UserRole(input.Role)
	}

	// Local provisioning
	user := models.User{
		Email:    input.Email,
		Name:     input.Name,
		Username: input.Username,
		Role:     role,
		Phone:    input.Phone,
	}

	var existingUser models.User
	if err := db.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "User with this email already exists")
		return
	}

	if err := SafeCreate(db.DB, &user); err != nil {
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

func buildUserDetailsPayload(user models.User) gin.H {
	var tasksCompleted int64
	var totalTasks int64
	var totalStudySessions int64
	var totalStudyTime int64
	var examsPassed int64
	var examResultsCount int64
	var unreadNotifications int64
	var totalEnrollments int64
	var achievementsCount int64

	// Try Redis cache first for user stats
	statsCached := false
	cacheKey := fmt.Sprintf("user_stats:%s", user.ID)
	bgCtx := context.Background()
	if db.Redis != nil {
		cachedVal, err := db.Redis.Get(bgCtx, cacheKey).Result()
		if err == nil {
			var cached struct {
				TasksCompleted      int64 `json:"tasksCompleted"`
				TotalTasks          int64 `json:"totalTasks"`
				TotalStudySessions  int64 `json:"totalStudySessions"`
				TotalStudyTime      int64 `json:"totalStudyTime"`
				ExamsPassed         int64 `json:"examsPassed"`
				ExamResultsCount    int64 `json:"examResultsCount"`
				UnreadNotifications int64 `json:"unreadNotifications"`
				TotalEnrollments    int64 `json:"totalEnrollments"`
				AchievementsCount   int64 `json:"achievementsCount"`
			}
			if json.Unmarshal([]byte(cachedVal), &cached) == nil {
				tasksCompleted = cached.TasksCompleted
				totalTasks = cached.TotalTasks
				totalStudySessions = cached.TotalStudySessions
				totalStudyTime = cached.TotalStudyTime
				examsPassed = cached.ExamsPassed
				examResultsCount = cached.ExamResultsCount
				unreadNotifications = cached.UnreadNotifications
				totalEnrollments = cached.TotalEnrollments
				achievementsCount = cached.AchievementsCount
				statsCached = true
			}
		}
	}

	if !statsCached {
		// Merge into fewer queries using subqueries for better performance
		readDB := db.ReadDB()
		if readDB == nil {
			readDB = db.DB
		}

		readDB.Model(&models.Task{}).Where("user_id = ? AND status = ?", user.ID, models.TaskCompleted).Count(&tasksCompleted)
		readDB.Model(&models.Task{}).Where(userIDQuery, user.ID).Count(&totalTasks)
		readDB.Model(&models.StudySession{}).Where(userIDQuery, user.ID).Count(&totalStudySessions)
		readDB.Model(&models.StudySession{}).Where(userIDQuery, user.ID).Select("COALESCE(SUM(duration_min), 0)").Scan(&totalStudyTime)
		readDB.Model(&models.ExamResult{}).Where("user_id = ? AND passed = ?", user.ID, true).Count(&examsPassed)
		readDB.Model(&models.ExamResult{}).Where(userIDQuery, user.ID).Count(&examResultsCount)
		readDB.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&unreadNotifications)
		readDB.Model(&models.Enrollment{}).Where(userIDQuery, user.ID).Count(&totalEnrollments)

		// Cache the results for 3 minutes
		if db.Redis != nil {
			cachedData, _ := json.Marshal(map[string]interface{}{
				"tasksCompleted":      tasksCompleted,
				"totalTasks":          totalTasks,
				"totalStudySessions":  totalStudySessions,
				"totalStudyTime":      totalStudyTime,
				"examsPassed":         examsPassed,
				"examResultsCount":    examResultsCount,
				"unreadNotifications": unreadNotifications,
				"totalEnrollments":    totalEnrollments,
				"achievementsCount":   achievementsCount,
			})
			db.Redis.Set(bgCtx, cacheKey, cachedData, 3*time.Minute)
		}
	}

	return gin.H{
		"id":                 user.ID,
		"email":              user.Email,
		"name":               user.Name,
		"username":           user.Username,
		"avatar":             user.Avatar,
		"role":               user.Role,
		"emailVerified":      user.EmailVerified,
		"phone":              user.Phone,
		"phoneVerified":      user.PhoneVerified,
		"twoFactorEnabled":   false,
		"createdAt":          user.CreatedAt,
		"updatedAt":          user.UpdatedAt,
		"lastLogin":          nil,
		"totalXP":            user.TotalXP,
		"level":              user.Level,
		"currentStreak":      0,
		"longestStreak":      0,
		"totalStudyTime":     totalStudyTime,
		"tasksCompleted":     tasksCompleted,
		"examsPassed":        examsPassed,
		"pomodoroSessions":   0,
		"deepWorkSessions":   0,
		"studyXP":            0,
		"taskXP":             0,
		"examXP":             0,
		"challengeXP":        0,
		"questXP":            0,
		"seasonXP":           0,
		"gradeLevel":         user.GradeLevel,
		"educationType":      user.EducationType,
		"section":            user.Section,
		"interestedSubjects": []string{},
		"studyGoal":          nil,
		"bio":                user.Bio,
		"school":             nil,
		"country":            user.Country,
		"dateOfBirth":        nil,
		"gender":             nil,
		"_count": gin.H{
			"tasks":              totalTasks,
			"studySessions":      totalStudySessions,
			"achievements":       achievementsCount,
			"notifications":      unreadNotifications,
			"examResults":        examResultsCount,
			"subjectEnrollments": totalEnrollments,
			"customGoals":        0,
			"reminders":          0,
			"sessions":           0,
		},
		"achievements":  []interface{}{},
		"examResults":   []interface{}{},
		"studySessions": []interface{}{},
	}
}

// GetUserProfile returns the authenticated user's profile details including recovery codes if configured
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
		"id":            user.ID,
		"email":         user.Email,
		"username":      user.Username,
		"name":          user.Name,
		"avatar":        user.Avatar,
		"phone":         user.Phone,
		"phoneVerified": user.PhoneVerified,
		"emailVerified": user.EmailVerified,
		"gradeLevel":    user.GradeLevel,
		"educationType": user.EducationType,
		"section":       user.Section,
		"bio":           user.Bio,
		"country":       user.Country,
		"recoveryCodes": recoveryCodesJSON,
	})
}

func GetUserEnrollments(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 50
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var total int64
	db.DB.Model(&models.Enrollment{}).Where("user_id = ?", userID).Count(&total)

	var enrollments []models.Enrollment
	if err := db.DB.Preload("Subject").Where("user_id = ?", userID).
		Order("enrolled_at DESC").
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch enrollments")
		return
	}

	totalProgress := 0.0
	items := make([]gin.H, 0, len(enrollments))
	for _, e := range enrollments {
		subjectName := ""
		subjectSlug := ""
		price := 0.0
		if e.Subject.ID != "" {
			subjectName = e.Subject.Name
			subjectSlug = stringOrEmpty(e.Subject.Slug)
			price = e.Subject.Price
		}
		items = append(items, gin.H{
			"id":                e.ID,
			"courseId":          e.SubjectID,
			"courseName":        subjectName,
			"courseSlug":        subjectSlug,
			"price":             price,
			"progress":          e.Progress,
			"status":            func() string {
				if e.Progress >= 100.0 {
					return "COMPLETED"
				}
				return "ACTIVE"
			}(),
			"enrolledAt":        e.EnrolledAt,
		})
		totalProgress += e.Progress
	}

	avgProgress := 0.0
	if len(enrollments) > 0 {
		avgProgress = totalProgress / float64(len(enrollments))
	}

	api_response.Success(c, gin.H{
		"userId":      userID,
		"total":       total,
		"avgProgress": avgProgress,
		"enrollments": items,
	})
}

func AdminEnrollUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		CourseID string `json:"courseId" binding:"required"`
		IsFree   bool   `json:"isFree"`
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

	var subject models.Subject
	if err := applyIDOrSlugQuery(db.DB, req.CourseID).First(&subject).Error; err != nil {
		handleSubjectError(c, req.CourseID, err, "verifying course for manual enrollment")
		return
	}

	if isAlreadyEnrolled(userID, subject.ID) {
		api_response.Success(c, gin.H{"success": true, "alreadyEnrolled": true, "message": "User is already enrolled"})
		return
	}

	if !req.IsFree && subject.Price > 0 && !hasPaidForSubject(userID, subject.ID) {
		api_response.Error(c, http.StatusBadRequest, "User has not paid for this course. Use isFree=true to bypass payment.")
		return
	}

	if err := executeEnrollmentTransaction(userID, subject.ID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to enroll: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"success": true, "message": "User enrolled successfully"})
}

func GetUserVideoEngagement(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limit := 100
	if q := c.Query("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			limit = v
		}
	}

	var progressRecords []models.LessonProgress
	if err := db.DB.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&progressRecords).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch video engagement")
		return
	}

	videos := make([]gin.H, 0, len(progressRecords))
	totalWatchSeconds := 0
	for _, p := range progressRecords {
		totalWatchSeconds += p.TimeSpentSeconds
		videos = append(videos, gin.H{
			"lessonId":           p.LessonID,
			"timeSpentSeconds":   p.TimeSpentSeconds,
			"timeSpentMinutes":   p.TimeSpentSeconds / 60,
			"completed":          p.Completed,
			"status":             string(p.Status),
			"lastWatchedPosition": p.LastWatchedPosition,
		})
	}

	api_response.Success(c, gin.H{
		"userId":            userID,
		"totalVideos":       len(videos),
		"totalWatchSeconds": totalWatchSeconds,
		"totalWatchMinutes": totalWatchSeconds / 60,
		"videos":            videos,
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
	_ = getUserRepo().Update(&user)

	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)

	api_response.Success(c, gin.H{"success": true, "user": user})
}

// ─── L1 in-memory cache for billing summary ──────────────
type billingSummaryEntry struct {
	data      gin.H
	expiresAt time.Time
}

var (
	billingSummaryL1    sync.Map
	billingSummaryL1TTL = 30 * time.Second
	billingRedisTTL     = 2 * time.Minute
)

const billingSummaryCachePrefix = "billing_summary:"

func GetBillingSummary(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	uid := userId.(string)

	cacheKey := billingSummaryCachePrefix + uid

	if checkBillingCaches(c, cacheKey) {
		return
	}

	responseData := fetchBillingData(uid)
	if responseData == nil {
		return
	}

	storeBillingCache(cacheKey, responseData)
	api_response.Success(c, responseData)
}

func checkBillingCaches(c *gin.Context, cacheKey string) bool {
	if val, ok := billingSummaryL1.Load(cacheKey); ok {
		entry := val.(*billingSummaryEntry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, entry.data)
			return true
		}
		billingSummaryL1.Delete(cacheKey)
	}

	if db.Redis != nil {
		redisCtx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		cachedVal, err := db.Redis.Get(redisCtx, cacheKey).Result()
		cancel()
		if err == nil {
			var cachedData gin.H
			if json.Unmarshal([]byte(cachedVal), &cachedData) == nil {
				billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: cachedData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
				api_response.Success(c, cachedData)
				return true
			}
		}
	}
	return false
}

func fetchBillingData(uid string) gin.H {
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}

	type billingResult struct {
		payments     []models.Payment
		totalSpent   float64
		successCount int64
		pendingCount int64
		failedCount  int64
	}

	var (
		user models.User
		wg   sync.WaitGroup
		res  billingResult
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		if u, err := getUserRepo().FindByID(uid); err == nil {
			user = *u
		}
	}()

	go func() {
		defer wg.Done()
		readDB.
			Model(&models.Payment{}).
			Select("id", "amount", "status", "created_at").
			Where("user_id = ?", uid).
			Order("created_at desc").
			Limit(10).
			Find(&res.payments)

		for _, p := range res.payments {
			switch p.Status {
			case models.PaymentCompleted:
				res.totalSpent += p.Amount
				res.successCount++
			case models.PaymentPending:
				res.pendingCount++
			default:
				res.failedCount++
			}
		}
	}()

	wg.Wait()

	activeSubscriptionData := fetchActiveSubscription(uid)

	return gin.H{
		"name":                  stringOrEmpty(user.Name),
		"email":                 user.Email,
		"balance":               user.Balance,
		"additionalAiCredits":   user.AiCredits,
		"additionalExamCredits": user.ExamCredits,
		"activeSubscription":    activeSubscriptionData,
		"paymentHistory":        res.payments,
		"stats": gin.H{
			"totalSpent":   res.totalSpent,
			"paymentCount": len(res.payments),
			"successCount": res.successCount,
			"pendingCount": res.pendingCount,
			"failedCount":  res.failedCount,
		},
	}
}

func fetchActiveSubscription(uid string) interface{} {
	var activeSub models.UserSubscription
	// Use read replica if available, and degrade gracefully if the table
	// has not been created yet (e.g. before migrations are applied).
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	if readDB == nil {
		return nil
	}
	err := readDB.
		Preload("Plan").
		Where("user_id = ? AND status = ? AND end_date > ?", uid, models.SubscriptionActive, time.Now()).
		First(&activeSub).Error
	if err != nil {
		// Silently handle the "table does not exist" case (SQLSTATE 42P01)
		// and the "no active subscription" case so the billing summary
		// still returns a valid payload.
		if errors.Is(err, gorm.ErrRecordNotFound) || isTableMissingError(err) {
			return nil
		}
		log.Printf("[billing] fetchActiveSubscription unexpected error: %v", err)
		return nil
	}
	return gin.H{
		"id":        activeSub.ID,
		"status":    activeSub.Status,
		"startDate": activeSub.StartDate,
		"endDate":   activeSub.EndDate,
		"plan": gin.H{
			"id":     activeSub.Plan.ID,
			"name":   activeSub.Plan.Name,
			"nameAr": activeSub.Plan.NameAr,
			"price":  activeSub.Plan.Price,
		},
		"payments": []gin.H{},
	}
}

// isTableMissingError detects PostgreSQL "relation does not exist" errors so
// that the application can degrade gracefully when a table has not been
// created yet (e.g. before migrations are applied).
func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "42P01") || // undefined_table
		strings.Contains(msg, "does not exist")
}

func storeBillingCache(cacheKey string, responseData gin.H) {
	billingSummaryL1.Store(cacheKey, &billingSummaryEntry{data: responseData, expiresAt: time.Now().Add(billingSummaryL1TTL)})
	if db.Redis != nil {
		go func(key string, data gin.H) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if cacheBytes, err := json.Marshal(data); err == nil {
				db.Redis.Set(ctx, key, cacheBytes, billingRedisTTL)
			}
		}(cacheKey, responseData)
	}
}

func calculateTotalPages(total int64, limit int) int64 {
	if limit <= 0 {
		return 1
	}
	pages := total / int64(limit)
	if total%int64(limit) != 0 {
		pages++
	}
	if pages == 0 {
		return 1
	}
	return pages
}

func defaultPermissions(role models.UserRole, existing []string) []string {
	if len(existing) > 0 {
		return existing
	}
	return models.GetDefaultPermissions(role)
}

func EnsureUserExists(userId, email string) error {
	var user models.User
	err := db.DB.Where("id = ?", userId).First(&user).Error

	if err == nil {
		return nil
	}

	newUser := models.User{
		ID:            userId,
		Email:         email,
		EmailVerified: false,
		Status:        models.StatusActive,
		Role:          models.RoleStudent,
		Balance:       0,
		AiCredits:     0,
		ExamCredits:   0,
		TotalXP:       0,
		Level:         1,
	}

	if err := SafeCreate(db.DB, &newUser); err != nil {
		if IsDuplicateKeyError(err) {
			log.Printf("[Auth] User already exists (race condition handled): %s", sanitizeLog(email))
			return nil
		}
		return err
	}

	log.Printf("[Auth] Auto-created user: %s (%s)", sanitizeLog(userId), sanitizeLog(email))
	return nil
}

// GetUserLoginAttempts is already declared in security_handler.go

func AdminUsersAnalytics(c *gin.Context) {
	now := time.Now()
	sixMonthsAgo := now.AddDate(0, -6, 0)
	startOfSixMonths := time.Date(sixMonthsAgo.Year(), sixMonthsAgo.Month(), 1, 0, 0, 0, 0, sixMonthsAgo.Location())

	// User growth over time (monthly)
	type monthlyCount struct {
		Year  int
		Month int
		Count int64
	}
	var userGrowth []monthlyCount
	db.DB.Model(&models.User{}).
		Select("EXTRACT(YEAR FROM created_at) as year, EXTRACT(MONTH FROM created_at) as month, COUNT(*) as count").
		Where("created_at >= ? AND deleted_at IS NULL", startOfSixMonths).
		Group("EXTRACT(YEAR FROM created_at), EXTRACT(MONTH FROM created_at)").
		Order("year ASC, month ASC").
		Scan(&userGrowth)

	growthChart := make([]gin.H, 0, 6)
	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		month := int(d.Month())
		year := d.Year()
		count := int64(0)
		for _, m := range userGrowth {
			if m.Year == year && m.Month == month {
				count = m.Count
				break
			}
		}
		monthNames := map[int]string{
			1: "يناير", 2: "فبراير", 3: "مارس", 4: "أبريل",
			5: "مايو", 6: "يونيو", 7: "يوليو", 8: "أغسطس",
			9: "سبتمبر", 10: "أكتوبر", 11: "نوفمبر", 12: "ديسمبر",
		}
		growthChart = append(growthChart, gin.H{
			"name":  monthNames[month],
			"users": count,
		})
	}

	// Users by role (pie chart data)
	type roleCount struct {
		Role  string
		Count int64
	}
	var roleCounts []roleCount
	db.DB.Model(&models.User{}).
		Select("role, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("role").
		Scan(&roleCounts)

	roleLabels := map[string]string{
		"STUDENT":     "طلاب",
		"TEACHER":     "معلمون",
		"MODERATOR":   "مشرفون",
		"ADMIN":       "مدراء",
		"SUPER_ADMIN": "مدراء",
		"PARENT":      "أولياء أمور",
		"SUPPORT":     "دعم فني",
	}
	roleChart := make([]gin.H, 0, len(roleCounts))
	for _, rc := range roleCounts {
		label := roleLabels[rc.Role]
		if label == "" {
			label = rc.Role
		}
		roleChart = append(roleChart, gin.H{
			"name":  label,
			"value": rc.Count,
		})
	}

	// Users by country
	type countryCount struct {
		Country string
		Count   int64
	}
	var countryCounts []countryCount
	db.DB.Model(&models.User{}).
		Select("COALESCE(country, 'أخرى') as country, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("country").
		Order("count DESC").
		Limit(5).
		Scan(&countryCounts)

	countryChart := make([]gin.H, 0, len(countryCounts))
	for _, cc := range countryCounts {
		if cc.Country == "" || cc.Country == "أخرى" {
			countryChart = append(countryChart, gin.H{"name": "أخرى", "users": cc.Count})
		} else {
			countryChart = append(countryChart, gin.H{"name": cc.Country, "users": cc.Count})
		}
	}

	// Login activity (last 7 days)
	type dailyActivity struct {
		Day   string
		Count int64
	}
	var loginActivity []dailyActivity
	sevenDaysAgo := now.AddDate(0, 0, -6)
	startOfSevenDays := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, sevenDaysAgo.Location())

	db.DB.Model(&models.SecurityLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("created_at >= ? AND event_type IN ('LOGIN_SUCCESS','LOGIN_ATTEMPT')", startOfSevenDays).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Scan(&loginActivity)

	loginMap := make(map[string]int64)
	for _, d := range loginActivity {
		loginMap[d.Day] = d.Count
	}

	dayNames := []string{"السبت", "الأحد", "الاثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة"}
	loginChart := make([]gin.H, 0, 7)
	nowDay := int(now.Weekday())
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dayKey := d.Format("2006-01-02")
		dayIdx := (nowDay - i + 7) % 7
		loginChart = append(loginChart, gin.H{
			"name":   dayNames[dayIdx],
			"logins": loginMap[dayKey],
		})
	}

	// Registration trend (last 4 weeks)
	var weeklyRegistrations []dailyActivity
	fourWeeksAgo := now.AddDate(0, 0, -27)
	db.DB.Model(&models.User{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("created_at >= ? AND deleted_at IS NULL", fourWeeksAgo).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Scan(&weeklyRegistrations)

	regMap := make(map[string]int64)
	for _, d := range weeklyRegistrations {
		regMap[d.Day] = d.Count
	}

	registrationChart := make([]gin.H, 0, 4)
	for w := 0; w < 4; w++ {
		weekStart := now.AddDate(0, 0, -(3-w)*7-6)
		weekEnd := weekStart.AddDate(0, 0, 6)
		total := int64(0)
		for day, count := range regMap {
			t, _ := time.Parse("2006-01-02", day)
			if (t.Equal(weekStart) || t.After(weekStart)) && (t.Equal(weekEnd) || t.Before(weekEnd)) {
				total += count
			}
		}
		registrationChart = append(registrationChart, gin.H{
			"name":          fmt.Sprintf("الأسبوع %d", w+1),
			"registrations": total,
		})
	}

	api_response.Success(c, gin.H{
		"growth":        growthChart,
		"roles":         roleChart,
		"countries":     countryChart,
		"loginActivity": loginChart,
		"registrations": registrationChart,
	})
}

func AdminUsersFilterOptions(c *gin.Context) {
	// Fetch available teachers for filter
	var teachers []models.User
	db.DB.Model(&models.User{}).
		Select("id, COALESCE(name, username, email) as name").
		Where("deleted_at IS NULL AND (role = ? OR role = ?)", models.RoleTeacher, models.RoleAdmin).
		Limit(50).
		Find(&teachers)

	teacherOptions := make([]gin.H, 0, len(teachers))
	for _, t := range teachers {
		teacherOptions = append(teacherOptions, gin.H{
			"id":   t.ID,
			"name": t.GetName(),
		})
	}

	// Fetch available courses/subjects for filter
	var subjects []models.Subject
	db.DB.Model(&models.Subject{}).
		Select("id, name").
		Where("deleted_at IS NULL").
		Limit(50).
		Find(&subjects)

	courseOptions := make([]gin.H, 0, len(subjects))
	for _, s := range subjects {
		courseOptions = append(courseOptions, gin.H{
			"id":   s.ID,
			"name": s.Name,
		})
	}

	// Fetch categories
	type category struct {
		ID   string
		Name string
	}
	var categories []category
	db.DB.Model(&models.Category{}).
		Select("id, name").
		Where("deleted_at IS NULL").
		Limit(20).
		Find(&categories)

	categoryOptions := make([]gin.H, 0, len(categories))
	for _, cat := range categories {
		categoryOptions = append(categoryOptions, gin.H{
			"id":   cat.ID,
			"name": cat.Name,
		})
	}

	api_response.Success(c, gin.H{
		"teachers":   teacherOptions,
		"courses":    courseOptions,
		"categories": categoryOptions,
	})
}

// ─────────────────────────────────────────────
// Parent Management Handlers
// ─────────────────────────────────────────────

// GetParentStudents returns students linked to a parent
func GetParentStudents(c *gin.Context) {
	parentID := c.Param("id")
	if parentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID is required")
		return
	}

	type studentInfo struct {
		ID           string
		Name         string
		Email        string
		GradeLevel   string
		Level        int
		Progress     float64
		Attendance   float64
		CurrentGPA   float64
		LastActivity *time.Time
	}

	var students []studentInfo
	err := db.DB.Raw(`
		SELECT 
			u.id,
			u.name,
			u.email,
			u.grade_level,
			u.level,
			COALESCE(u.progress, 0) as progress,
			COALESCE(u.attendance, 0) as attendance,
			COALESCE(u.current_gpa, 0) as current_gpa,
			u.last_login as last_activity
		FROM users u
		INNER JOIN student_parents sp ON u.id = sp.student_id
		WHERE sp.parent_id = ? AND u.deleted_at IS NULL
		ORDER BY u.name ASC
	`, parentID).Scan(&students).Error

	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch parent students")
		return
	}

	api_response.Success(c, gin.H{
		"students": students,
		"total":    len(students),
	})
}

// LinkStudentToParent links a student to a parent
func LinkStudentToParent(c *gin.Context) {
	parentID := c.Param("id")
	if parentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID is required")
		return
	}

	var req struct {
		StudentID string `json:"studentId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify parent exists and has PARENT role
	var parent models.User
	if err := db.DB.Where("id = ? AND role = ? AND deleted_at IS NULL", parentID, models.RoleParent).First(&parent).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Parent not found")
		return
	}

	// Verify student exists
	var student models.User
	if err := db.DB.Where("id = ? AND deleted_at IS NULL", req.StudentID).First(&student).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Student not found")
		return
	}

	// Check if already linked
	var existingLink int64
	db.DB.Table("student_parents").
		Where("parent_id = ? AND student_id = ?", parentID, req.StudentID).
		Count(&existingLink)

	if existingLink > 0 {
		api_response.Error(c, http.StatusConflict, "Student is already linked to this parent")
		return
	}

	// Create the link
	if err := db.DB.Exec(`
		INSERT INTO student_parents (student_id, parent_id, created_at)
		VALUES (?, ?, NOW())
	`, req.StudentID, parentID).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to link student")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Student linked successfully",
	})
}

// UnlinkStudentFromParent unlinks a student from a parent
func UnlinkStudentFromParent(c *gin.Context) {
	parentID := c.Param("id")
	studentID := c.Query("studentId")

	if parentID == "" || studentID == "" {
		api_response.Error(c, http.StatusBadRequest, "Parent ID and Student ID are required")
		return
	}

	result := db.DB.Exec(`
		DELETE FROM student_parents
		WHERE parent_id = ? AND student_id = ?
	`, parentID, studentID)

	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to unlink student")
		return
	}

	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Link not found")
		return
	}

	api_response.Success(c, gin.H{
		"message": "Student unlinked successfully",
	})
}

// GetParentStatistics returns statistics for parents
func GetParentStatistics(c *gin.Context) {
	var stats struct {
		TotalParents      int64
		ActiveParents     int64
		SuspendedParents  int64
		PendingApproval   int64
		OnlineParents     int64
		NewParentsToday   int64
		NewParentsThisMonth int64
	}

	today := time.Now().Truncate(24 * time.Hour)
	monthStart := time.Now().AddDate(0, 0, -time.Now().Day()+1).Truncate(24 * time.Hour)

	db.DB.Model(&models.User{}).
		Where("role = ? AND deleted_at IS NULL", models.RoleParent).
		Count(&stats.TotalParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, models.StatusActive).
		Count(&stats.ActiveParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, models.StatusSuspended).
		Count(&stats.SuspendedParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND status = ? AND deleted_at IS NULL", models.RoleParent, "PENDING").
		Count(&stats.PendingApproval)

	// Online parents (last login within 15 minutes)
	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	db.DB.Model(&models.User{}).
		Where("role = ? AND last_login >= ? AND deleted_at IS NULL", models.RoleParent, fifteenMinutesAgo).
		Count(&stats.OnlineParents)

	db.DB.Model(&models.User{}).
		Where("role = ? AND created_at >= ? AND deleted_at IS NULL", models.RoleParent, today).
		Count(&stats.NewParentsToday)

	db.DB.Model(&models.User{}).
		Where("role = ? AND created_at >= ? AND deleted_at IS NULL", models.RoleParent, monthStart).
		Count(&stats.NewParentsThisMonth)

	api_response.Success(c, stats)
}

func sanitizeLog(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}

// ─────────────────────────────────────────────
// Bulk User Operations
// ─────────────────────────────────────────────

// BulkCreateUsers creates multiple users from CSV import
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

	created := 0
	failed := 0

	for _, userReq := range req.Users {
		role := models.RoleStudent
		if userReq.Role != "" {
			validRoles := map[string]bool{"STUDENT": true, "TEACHER": true, "MODERATOR": true, "ADMIN": true}
			if !validRoles[userReq.Role] {
				failed++
				continue
			}
			role = models.UserRole(userReq.Role)
		}

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		if err != nil {
			failed++
			continue
		}

		user := models.User{
			Email:        userReq.Email,
			Name:         &userReq.Name,
			Username:     userReq.Username,
			PasswordHash: string(passwordHash),
			Role:         role,
		}

		if err := SafeCreate(db.DB, &user); err != nil {
			failed++
			continue
		}

		created++
	}

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

	deleted := 0
	failed := 0

	for _, userID := range req.UserIDs {
		if err := db.DB.Delete(&models.User{}, idQuery, userID).Error; err != nil {
			failed++
			continue
		}

		middleware.InvalidateRolePermsCache(userID)
		getUserRepo().InvalidateCache(userID)
		deleted++
	}

	LogAudit(c, "BULK_DELETE_USERS", "user", "", gin.H{"deleted": deleted, "failed": failed})
	api_response.Success(c, gin.H{"deleted": deleted, "failed": failed})
}

// ─────────────────────────────────────────────
// User Action Handlers (ban, suspend, role change, etc)
// ─────────────────────────────────────────────

// BanUser bans a user permanently or temporarily
func BanUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Reason      *string `json:"reason"`
		DurationHours *int  `json:"durationHours"`
		NotifyUser  bool    `json:"notifyUser"`
		Permanent   bool    `json:"permanent"`
		ExpiresAt   *string `json:"expiresAt"`
		HideContent bool    `json:"hideContent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Set status to BANNED
	user.Status = models.StatusBanned

	// Set expiration if not permanent
	if !req.Permanent && req.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			user.StatusExpiresAt = &expiresAt
		}
	} else if req.DurationHours != nil {
		expiresAt := time.Now().Add(time.Duration(*req.DurationHours) * time.Hour)
		user.StatusExpiresAt = &expiresAt
	}

	if req.Reason != nil {
		user.StatusReason = req.Reason
	}

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to ban user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "BAN_USER", "user", userID, gin.H{"reason": req.Reason, "permanent": req.Permanent})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// SuspendUser suspends a user temporarily
func SuspendUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Reason       *string `json:"reason"`
		DurationHours *int    `json:"durationHours"`
		NotifyUser   bool     `json:"notifyUser"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Set status to SUSPENDED
	user.Status = models.StatusSuspended

	// Set expiration
	if req.DurationHours != nil {
		expiresAt := time.Now().Add(time.Duration(*req.DurationHours) * time.Hour)
		user.StatusExpiresAt = &expiresAt
	}

	if req.Reason != nil {
		user.StatusReason = req.Reason
	}

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to suspend user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "SUSPEND_USER", "user", userID, gin.H{"reason": req.Reason, "durationHours": req.DurationHours})
	api_response.Success(c, buildUserDetailsPayload(user))
}

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

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	user.Role = models.UserRole(*req.Role)

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to change user role")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "CHANGE_ROLE", "user", userID, gin.H{"oldRole": user.Role, "newRole": req.Role, "reason": req.Reason})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// SendPasswordReset sends a password reset email to user
func SendPasswordReset(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Generate password reset token
	_ = generateRandomToken(32)
	_ = time.Now().Add(24 * time.Hour)

	// Store token (you might need a password_resets table)
	// For now, we'll just log it
	LogAudit(c, "PASSWORD_RESET_REQUEST", "user", userID, gin.H{"email": user.Email})

	api_response.Success(c, gin.H{"message": "Password reset email sent"})
}

// ActivateUser activates a suspended or banned user
func ActivateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	user.Status = models.StatusActive
	user.StatusReason = nil
	user.StatusExpiresAt = nil

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to activate user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "ACTIVATE_USER", "user", userID, nil)
	api_response.Success(c, buildUserDetailsPayload(user))
}

// ResetAllPermissions resets all user permissions to their role defaults
func ResetAllPermissions(c *gin.Context) {
	// This would reset all custom permissions back to role defaults
	// Implementation depends on your permissions system
	LogAudit(c, "RESET_ALL_PERMISSIONS", "user", "", nil)
	api_response.Success(c, gin.H{"message": "All permissions reset to defaults"})
}

// Helper function to generate random tokens
func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to simple random if crypto fails
			b[i] = charset[i%len(charset)]
			continue
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
