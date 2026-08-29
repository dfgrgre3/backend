package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"log"
	"net/http"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	idempotencyTTL    = 24 * time.Hour
	idempotencyPrefix = "idempotency:"
)

type idempotencyResponse struct {
	Status int
	Body   json.RawMessage
}

func Idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := idempotencyKey(c)
		if !ok {
			c.Next()
			return
		}

		dedupKey := idempotencyPrefix + c.Request.Method + ":" + c.FullPath() + ":" + key
		if replayCachedIdempotencyResponse(c, dedupKey) {
			return
		}

		locked, err := lockIdempotencyRequest(c, dedupKey)
		if err != nil || !locked {
			handleIdempotencyLockResult(c, err)
			return
		}
		defer cache.Redis.Del(c.Request.Context(), idempotencyLockKey(dedupKey))

		blw := captureIdempotencyResponse(c)
		c.Next()
		cacheIdempotencyResponse(c, dedupKey, blw)
	}
}

func idempotencyKey(c *gin.Context) (string, bool) {
	if isSafeIdempotencyMethod(c.Request.Method) || cache.Redis == nil {
		return "", false
	}

	key := c.GetHeader("Idempotency-Key")
	return key, key != ""
}

func isSafeIdempotencyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodOptions, http.MethodHead:
		return true
	default:
		return false
	}
}

func replayCachedIdempotencyResponse(c *gin.Context, dedupKey string) bool {
	existing, err := cache.Redis.Get(c.Request.Context(), dedupKey).Result()
	if err != nil || existing == "" {
		return false
	}

	var cached idempotencyResponse
	if json.Unmarshal([]byte(existing), &cached) != nil {
		return false
	}

	c.AbortWithStatusJSON(cached.Status, cached.Body)
	return true
}

func lockIdempotencyRequest(c *gin.Context, dedupKey string) (bool, error) {
	bodyHash := sha256.Sum256(idempotencyBody(c))
	return cache.Redis.SetNX(c.Request.Context(), idempotencyLockKey(dedupKey), string(bodyHash[:]), 30*time.Second).Result()
}

func idempotencyBody(c *gin.Context) []byte {
	bodyBytes, exists := c.Get(gin.BodyBytesKey)
	if !exists {
		return nil
	}

	bodyData, ok := bodyBytes.([]byte)
	if !ok {
		return nil
	}

	return bodyData
}

func idempotencyLockKey(dedupKey string) string {
	return dedupKey + ":lock"
}

func handleIdempotencyLockResult(c *gin.Context, err error) {
	if err != nil {
		// Fail closed, not open: the caller explicitly sent an
		// Idempotency-Key, opting into duplicate-request protection for
		// what is typically a payment/enrollment-style write. Letting the
		// request through here (as before) would silently drop that
		// protection during a Redis outage/blip, allowing the exact
		// double-execution (double charge, double enrollment) the header
		// was meant to prevent. A 503 tells the client to retry — which is
		// safe, since they're already using idempotency keys.
		log.Printf("[Idempotency] Redis error, rejecting to avoid unprotected duplicate execution: %v", err)
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": "Unable to verify request idempotency right now. Please retry with the same Idempotency-Key.",
		})
		return
	}

	c.AbortWithStatusJSON(http.StatusConflict, gin.H{
		"error": "Request is already being processed. Use the same Idempotency-Key to retry.",
	})
}

func captureIdempotencyResponse(c *gin.Context) *bodyLogWriter {
	blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
	c.Writer = blw
	return blw
}

func cacheIdempotencyResponse(c *gin.Context, dedupKey string, blw *bodyLogWriter) {
	if c.Writer.Status() >= 500 {
		return
	}

	resp := idempotencyResponse{
		Status: c.Writer.Status(),
		Body:   blw.body.Bytes(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	cache.Redis.Set(c.Request.Context(), dedupKey, string(data), idempotencyTTL)
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
