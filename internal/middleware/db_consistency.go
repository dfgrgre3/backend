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
	writeTracker := &sync.Map{}
	startWriteTrackerCleanup(ctx, writeTracker, consistencyWindow, cleanupInterval)

	return func(c *gin.Context) {
		method := c.Request.Method
		userID := dbConsistencyUserID(c)

		if isWriteMethod(method) {
			recordDBConsistencyWrite(c.Request.Context(), writeTracker, userID, consistencyWindow)
			c.Next()
			return
		}

		if shouldForceSourceDB(c, method, writeTracker, userID, consistencyWindow) {
			forceSourceDB(c, gormDB)
		}

		c.Next()
	}
}

func startWriteTrackerCleanup(ctx context.Context, writeTracker *sync.Map, consistencyWindow, cleanupInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				deleteExpiredWrites(writeTracker, now, consistencyWindow)
			}
		}
	}()
}

func deleteExpiredWrites(writeTracker *sync.Map, now time.Time, consistencyWindow time.Duration) {
	writeTracker.Range(func(key, value interface{}) bool {
		lastWrite, ok := value.(time.Time)
		if ok && now.Sub(lastWrite) > consistencyWindow {
			writeTracker.Delete(key)
		}

		return true
	})
}

func dbConsistencyUserID(c *gin.Context) string {
	if userID := c.GetString("userId"); userID != "" {
		return userID
	}

	if userID := c.GetString("user_id"); userID != "" {
		return userID
	}

	return c.ClientIP()
}

func recordDBConsistencyWrite(ctx context.Context, writeTracker *sync.Map, userID string, consistencyWindow time.Duration) {
	if db.Redis != nil {
		_ = db.Redis.Set(ctx, dbConsistencyWriteKey(userID), "1", consistencyWindow).Err()
		return
	}

	writeTracker.Store(userID, time.Now())
}

func shouldForceSourceDB(c *gin.Context, method string, writeTracker *sync.Map, userID string, consistencyWindow time.Duration) bool {
	return c.GetHeader("X-Consistency-Level") == "strong" ||
		shouldForceSourceAfterWrite(c.Request.Context(), method, writeTracker, userID, consistencyWindow)
}

func shouldForceSourceAfterWrite(ctx context.Context, method string, writeTracker *sync.Map, userID string, consistencyWindow time.Duration) bool {
	if !isReadMethod(method) {
		return false
	}

	if db.Redis != nil {
		return hasRecentRedisWrite(ctx, userID)
	}

	return hasRecentTrackedWrite(writeTracker, userID, consistencyWindow)
}

func hasRecentRedisWrite(ctx context.Context, userID string) bool {
	val, err := db.Redis.Exists(ctx, dbConsistencyWriteKey(userID)).Result()
	return err == nil && val > 0
}

func hasRecentTrackedWrite(writeTracker *sync.Map, userID string, consistencyWindow time.Duration) bool {
	lastWrite, ok := writeTracker.Load(userID)
	if !ok {
		return false
	}

	writtenAt, ok := lastWrite.(time.Time)
	return ok && time.Since(writtenAt) < consistencyWindow
}

func dbConsistencyWriteKey(userID string) string {
	return "db_consistency:write:" + userID
}

func forceSourceDB(c *gin.Context, gormDB *gorm.DB) {
	c.Set("db", gormDB.Session(&gorm.Session{}).Clauses(dbresolver.Write))
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
