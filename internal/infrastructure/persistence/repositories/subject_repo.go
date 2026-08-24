package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// =============================================================
// Constants & Configuration
// =============================================================

const (
	SubjectCachePrefix    = "subject:"
	SubjectCacheTTL       = 30 * time.Minute // Subjects change less frequently than users
	subjectCacheKeyFormat = "%sid:%s"
)

// allowedSubjectFilters is a whitelist of safe column names for dynamic filtering.
// This prevents SQL injection through user-controlled filter keys.
var allowedSubjectFilters = map[string]string{
	"isActive":    "is_active",
	"isPublished": "is_published",
	"level":       "level",
	"categoryId":  "category_id",
	"language":    "language",
	"isFeatured":  "is_featured",
}

// =============================================================
// Repository
// =============================================================

// SubjectRepository handles subject persistence with caching and safe filtering
type SubjectRepository struct {
	db *gorm.DB
}

// sfGroup is shared at package level to ensure singleflight deduplication
// works across all repository instances (singleton pattern compatibility)
var sfGroup singleflight.Group

// NewSubjectRepository creates a new SubjectRepository instance
func NewSubjectRepository(dbConn *gorm.DB) *SubjectRepository {
	return &SubjectRepository{db: dbConn}
}

// =============================================================
// Read Operations
// =============================================================

// FindByID retrieves a subject by ID with singleflight + Redis cache
func (r *SubjectRepository) FindByID(id string) (*models.Subject, error) {
	cacheKey := fmt.Sprintf(subjectCacheKeyFormat, SubjectCachePrefix, id)

	val, err, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
		var subject models.Subject

		// Layer 1: Try Redis cache
		if cache.Redis != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			cachedVal, redisErr := cache.Redis.Get(ctx, cacheKey).Result()
			cancel()

			if redisErr == nil {
				if unmarshalErr := json.Unmarshal([]byte(cachedVal), &subject); unmarshalErr == nil {
					return &subject, nil
				}
			}
		}

		// Layer 2: Database with full relation preloading
		if dbErr := r.db.Preload("Topics.SubTopics.Attachments").First(&subject, queryByID, id).Error; dbErr != nil {
			return nil, dbErr
		}

		// Populate cache on successful DB fetch
		r.cacheSubject(&subject)
		return &subject, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(*models.Subject), nil
}

// FindAll retrieves subjects with safe dynamic filtering
func (r *SubjectRepository) FindAll(filters map[string]interface{}) ([]models.Subject, error) {
	var subjects []models.Subject
	query := r.db.Model(&models.Subject{}).Preload("Topics.SubTopics.Attachments")

	for k, v := range filters {
		// Only allow whitelisted filter keys to prevent SQL injection
		safeColumn, ok := allowedSubjectFilters[k]
		if !ok {
			log.Printf("[SubjectRepo] Ignoring unsafe/unknown filter key: %s", k)
			continue
		}
		query = query.Where(fmt.Sprintf("%s = ?", safeColumn), v)
	}

	if err := query.Find(&subjects).Error; err != nil {
		return nil, err
	}
	return subjects, nil
}

// =============================================================
// Write Operations
// =============================================================

// Create inserts a new subject and populates cache
func (r *SubjectRepository) Create(subject *models.Subject) error {
	if err := r.db.Create(subject).Error; err != nil {
		return err
	}
	r.cacheSubject(subject)
	return nil
}

// Update modifies an existing subject and refreshes cache
func (r *SubjectRepository) Update(subject *models.Subject) error {
	if err := r.db.Save(subject).Error; err != nil {
		return err
	}
	r.cacheSubject(subject)
	return nil
}

// Delete removes a subject and invalidates its cache
func (r *SubjectRepository) Delete(id string) error {
	if err := r.db.Delete(&models.Subject{}, queryByID, id).Error; err != nil {
		return err
	}

	if cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cache.Redis.Del(ctx, fmt.Sprintf(subjectCacheKeyFormat, SubjectCachePrefix, id))
	}
	return nil
}

// =============================================================
// Cache Management
// =============================================================

// cacheSubject asynchronously writes subject data to Redis
func (r *SubjectRepository) cacheSubject(subject *models.Subject) {
	if cache.Redis == nil || subject == nil || subject.ID == "" {
		return
	}

	data, marshalErr := json.Marshal(subject)
	if marshalErr != nil {
		log.Printf("[SubjectRepo] Failed to marshal subject %s for cache: %v", subject.ID, marshalErr)
		return
	}

	go func(id string, payload []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		key := fmt.Sprintf(subjectCacheKeyFormat, SubjectCachePrefix, id)
		cache.Redis.Set(ctx, key, payload, SubjectCacheTTL)
	}(subject.ID, data)
}

// InvalidateSubjectCache clears cached subject data when relations are updated
//
// PERFORMANCE/CORRECTNESS: Redis DEL does not glob-expand its key argument —
// a literal `Del(ctx, "subject:list:*")` only deletes a key named exactly
// "subject:list:*" (which never exists), so this call was a silent no-op and
// list caches never actually got invalidated here. Fixed to SCAN for
// matching keys and DEL each one, same pattern already used correctly by
// InvalidateAllSubjectCache below.
func (r *SubjectRepository) InvalidateSubjectCache(id string) {
	if cache.Redis == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Delete single subject cache
	cache.Redis.Del(ctx, fmt.Sprintf(subjectCacheKeyFormat, SubjectCachePrefix, id))

	// Delete list caches that might contain stale data (glob pattern requires
	// SCAN + DEL per matched key — see comment above).
	listPattern := fmt.Sprintf("%slist:*", SubjectCachePrefix)
	iter := cache.Redis.Scan(ctx, 0, listPattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := iter.Err(); err != nil {
			log.Printf("[SubjectRepo] Error during list cache scan: %v", err)
			break
		}
		cache.Redis.Del(ctx, iter.Val())
	}
	if err := iter.Err(); err != nil {
		log.Printf("[SubjectRepo] Final list cache scan error: %v", err)
	}

	// Invalidate dependent dashboard stats
	cache.Redis.Del(ctx, "admin:dashboard:stats")
}

// InvalidateAllSubjectCache clears all subject-related cache (for bulk updates)
func (r *SubjectRepository) InvalidateAllSubjectCache() {
	if cache.Redis == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pattern := fmt.Sprintf("%s*", SubjectCachePrefix)
	iter := cache.Redis.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		if err := iter.Err(); err != nil {
			log.Printf("[SubjectRepo] Error during cache scan: %v", err)
			break
		}
		cache.Redis.Del(ctx, iter.Val())
	}

	// Check final iterator error after loop exits
	if err := iter.Err(); err != nil {
		log.Printf("[SubjectRepo] Final scan error: %v", err)
	}

	// Invalidate dependent dashboard stats
	cache.Redis.Del(ctx, "admin:dashboard:stats")
}
