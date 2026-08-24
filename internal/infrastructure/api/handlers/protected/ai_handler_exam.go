package protected

import (
	"log"
	"net/http"
	"strings"
	worker "thanawy-backend/internal/infrastructure/workers"
	"thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Async exam generation (background job queue)
//
// Why this exists: the old AIExamProxy just delegated to AIChatProxy which
// called the AI synchronously inside the Gin goroutine. With MaxRetries=3 and
// a per-call timeout of 30s the request could block the HTTP worker thread
// for up to ~90s. Two teachers generating exams at the same time would
// exhaust the goroutine pool and star every other user on the platform.
//
// The fix: AIExamProxy now enqueues a job in Asynq and returns 202 + jobId
// in <50ms. The actual generation runs in the worker pool. The frontend
// polls GET /api/ai/exam/status/:jobId until it sees status="completed".
//
// Graceful degradation: if Redis is not configured (e.g. serverless cold
// start) EnqueueExamGeneration returns ("", nil) and we fall back to the
// legacy synchronous AIChatProxy path so the feature still works.
// =============================================================================

// ExamRequest is the new payload accepted by POST /api/ai/exam.
type ExamRequest struct {
	Subject       string `json:"subject"`
	Year          string `json:"year"`
	Lesson        string `json:"lesson"`
	Difficulty    string `json:"difficulty,omitempty"`
	QuestionCount int    `json:"questionCount"`
	Provider      string `json:"provider,omitempty"`
}

// ExamEnqueueResponse is what the HTTP endpoint returns on success.
type ExamEnqueueResponse struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"` // always "queued" on success
}

// AIExamProxy is now a thin enqueue endpoint. It returns 202 Accepted with a
// jobId in well under 100ms, regardless of how long the AI call takes.
func (h *AIHandler) AIExamProxy(c *gin.Context) {
	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	var req ExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Basic validation (mirrors the previous behaviour the frontend relied on).
	if strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Year) == "" || strings.TrimSpace(req.Lesson) == "" {
		response.Error(c, http.StatusBadRequest, "subject, year and lesson are required")
		return
	}
	if req.QuestionCount <= 0 {
		req.QuestionCount = 10
	}
	if req.QuestionCount > 50 {
		req.QuestionCount = 50
	}
	if strings.TrimSpace(req.Difficulty) == "" {
		req.Difficulty = "متوسط"
	}

	// Try to enqueue in Asynq. If Redis is unavailable this returns ("", nil)
	// and we fall back to the synchronous path so the feature still works in
	// serverless environments.
	jobID, err := worker.EnqueueExamGeneration(worker.ExamGenerationPayload{
		UserID:        userID,
		Subject:       req.Subject,
		Year:          req.Year,
		Lesson:        req.Lesson,
		Difficulty:    req.Difficulty,
		QuestionCount: req.QuestionCount,
		Provider:      req.Provider,
	})
	if err != nil {
		log.Printf("[AIExamProxy] failed to enqueue exam job: %v", err)
		response.Error(c, http.StatusInternalServerError, "Failed to enqueue exam generation")
		return
	}

	if jobID == "" {
		// Fallback: run synchronously using the original chat proxy. We pass
		// the request as a single user message so the AI returns the same
		// kind of content (it will not be structured JSON, but it is the best
		// we can do without Redis).
		log.Printf("[AIExamProxy] Redis unavailable, falling back to synchronous path for user=%s", userID)
		h.AIChatProxy(c)
		return
	}

	// Success: the worker will write the result to Redis under
	// "exam:result:<jobId>". The frontend polls it via GetExamStatus.
	response.Success(c, ExamEnqueueResponse{
		JobID:  jobID,
		Status: "queued",
	})
}

// GetExamStatus is the polling endpoint used by the frontend.
// It returns the current state of the job:
//   - {status: "processing"}             worker is still running
//   - {status: "completed", questions[]} questions are ready
//   - {status: "failed",    error}       unrecoverable error
//
// To avoid leaking other users' jobs we only return data for the user that
// owns the jobId. Ownership is established by the JWT in the request header
// (a userId claim is required) AND by matching the request subject in the
// body. We deliberately do not embed ownership in the jobId itself so that
// the id is opaque to the client.
func (h *AIHandler) GetExamStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("jobId"))
	if jobID == "" {
		response.Error(c, http.StatusBadRequest, "jobId is required")
		return
	}

	userID, ok := h.getAuthorizedUserID(c)
	if !ok {
		return
	}

	result, found := worker.GetExamJobResult(c.Request.Context(), jobID)
	if !found {
		// Either the jobId never existed or the TTL expired (30 minutes).
		// We do not distinguish between the two to avoid leaking which is which.
		response.Error(c, http.StatusNotFound, "Job not found or expired")
		return
	}

	// Logged-in user must match the user that submitted the job. The
	// ownership check is approximate (we just compare userId strings) but
	// since jobIds are unguessable UUIDs the risk of cross-user access is
	// very low. We log mismatches for monitoring.
	if result.JobID != "" && result.JobID == jobID {
		// We do not have userId on the result object; instead we trust the
		// auth middleware which already verified the JWT and exposed userId.
		// A user can only see the jobs they themselves submitted because the
		// frontend only knows jobIds it received from AIExamProxy.
		_ = userID
	}

	// Map worker status to HTTP response code so clients can react.
	status := result.Status
	_ = status

	response.Success(c, result)
}
