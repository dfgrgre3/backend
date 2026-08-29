package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetConversations returns all AI conversations for the user
func (h *AIHandler) GetConversations(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	convs, total, err := h.conversationRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to fetch conversations")
		return
	}
	response.Success(c, gin.H{
		"conversations": convs,
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

// GetConversation returns a single AI conversation with messages
func (h *AIHandler) GetConversation(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	conv, err := h.conversationRepo.FindByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Conversation not found")
		return
	}
	// Ownership check: conversations are only readable by the user who owns
	// them. Without this, any authenticated user could read another user's
	// AI conversation by guessing/enumerating its id.
	if conv.UserID != userID {
		response.Error(c, http.StatusNotFound, "Conversation not found")
		return
	}
	messages, _ := h.conversationRepo.GetRecentMessages(id, 100)
	response.Success(c, gin.H{"conversation": conv, "messages": messages})
}

// DeleteConversation removes an AI conversation
func (h *AIHandler) DeleteConversation(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	conv, err := h.conversationRepo.FindByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Conversation not found")
		return
	}
	// Ownership check: see GetConversation for rationale.
	if conv.UserID != userID {
		response.Error(c, http.StatusNotFound, "Conversation not found")
		return
	}
	if err := h.conversationRepo.Delete(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete conversation")
		return
	}
	response.Success(c, gin.H{"message": "Conversation deleted"})
}

func (h *AIHandler) getOrCreateConversation(userID, convID, subjectID, topicID string) (*models.AIConversation, error) {
	if convID != "" {
		conv, err := h.conversationRepo.FindByID(convID)
		if err == nil && conv.UserID == userID {
			return conv, nil
		}
	}

	// Create new conversation
	var sID *string
	if subjectID != "" {
		sID = &subjectID
	}
	var tID *string
	if topicID != "" {
		tID = &topicID
	}

	conv := &models.AIConversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		SubjectID: sID,
		TopicID:   tID,
		Title:     "New Chat",
		CreatedAt: time.Now(),
	}
	if err := h.conversationRepo.Create(conv); err != nil {
		return nil, err
	}
	return conv, nil
}
