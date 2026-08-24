package protected

import (
	"context"
	"log"
	"net/http"
	"strings"
	"thanawy-backend/internal/infrastructure/api/response"
	worker "thanawy-backend/internal/infrastructure/workers"

	"github.com/gin-gonic/gin"
)

// SummarizeLessonProxy is now a thin enqueue endpoint.
// It returns 202 Accepted + jobId immediately (< 50 ms).
// The worker runs the actual LLM call asynchronously.
// Polling: GET /api/ai/summarize/status/:jobId
func (h *AIHandler) SummarizeLessonProxy(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		response.Error(c, http.StatusBadRequest, "content is required")
		return
	}

	jobID, err := worker.EnqueueLessonSummary(worker.LessonSummaryPayload{
		UserID:  userID,
		Content: req.Content,
	})
	if err != nil {
		log.Printf("[SummarizeLessonProxy] enqueue failed: %v", err)
		response.Error(c, http.StatusInternalServerError, "Failed to enqueue summarization job")
		return
	}

	if jobID == "" {
		// Redis unavailable — synchronous fallback via AIChatProxy
		log.Printf("[SummarizeLessonProxy] Redis unavailable, falling back to sync path for user=%s", userID)
		h.AIChatProxy(c)
		return
	}

	response.Success(c, gin.H{"jobId": jobID, "status": "queued"})
}

// GetSummarizeStatus is the polling endpoint for lesson summarization jobs.
func (h *AIHandler) GetSummarizeStatus(c *gin.Context) {
	h.pollAIJobStatus(c, worker.GetSummaryJobResult)
}

// GradeEssayProxy is now a thin enqueue endpoint.
// It returns 202 Accepted + jobId immediately (< 50 ms).
// Polling: GET /api/ai/grade-essay/status/:jobId
func (h *AIHandler) GradeEssayProxy(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	var req struct {
		Content  string `json:"content"`
		Topic    string `json:"topic"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		response.Error(c, http.StatusBadRequest, "content is required")
		return
	}

	jobID, err := worker.EnqueueEssayGrade(worker.EssayGradePayload{
		UserID:   userID,
		Content:  req.Content,
		Topic:    req.Topic,
		Language: req.Language,
	})
	if err != nil {
		log.Printf("[GradeEssayProxy] enqueue failed: %v", err)
		response.Error(c, http.StatusInternalServerError, "Failed to enqueue essay grading job")
		return
	}

	if jobID == "" {
		// Redis unavailable — synchronous fallback via AIChatProxy
		log.Printf("[GradeEssayProxy] Redis unavailable, falling back to sync path for user=%s", userID)
		h.AIChatProxy(c)
		return
	}

	response.Success(c, gin.H{"jobId": jobID, "status": "queued"})
}

// GetEssayGradeStatus is the polling endpoint for essay grading jobs.
func (h *AIHandler) GetEssayGradeStatus(c *gin.Context) {
	h.pollAIJobStatus(c, worker.GetEssayJobResult)
}

// pollAIJobStatus is the shared implementation for all AI job polling endpoints.
func (h *AIHandler) pollAIJobStatus(c *gin.Context, getter func(context.Context, string) (*worker.AIJobResult, bool)) {
	jobID := strings.TrimSpace(c.Param("jobId"))
	if jobID == "" {
		response.Error(c, http.StatusBadRequest, "jobId is required")
		return
	}

	if _, ok := h.getAuthorizedUserID(c); !ok {
		return
	}

	result, found := getter(c.Request.Context(), jobID)
	if !found {
		response.Error(c, http.StatusNotFound, "Job not found or expired")
		return
	}

	// Map generic AIJobResult fields to the response shape the frontend expects.
	// EssayGrader expects {evaluation:string}; LessonSummarizer expects {summary:string}.
	// We include both so this single handler works for both polling endpoints.
	resp := gin.H{
		"status":     result.Status,
		"jobId":      result.JobID,
		"result":     result.Result, // raw: frontends can read this generic field
		"evaluation": result.Result, // alias for EssayGrader
		"summary":    result.Result, // alias for LessonSummarizer
	}
	if result.Error != "" {
		resp["error"] = result.Error
	}
	response.Success(c, resp)
}
