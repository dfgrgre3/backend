package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/gorm"
)

// =============================================================
// Constants & Configuration
// =============================================================

const (
	UserCachePrefix         = "user:"
	UserCacheTTL            = 15 * time.Minute
	localUserCacheTTL       = 5 * time.Minute
	userEmailCacheKeyFormat = "%semail:%s"
	userIDCacheKeyFormat    = "%sid:%s"
	queryByEmail            = "email ILIKE ?"
)

// =============================================================
// Types & Global State
// =============================================================

// InMemoryUserCache represents a locally cached user with expiration
type InMemoryUserCache struct {
	User      *models.User
	ExpiresAt time.Time
}

var (
	localUserCache sync.Map
)

// Column existence check (runs once at startup)
var (
	userDeletedAtOnce         sync.Once
	userDeletedAtColumnExists bool
)

func hasUserDeletedAtColumn() bool {
	userDeletedAtOnce.Do(func() {
		if db.DB != nil {
			userDeletedAtColumnExists = db.DB.Migrator().HasColumn(&models.User{}, "deleted_at")
		}
	})
	return userDeletedAtColumnExists
}

// WarmupUserRepository triggers the deleted_at column check at startup
// so it doesn't run on the first request. Call after DB connection is established.
func WarmupUserRepository() {
	hasUserDeletedAtColumn()
}

// =============================================================
// UserRepository
// =============================================================

// UserRepository handles user persistence with multi-layer caching
type UserRepository struct {
	db                 *gorm.DB
	hasDeletedAtColumn bool
}

// NewUserRepository creates a new UserRepository instance
func NewUserRepository(dbConn *gorm.DB) *UserRepository {
	return &UserRepository{
		db:                 dbConn,
		hasDeletedAtColumn: hasUserDeletedAtColumn(),
	}
}

// activeUserDB returns the appropriate GORM scope based on soft-delete support
func (r *UserRepository) activeUserDB() *gorm.DB {
	if r.hasDeletedAtColumn {
		return r.db
	}
	return r.db.Unscoped()
}

// =============================================================
// CRUD Operations
// =============================================================

// Create inserts a new user and populates caches
func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return err
	}
	r.cacheUser(user)
	return nil
}

// FindByEmail retrieves a user by email with singleflight deduplication
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	cacheKey := fmt.Sprintf(userEmailCacheKeyFormat, UserCachePrefix, email)

	val, err, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
		var user models.User

		// 1. Try Redis cache first
		if cache.Redis != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			cachedVal, redisErr := cache.Redis.Get(ctx, cacheKey).Result()
			cancel()

			if redisErr == nil {
				if unmarshalErr := json.Unmarshal([]byte(cachedVal), &user); unmarshalErr == nil {
					return &user, nil
				}
			}
		}

		// 2. Fallback to database
		if dbErr := r.activeUserDB().Where(queryByEmail, email).Take(&user).Error; dbErr != nil {
			return nil, dbErr
		}

		// 3. Populate cache on successful DB fetch
		r.cacheUser(&user)
		return &user, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(*models.User), nil
}

// FindByEmailNoCache retrieves a user by email bypassing all cache layers
func (r *UserRepository) FindByEmailNoCache(email string) (*models.User, error) {
	var user models.User
	if err := r.activeUserDB().Where(queryByEmail, email).Take(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID retrieves a user by ID with local memory + Redis + DB fallback
func (r *UserRepository) FindByID(id string) (*models.User, error) {
	// Layer 1: Local in-memory cache (bypasses network latency)
	if cachedUser, ok := r.tryLocalUserCache(id); ok {
		return cachedUser, nil
	}

	cacheKey := fmt.Sprintf(userIDCacheKeyFormat, UserCachePrefix, id)

	// Layer 2 & 3: SingleFlight -> Redis -> DB
	val, err, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
		return r.fetchUserFromStore(cacheKey, id)
	})

	if err != nil {
		return nil, err
	}

	resUser := val.(*models.User)

	// Update local memory cache on successful fetch
	localUserCache.Store(id, InMemoryUserCache{
		User:      resUser,
		ExpiresAt: time.Now().Add(localUserCacheTTL),
	})

	return resUser, nil
}

// Update modifies an existing user and invalidates stale cache entries
func (r *UserRepository) Update(user *models.User) error {
	var oldEmail string
	if user.ID != "" {
		var existing models.User
		if err := r.db.Select("email").Take(&existing, queryByID, user.ID).Error; err == nil {
			oldEmail = existing.Email
		}
	}

	if err := r.db.Save(user).Error; err != nil {
		return err
	}

	// Invalidate old email cache if email changed
	if oldEmail != "" && oldEmail != user.Email && cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cache.Redis.Del(ctx, fmt.Sprintf(userEmailCacheKeyFormat, UserCachePrefix, oldEmail))
		cancel()
	}

	r.cacheUser(user)
	return nil
}

// InvalidateCache manually evicts a user from both local and Redis caches
func (r *UserRepository) InvalidateCache(id string) {
	localUserCache.Delete(id)

	if cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cache.Redis.Del(ctx, fmt.Sprintf(userIDCacheKeyFormat, UserCachePrefix, id))
	}
}

// =============================================================
// Private Helpers
// =============================================================

// tryLocalUserCache attempts to fetch from in-memory cache
func (r *UserRepository) tryLocalUserCache(id string) (*models.User, bool) {
	val, ok := localUserCache.Load(id)
	if !ok {
		return nil, false
	}

	cached := val.(InMemoryUserCache)
	if time.Now().Before(cached.ExpiresAt) {
		return cached.User, true
	}

	// Expired entry: clean up lazily
	localUserCache.Delete(id)
	return nil, false
}

// fetchUserFromStore retrieves user from Redis then DB, caching on success
func (r *UserRepository) fetchUserFromStore(cacheKey, id string) (*models.User, error) {
	// Try Redis first
	if cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		cachedVal, err := cache.Redis.Get(ctx, cacheKey).Result()
		cancel()

		if err == nil {
			var user models.User
			if unmarshalErr := json.Unmarshal([]byte(cachedVal), &user); unmarshalErr == nil {
				return &user, nil
			}
		}
	}

	// Fallback to database
	var user models.User
	if err := r.activeUserDB().Take(&user, queryByID, id).Error; err != nil {
		return nil, err
	}

	r.cacheUser(&user)
	return &user, nil
}

// cacheUser populates both local memory and Redis caches
func (r *UserRepository) cacheUser(user *models.User) {
	if user == nil || user.ID == "" {
		return
	}

	// Always populate local cache
	localUserCache.Store(user.ID, InMemoryUserCache{
		User:      user,
		ExpiresAt: time.Now().Add(localUserCacheTTL),
	})

	if cache.Redis == nil {
		return
	}

	data, marshalErr := json.Marshal(user)
	if marshalErr != nil {
		log.Printf("[UserRepo] Failed to marshal user %s for cache: %v", user.ID, marshalErr)
		return
	}

	// Async Redis write to avoid blocking the main flow
	go func(id, email string, payload []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		idKey := fmt.Sprintf(userIDCacheKeyFormat, UserCachePrefix, id)
		emailKey := fmt.Sprintf(userEmailCacheKeyFormat, UserCachePrefix, email)

		cache.Redis.Set(ctx, idKey, payload, UserCacheTTL)
		if email != "" {
			cache.Redis.Set(ctx, emailKey, payload, UserCacheTTL)
		}
	}(user.ID, user.Email, data)
}
