package aiservice

import (
	models "thanawy-backend/internal/domain/common"
	database "thanawy-backend/internal/infrastructure/database"
	"time"

	"github.com/google/uuid"
)

// LogAIInteraction logs AI usage for analytics and cost tracking
func (s *AIService) LogAIInteraction(action, userID, input, output string) error {
	interaction := models.AIConversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     action,
		CreatedAt: time.Now(),
	}

	if userID == "" {
		interaction.UserID = "system"
	}

	// Create a message for the conversation
	message := models.AIMessage{
		ID:             uuid.New().String(),
		ConversationID: interaction.ID,
		Role:           "user",
		Content:        input,
		CreatedAt:      time.Now(),
	}

	// Save conversation and first message
	tx := database.DB.Begin()
	defer func() {
		if recover() != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&interaction).Error; err != nil {
		tx.Rollback()
		return err
	}

	message.ConversationID = interaction.ID
	if err := tx.Create(&message).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Add assistant response if provided
	if output != "" {
		assistantMsg := models.AIMessage{
			ID:             uuid.New().String(),
			ConversationID: interaction.ID,
			Role:           "assistant",
			Content:        output,
			CreatedAt:      time.Now(),
		}
		if err := tx.Create(&assistantMsg).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
