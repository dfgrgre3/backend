package protected

import (
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AIChatProxy handles chat requests with conversation history (new version)
func (h *AIHandler) AIChatProxy(c *gin.Context) {
	var req ChatRequest
	if !h.bindAndValidateRequest(c, &req) {
		return
	}

	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	// Get or create conversation
	conversation, err := h.getOrCreateConversation(userID, req.ConversationID, req.SubjectID, req.TopicID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to manage conversation")
		return
	}

	// Process and save user message
	_, userContent := h.processUserMessage(&req, conversation.ID)

	// Get history and build messages for AI API
	messages, err := h.conversationRepo.GetRecentMessages(conversation.ID, MaxContextMessages)
	if err != nil {
		messages = []models.AIMessage{}
	}

	aiMessages := h.buildAIMessages(messages)
	if req.Image != "" {
		aiMessages[len(aiMessages)-1]["content"] = userContent
	}

	// Check cache (only for text-only requests)
	cacheKey := ""
	if req.Image == "" {
		cacheKey = h.buildCacheKey(aiMessages)
		if cachedResponse := h.getCachedResponse(c.Request.Context(), cacheKey); cachedResponse != "" {
			h.respondWithCached(c, conversation.ID, cachedResponse)
			return
		}
	}

	// Select model and handle streaming
	model := "google/gemini-2.0-flash-001"
	if req.Image != "" {
		model = "google/gemini-pro-1.5"
	}

	if req.Stream {
		h.handleStreamingChat(c, aiMessages, conversation.ID, model)
		return
	}

	// Handle standard AI response
	h.handleAIResponse(c, conversation.ID, userID, cacheKey, model, aiMessages, req.Image != "")
}

func (h *AIHandler) bindAndValidateRequest(c *gin.Context, req *ChatRequest) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return false
	}

	if !h.validateMessage(c, req) {
		return false
	}

	if !h.validateImage(c, req) {
		return false
	}

	return h.validateRequestStructure(c, req)
}

func (h *AIHandler) validateMessage(c *gin.Context, req *ChatRequest) bool {
	if req.Message != "" {
		if len([]rune(req.Message)) > 2000 {
			response.Error(c, http.StatusBadRequest, "Message exceeds maximum length of 2000 characters")
			return false
		}
		if strings.TrimSpace(req.Message) == "" && req.Image == "" {
			response.Error(c, http.StatusBadRequest, "Message cannot be empty")
			return false
		}
	}
	return true
}

func (h *AIHandler) validateImage(c *gin.Context, req *ChatRequest) bool {
	if req.Image != "" {
		if len(req.Image) > 5*1024*1024 {
			response.Error(c, http.StatusBadRequest, "Image size exceeds 5MB limit")
			return false
		}
		if !isValidBase64Image(req.Image) {
			response.Error(c, http.StatusBadRequest, "Invalid image format")
			return false
		}
	}
	return true
}

func (h *AIHandler) validateRequestStructure(c *gin.Context, req *ChatRequest) bool {
	if (req.Message == "" && req.Image == "") || (req.ConversationID != "" && len(req.ConversationID) > 100) {
		errorMsg := "Message or image is required"
		if req.ConversationID != "" {
			errorMsg = "Invalid conversation ID"
		}
		response.Error(c, http.StatusBadRequest, errorMsg)
		return false
	}
	return true
}

func (h *AIHandler) getAuthorizedUserID(c *gin.Context) (string, bool) {
	userIDValue, exists := c.Get("userId")
	if !exists || userIDValue == nil {
		response.Error(c, http.StatusUnauthorized, "Authentication required to use AI features")
		return "", false
	}
	userIDStr, ok := userIDValue.(string)
	if !ok || userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "Invalid user session")
		return "", false
	}
	return userIDStr, true
}

func (h *AIHandler) processUserMessage(req *ChatRequest, conversationID string) (string, interface{}) {
	userMessageText := req.Message
	var userContent interface{}

	if req.Image != "" {
		if userMessageText == "" {
			userMessageText = "[صورة]"
		}
		userContent = []map[string]interface{}{
			{"type": "text", "text": req.Message},
			{"type": "image_url", "image_url": map[string]string{"url": req.Image}},
		}
	} else {
		userContent = req.Message
	}

	userMessage := &models.AIMessage{
		ConversationID: conversationID,
		Role:           "user",
		Content:        userMessageText,
	}
	if err := h.conversationRepo.AddMessage(userMessage); err != nil {
		log.Printf("Failed to save user message: %v", err)
	}

	return userMessageText, userContent
}

func (h *AIHandler) respondWithCached(c *gin.Context, conversationID, cachedResponse string) {
	assistantMessage := &models.AIMessage{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        cachedResponse,
		Model:          stringPtr("cached"),
	}
	h.conversationRepo.AddMessage(assistantMessage)
	response.Success(c, ChatResponse{
		Reply:          cachedResponse,
		ConversationID: conversationID,
		MessageID:      assistantMessage.ID,
	})
}

func (h *AIHandler) handleAIResponse(c *gin.Context, conversationID, userID, cacheKey, model string, aiMessages []map[string]interface{}, isVision bool) {
	reply, usedModel, err := h.callAIWithRetryCustom(c.Request.Context(), aiMessages, model)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get AI response")
		return
	}

	assistantMessage := &models.AIMessage{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        reply,
		Model:          stringPtr(usedModel),
	}
	h.conversationRepo.AddMessage(assistantMessage)

	if cacheKey != "" {
		h.cacheResponse(c.Request.Context(), cacheKey, reply)
	}

	h.deductCredits(userID, isVision)

	response.Success(c, ChatResponse{
		Reply:          reply,
		ConversationID: conversationID,
		MessageID:      assistantMessage.ID,
	})
}

func (h *AIHandler) deductCredits(userID string, isVision bool) {
	if userID == "" {
		return
	}
	credits := 1
	if isVision {
		credits = 5
	}
	db.DB.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("aiCredits", gorm.Expr("GREATEST(0, \"aiCredits\" - ?)", credits))
}
