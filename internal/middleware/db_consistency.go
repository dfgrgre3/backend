package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"thanawy-backend/internal/db"
)

// DBConsistencyMiddleware ensures that if a user performs a write operation (POST, PUT, DELETE, PATCH),
// subsequent reads for a short period (e.g., 5 seconds) are forced to the Source database
// to avoid replication lag issues (Read-After-Write inconsistency).
func DBConsistencyMiddleware(gormDB *gorm.DB) gin.HandlerFunc {
	return dbConsistencyMiddleware(context.Background(), gormDB, 5*time.Second, time.Minute)
}

func dbConsistencyMiddleware(ctx context.Context, gormDB *gorm.DB, consistencyWindow, cleanupInterval time.Duration) gin.HandlerFunc {
	// Fallback local map
	writeTracker := &sync.Map{}

	// Eviction loop to prevent memory leak in local fallback map
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				writeTracker.Range(func(key, value interface{}) bool {
					if lastWrite, ok := value.(time.Time); ok {
						if now.Sub(lastWrite) > consistencyWindow {
							writeTracker.Delete(key)
						}
					}
					return true
				})
			}
		}
	}()

	return func(c *gin.Context) {
		method := c.Request.Method
		userID := c.GetString("userId")
		if userID == "" {
			userID = c.GetString("user_id")
		}
		if userID == "" {
			userID = c.ClientIP()
		}

		ctx := c.Request.Context()

		// If it's a write operation, record the timestamp
		if isWriteMethod(method) {
			if db.Redis != nil {
				// Set write flag in Redis with TTL matching consistency window
				_ = db.Redis.Set(ctx, "db_consistency:write:"+userID, "1", consistencyWindow).Err()
			} else {
				writeTracker.Store(userID, time.Now())
			}
			c.Next()
			return
		}

		// For GET/HEAD requests, check if we should force Source
		shouldForceSource := false
		if isReadMethod(method) && db.Redis != nil {
			val, err := db.Redis.Exists(ctx, "db_consistency:write:"+userID).Result()
			if err == nil && val > 0 {
				shouldForceSource = true
			}
		} else if isReadMethod(method) {
			if lastWrite, ok := writeTracker.Load(userID); ok {
				if time.Since(lastWrite.(time.Time)) < consistencyWindow {
					shouldForceSource = true
				}
			}
		}

		if shouldForceSource {
			// Force read from Source to ensure consistency
			c.Set("db", gormDB.Session(&gorm.Session{}).Clauses(dbresolver.Write))
		}

		// Also check for explicit consistency header
		if c.GetHeader("X-Consistency-Level") == "strong" {
			c.Set("db", gormDB.Session(&gorm.Session{}).Clauses(dbresolver.Write))
		}

		c.Next()
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

func isReadMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}
