package protected

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *AIHandler) buildAIMessages(history []models.AIMessage) []map[string]interface{} {
	messages := []map[string]interface{}{
		{"role": "system", "content": "You are a helpful educational assistant for the Thanawy platform."},
	}

	for _, m := range history {
		messages = append(messages, map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	return messages
}

// contentToString safely extracts a string from an interface{} that may be a string or an array
func (h *AIHandler) buildCacheKey(messages []map[string]interface{}) string {
	data, _ := json.Marshal(messages)
	return fmt.Sprintf("ai_cache:%x", data)
}

func (h *AIHandler) getCachedResponse(ctx context.Context, key string) string {
	if cache.Redis == nil {
		return ""
	}
	val, err := cache.Redis.Get(ctx, key).Result()
	if err == nil {
		return val
	}
	return ""
}

func (h *AIHandler) cacheResponse(ctx context.Context, key, response string) {
	if cache.Redis == nil {
		return
	}
	cache.Redis.Set(ctx, key, response, CacheTTL)
}

func (h *AIHandler) callAIWithRetryCustom(ctx context.Context, messages []map[string]interface{}, model string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, AIRequestTimeout)
	defer cancel()

	var lastErr error
	for i := 0; i < MaxRetries; i++ {
		reply, err := h.aiService.GenerateContentWithMessages(ctx, messages, model)
		if err == nil {
			return reply, model, nil
		}
		lastErr = err
		// Use a cryptographically secure random number for jitter to satisfy security scanners (S2245)
		jitter := int64(1000)
		if n, err := rand.Int(rand.Reader, big.NewInt(jitter)); err == nil {
			time.Sleep(time.Duration(n.Int64()) * time.Millisecond)
		} else {
			// Fallback if crypto/rand fails
			time.Sleep(time.Duration(jitter/2) * time.Millisecond)
		}
	}
	return "", "", lastErr
}

func (h *AIHandler) handleStreamingChat(c *gin.Context, messages []map[string]interface{}, convID, model string) {
	// Simple non-streaming fallback for now
	reply, usedModel, err := h.callAIWithRetryCustom(c.Request.Context(), messages, model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	assistantMessage := &models.AIMessage{
		ConversationID: convID,
		Role:           "assistant",
		Content:        reply,
		Model:          stringPtr(usedModel),
	}
	h.conversationRepo.AddMessage(assistantMessage)

	c.JSON(http.StatusOK, ChatResponse{
		Reply:          reply,
		ConversationID: convID,
		MessageID:      assistantMessage.ID,
	})
}

func stringPtr(s string) *string {
	return &s
}

func isValidBase64Image(s string) bool {
	if !strings.HasPrefix(s, "data:image/") {
		return false
	}
	return true // Basic check
}
