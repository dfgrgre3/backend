package protected

import (
	"sync"
	aiservice "thanawy-backend/internal/domain/ai/service"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
	airepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	"time"

	"github.com/gin-gonic/gin"
)

const (
	AIRequestTimeout   = 60 * time.Second
	MaxContextMessages = 20
	CacheTTL           = 24 * time.Hour
	MaxRetries         = 3
)

// AIHandler serves the AI-assistant endpoints (chat, exam generation, lesson
// summarization, essay grading, conversation history).
//
// Its methods are split across several files in this package (all sharing
// the protected package and the AIHandler receiver), grouped by area:
// this file (construction/wiring), ai_handler_chat.go (chat proxy + request
// validation), ai_handler_exam.go (async exam generation),
// ai_handler_summarize_grade.go (async lesson summary / essay grading),
// ai_handler_conversations.go (conversation CRUD), ai_handler_aliases.go
// (thin proxy aliases) and ai_handler_helpers.go (shared internals).
type AIHandler struct {
	conversationRepo models.AIConversationRepository
	aiService        *aiservice.AIService
}

var (
	sharedAIHandler *AIHandler
	aiHandlerOnce   sync.Once
)

func GetAIHandler() *AIHandler {
	aiHandlerOnce.Do(func() {
		sharedAIHandler = &AIHandler{
			conversationRepo: airepo.NewAIConversationRepo(db.DB),
			aiService:        aiservice.GetAIService(),
		}
	})
	return sharedAIHandler
}

func NewAIHandler() *AIHandler {
	return GetAIHandler()
}

type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversationId,omitempty"`
	SubjectID      string `json:"subjectId,omitempty"`
	TopicID        string `json:"topicId,omitempty"`
	Stream         bool   `json:"stream,omitempty"`
	Model          string `json:"model,omitempty"`
	Image          string `json:"image,omitempty"` // Base64 encoded image
}

type ChatResponse struct {
	Reply          string `json:"reply"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
}

// Package-level wrappers
func AIChatProxy(c *gin.Context)            { GetAIHandler().AIChatProxy(c) }
func AIExamProxy(c *gin.Context)            { GetAIHandler().AIExamProxy(c) }
func AISuggestProxy(c *gin.Context)         { GetAIHandler().AISuggestProxy(c) }
func AITipsProxy(c *gin.Context)            { GetAIHandler().AITipsProxy(c) }
func GetConversations(c *gin.Context)       { GetAIHandler().GetConversations(c) }
func GetConversation(c *gin.Context)        { GetAIHandler().GetConversation(c) }
func DeleteConversation(c *gin.Context)     { GetAIHandler().DeleteConversation(c) }
func ExplainMistakeProxy(c *gin.Context)    { GetAIHandler().ExplainMistakeProxy(c) }
func GenerateStudyPlanProxy(c *gin.Context) { GetAIHandler().GenerateStudyPlanProxy(c) }
func SummarizeLessonProxy(c *gin.Context)   { GetAIHandler().SummarizeLessonProxy(c) }
func GetSummarizeStatus(c *gin.Context)     { GetAIHandler().GetSummarizeStatus(c) }
func GradeEssayProxy(c *gin.Context)        { GetAIHandler().GradeEssayProxy(c) }
func GetEssayGradeStatus(c *gin.Context)    { GetAIHandler().GetEssayGradeStatus(c) }
func GetExamStatus(c *gin.Context)          { GetAIHandler().GetExamStatus(c) }
