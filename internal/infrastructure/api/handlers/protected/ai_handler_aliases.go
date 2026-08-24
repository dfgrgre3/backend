package protected

import (
	"github.com/gin-gonic/gin"
)

// buildLegacyExamPrompt is the synchronous-fallback prompt used when Redis
// is not available. It produces a non-JSON reply the legacy UI can display.
func (h *AIHandler) AISuggestProxy(c *gin.Context) {
	h.AIChatProxy(c)
}

// AITipsProxy handles study tips requests
func (h *AIHandler) AITipsProxy(c *gin.Context) {
	h.AIChatProxy(c)
}

// ExplainMistakeProxy handles requests to explain an exam mistake
func (h *AIHandler) ExplainMistakeProxy(c *gin.Context) {
	h.AIChatProxy(c)
}

// GenerateStudyPlanProxy handles study plan generation
func (h *AIHandler) GenerateStudyPlanProxy(c *gin.Context) {
	h.AIChatProxy(c)
}
