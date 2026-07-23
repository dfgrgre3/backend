package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": errUserNotFound})
			return
		}
		profilePtr, err = getUserRepo().FindByID(userIdStr)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": errUserNotFound})
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

	c.JSON(http.StatusOK, gin.H{
		"user":                 &profile,
		"hydratedRole":         role,
		"hydratedPerms":        perms,
		"accessTokenExpiresAt": c.GetInt64("accessTokenExpiresAt"),
	})
}

func GetUsers(c *gin.Context) {
	role := c.Query("role")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var users []models.User
	query := db.DB

	if role != "" {
		query = query.Where("role = ?", role)
	}
	if search != "" {
		query = query.Where("email ILIKE CONCAT('%', ?, '%') OR name ILIKE CONCAT('%', ?, '%') OR username ILIKE CONCAT('%', ?, '%')", search, search, search)
	}

	var total int64
	query.Model(&models.User{}).Count(&total)

	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	totalAdmins := int64(0)
	powerUsers := int64(0)
	db.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&totalAdmins)
	db.DB.Model(&models.User{}).Where("level >= ?", 10).Count(&powerUsers)

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":            user.ID,
			"email":         user.Email,
			"name":          user.Name,
			"username":      user.Username,
			"avatar":        user.Avatar,
			"role":          user.Role,
			"permissions":   user.GetEffectivePermissions(),
			"emailVerified": user.EmailVerified,
			"createdAt":     user.CreatedAt,
			"lastLogin":     nil,
			"totalXP":       user.TotalXP,
			"level":         user.Level,
			"currentStreak": 0,
			"_count": gin.H{
				"tasks":         0,
				"studySessions": 0,
				"achievements":  0,
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
			"totalUsers":  total,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errUserNotFound})
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

	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userId).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errUserNotFound})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	db.DB.First(&user, idQuery, user.ID)
	_ = getUserRepo().Update(&user)

	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)

	c.JSON(http.StatusOK, gin.H{"success": true, "user": user})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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
	c.JSON(http.StatusOK, responseData)
}

func checkBillingCaches(c *gin.Context, cacheKey string) bool {
	if val, ok := billingSummaryL1.Load(cacheKey); ok {
		entry := val.(*billingSummaryEntry)
		if time.Now().Before(entry.expiresAt) {
			c.JSON(http.StatusOK, entry.data)
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
				c.JSON(http.StatusOK, cachedData)
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

func sanitizeLog(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}
