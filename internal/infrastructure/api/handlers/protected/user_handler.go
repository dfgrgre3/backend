package protected

import (
	"crypto/rand"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"
	authrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	userrepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	"github.com/shopspring/decimal"
)

var (
	userRepo        *userrepo.UserRepository
	userRepoOnce    sync.Once
	sessionRepo     *authrepo.SessionRepository
	sessionRepoOnce sync.Once

	errFailedToGenerateTokens = os.Getenv("ERRFAILEDTOGENERATETOKENS")
	refreshTokenPath          = os.Getenv("REFRESHTOKENPATH")
	errInvalidEmail           = "Invalid email"
)

func getUserRepo() *userrepo.UserRepository {
	userRepoOnce.Do(func() {
		userRepo = userrepo.NewUserRepository(db.DB)
	})
	return userRepo
}

func getSessionRepo() *authrepo.SessionRepository {
	sessionRepoOnce.Do(func() {
		sessionRepo = authrepo.NewSessionRepository(db.DB)
	})
	return sessionRepo
}

type userAggregateCountRow struct {
	UserID string `gorm:"column:user_id"`
	Kind   string `gorm:"column:kind"`
	Count  int64  `gorm:"column:count"`
}

func aggregateUserCountRows(rows []userAggregateCountRow) (map[string]int64, map[string]int64, map[string]int64, map[string]int64) {
	taskMap := make(map[string]int64, len(rows))
	sessionMap := make(map[string]int64, len(rows))
	achievementMap := make(map[string]int64, len(rows))
	enrollmentMap := make(map[string]int64, len(rows))

	for _, row := range rows {
		switch row.Kind {
		case "tasks":
			taskMap[row.UserID] = row.Count
		case "sessions":
			sessionMap[row.UserID] = row.Count
		case "achievements":
			achievementMap[row.UserID] = row.Count
		case "enrollments":
			enrollmentMap[row.UserID] = row.Count
		}
	}

	return taskMap, sessionMap, achievementMap, enrollmentMap
}

func fetchUserAggregateCounts(userIDs []string) ([]userAggregateCountRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	var rows []userAggregateCountRow
	query := `
		SELECT user_id, 'tasks' AS kind, COUNT(*) AS count
		FROM "Task"
		WHERE user_id IN ? AND deleted_at IS NULL
		GROUP BY user_id
		UNION ALL
		SELECT user_id, 'sessions' AS kind, COUNT(*) AS count
		FROM "StudySession"
		WHERE user_id IN ? AND deleted_at IS NULL
		GROUP BY user_id
		UNION ALL
		SELECT user_id, 'achievements' AS kind, COUNT(*) AS count
		FROM "UserAchievement"
		WHERE user_id IN ? AND deleted_at IS NULL
		GROUP BY user_id
		UNION ALL
		SELECT user_id, 'enrollments' AS kind, COUNT(*) AS count
		FROM "SubjectEnrollment"
		WHERE user_id IN ? AND deleted_at IS NULL
		GROUP BY user_id
	`

	if err := db.DB.Raw(query, userIDs, userIDs, userIDs, userIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
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

// ─── L1 in-memory cache for billing summary ──────────────
func calculateTotalPages(total int64, limit int) int64 {
	return shared.CalculateTotalPages(total, limit)
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
		Balance:       decimal.NewFromInt(0),
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

func sanitizeLog(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}

// generateRandomToken returns a random alphanumeric token of the given length.
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
