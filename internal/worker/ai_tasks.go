package worker

// =============================================================================
// Async AI tasks: Essay Grading & Lesson Summarization
// =============================================================================
// Pattern mirrors exam.go exactly:
//   1. POST /api/ai/grade-essay  →  EnqueueEssayGrade()   → returns 202 + jobId
//   2. POST /api/ai/summarize    →  EnqueueLessonSummary() → returns 202 + jobId
//   3. Worker processes task, writes result to Redis (TTL 30 min)
//   4. Frontend polls GET /api/ai/grade-essay/status/:jobId  (or summarize/status)
//
// Graceful degradation: if Redis is unavailable the HTTP handlers fall back
// to the synchronous AIChatProxy path, so the feature still works everywhere.
// =============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/services"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ─── Shared constants ────────────────────────────────────────────────────────

const (
	// Task type identifiers (registered in server.go mux)
	TypeEssayGrade    = "ai:essay:grade"
	TypeLessonSummary = "ai:lesson:summarize"

	// Status strings (match exam.go for frontend re-use)
	AIJobStatusProcessing = "processing"
	AIJobStatusCompleted  = "completed"
	AIJobStatusFailed     = "failed"

	// Redis TTL for intermediate / final results
	aiJobResultTTL = 30 * time.Minute

	// Redis key prefixes
	essayResultKeyPrefix   = "ai:essay:result:"
	summaryResultKeyPrefix = "ai:summary:result:"
)

// AIJobResult is the generic result envelope stored in Redis and returned by
// the polling endpoints. It purposely mirrors the frontend type in
// lib/pollJobResult.ts so no translation is needed.
type AIJobResult struct {
	Status string `json:"status"`
	JobID  string `json:"jobId"`
	Result string `json:"result,omitempty"` // markdown text from the LLM
	Error  string `json:"error,omitempty"`
}

// ─── Essay grading ────────────────────────────────────────────────────────────

// EssayGradePayload is the input serialised into the asynq task.
type EssayGradePayload struct {
	JobID    string `json:"jobId"`
	UserID   string `json:"userId"`
	Content  string `json:"content"`
	Topic    string `json:"topic"`
	Language string `json:"language"`
}

// NewEssayGradeTask builds an asynq task from the payload.
func NewEssayGradeTask(p EssayGradePayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEssayGrade, data,
		asynq.MaxRetry(2),
		asynq.Timeout(120*time.Second),
	), nil
}

// EnqueueEssayGrade pushes the job onto the ai_critical queue and returns its
// jobId. Returns ("", nil) when Redis is unavailable so the caller can fall back.
func EnqueueEssayGrade(p EssayGradePayload) (string, error) {
	if p.JobID == "" {
		p.JobID = uuid.New().String()
	}
	task, err := NewEssayGradeTask(p)
	if err != nil {
		return "", err
	}

	cl := GetClient()
	if cl == nil {
		return "", nil // Redis unavailable → synchronous fallback
	}
	if _, err := cl.Enqueue(task, asynq.Queue("ai_critical")); err != nil {
		return "", fmt.Errorf("enqueue essay grade task: %w", err)
	}
	return p.JobID, nil
}

// SetEssayJobResult persists the job state to Redis.
func SetEssayJobResult(ctx context.Context, r AIJobResult) {
	setAIJobResult(ctx, essayResultKeyPrefix, r)
}

// GetEssayJobResult reads the current state from Redis.
func GetEssayJobResult(ctx context.Context, jobID string) (*AIJobResult, bool) {
	return getAIJobResult(ctx, essayResultKeyPrefix, jobID)
}

// EssayGradeHandler is the asynq worker handler registered in server.go.
type EssayGradeHandler struct {
	aiService *services.AIService
}

// NewEssayGradeHandler constructs the handler with the singleton AI service.
func NewEssayGradeHandler() *EssayGradeHandler {
	return &EssayGradeHandler{aiService: services.GetAIService()}
}

// ProcessTask is the asynq entrypoint. Runs in the worker pool, NOT in a Gin
// goroutine, so it is safe to block here for the full LLM round-trip.
func (h *EssayGradeHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p EssayGradePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf(jsonUnmarshalFailedFmt, err, asynq.SkipRetry)
	}

	log.Printf("[EssayWorker] start job=%s user=%s lang=%s", p.JobID, p.UserID, p.Language)

	SetEssayJobResult(ctx, AIJobResult{Status: AIJobStatusProcessing, JobID: p.JobID})

	result, err := h.gradeEssay(ctx, p)
	if err != nil {
		log.Printf("[EssayWorker] job=%s failed: %v", p.JobID, err)
		SetEssayJobResult(ctx, AIJobResult{
			Status: AIJobStatusFailed,
			JobID:  p.JobID,
			Error:  err.Error(),
		})
		return err
	}

	SetEssayJobResult(ctx, AIJobResult{
		Status: AIJobStatusCompleted,
		JobID:  p.JobID,
		Result: result,
	})
	log.Printf("[EssayWorker] job=%s completed", p.JobID)
	return nil
}

func (h *EssayGradeHandler) gradeEssay(ctx context.Context, p EssayGradePayload) (string, error) {
	lang := p.Language
	if lang == "" {
		lang = "Arabic"
	}
	topic := p.Topic
	if topic == "" {
		topic = "موضوع تعبير"
	}

	systemPrompt := "أنت مدرس لغات خبير. مهمتك تصحيح المواضيع الكتابية وتقديم تقييم مفصل وبناء."
	userPrompt := fmt.Sprintf(
		"صحح الموضوع التالي المكتوب باللغة %s حول \"%s\".\n\nالموضوع:\n%s\n\n"+
			"قدم تقييماً شاملاً يتضمن:\n"+
			"1. الدرجة الإجمالية من 20\n"+
			"2. نقاط القوة\n"+
			"3. نقاط الضعف والأخطاء اللغوية\n"+
			"4. اقتراحات للتحسين\n"+
			"5. ملاحظات النحو والإملاء\n\n"+
			"استخدم تنسيق Markdown.",
		lang, topic, p.Content,
	)

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	raw, err := h.aiService.GenerateContentWithMessages(ctx, messages, "google/gemini-2.0-flash-001")
	if err != nil {
		return "", fmt.Errorf("AI call failed: %w", err)
	}
	return raw, nil
}

// ─── Lesson summarization ─────────────────────────────────────────────────────

// LessonSummaryPayload is the input serialised into the asynq task.
type LessonSummaryPayload struct {
	JobID   string `json:"jobId"`
	UserID  string `json:"userId"`
	Content string `json:"content"`
}

// NewLessonSummaryTask builds an asynq task from the payload.
func NewLessonSummaryTask(p LessonSummaryPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeLessonSummary, data,
		asynq.MaxRetry(2),
		asynq.Timeout(120*time.Second),
	), nil
}

// EnqueueLessonSummary pushes the job onto the ai_critical queue.
// Returns ("", nil) when Redis is unavailable.
func EnqueueLessonSummary(p LessonSummaryPayload) (string, error) {
	if p.JobID == "" {
		p.JobID = uuid.New().String()
	}
	task, err := NewLessonSummaryTask(p)
	if err != nil {
		return "", err
	}

	cl := GetClient()
	if cl == nil {
		return "", nil // Redis unavailable → synchronous fallback
	}
	if _, err := cl.Enqueue(task, asynq.Queue("ai_critical")); err != nil {
		return "", fmt.Errorf("enqueue lesson summary task: %w", err)
	}
	return p.JobID, nil
}

// SetSummaryJobResult persists the job state to Redis.
func SetSummaryJobResult(ctx context.Context, r AIJobResult) {
	setAIJobResult(ctx, summaryResultKeyPrefix, r)
}

// GetSummaryJobResult reads the current state from Redis.
func GetSummaryJobResult(ctx context.Context, jobID string) (*AIJobResult, bool) {
	return getAIJobResult(ctx, summaryResultKeyPrefix, jobID)
}

// LessonSummaryHandler is the asynq worker handler registered in server.go.
type LessonSummaryHandler struct {
	aiService *services.AIService
}

// NewLessonSummaryHandler constructs the handler with the singleton AI service.
func NewLessonSummaryHandler() *LessonSummaryHandler {
	return &LessonSummaryHandler{aiService: services.GetAIService()}
}

// ProcessTask is the asynq entrypoint.
func (h *LessonSummaryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p LessonSummaryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf(jsonUnmarshalFailedFmt, err, asynq.SkipRetry)
	}

	log.Printf("[SummaryWorker] start job=%s user=%s", p.JobID, p.UserID)

	SetSummaryJobResult(ctx, AIJobResult{Status: AIJobStatusProcessing, JobID: p.JobID})

	result, err := h.summarize(ctx, p)
	if err != nil {
		log.Printf("[SummaryWorker] job=%s failed: %v", p.JobID, err)
		SetSummaryJobResult(ctx, AIJobResult{
			Status: AIJobStatusFailed,
			JobID:  p.JobID,
			Error:  err.Error(),
		})
		return err
	}

	SetSummaryJobResult(ctx, AIJobResult{
		Status: AIJobStatusCompleted,
		JobID:  p.JobID,
		Result: result,
	})
	log.Printf("[SummaryWorker] job=%s completed", p.JobID)
	return nil
}

func (h *LessonSummaryHandler) summarize(ctx context.Context, p LessonSummaryPayload) (string, error) {
	systemPrompt := "أنت معلم خبير. مهمتك تلخيص الدروس التعليمية بشكل واضح ومنظم."
	userPrompt := fmt.Sprintf(
		"لخص الدرس التالي بشكل شامل:\n\n%s\n\n"+
			"قدم:\n"+
			"1. ملخصاً مختصراً (5-10 نقاط رئيسية) بتنسيق Markdown\n"+
			"2. خريطة ذهنية بصيغة Mermaid (ضعها داخل ```mermaid ... ```)\n\n"+
			"استخدم اللغة العربية في الرد.",
		p.Content,
	)

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	raw, err := h.aiService.GenerateContentWithMessages(ctx, messages, "google/gemini-2.0-flash-001")
	if err != nil {
		return "", fmt.Errorf("AI call failed: %w", err)
	}
	return raw, nil
}

// ─── Shared Redis helpers ─────────────────────────────────────────────────────

func setAIJobResult(ctx context.Context, prefix string, r AIJobResult) {
	if db.Redis == nil || r.JobID == "" {
		return
	}
	data, err := json.Marshal(r)
	if err != nil {
		log.Printf("[AIWorker] marshal failed for %s%s: %v", prefix, r.JobID, err)
		return
	}
	if err := db.Redis.Set(ctx, prefix+r.JobID, data, aiJobResultTTL).Err(); err != nil {
		log.Printf("[AIWorker] redis write failed for %s%s: %v", prefix, r.JobID, err)
	}
}

func getAIJobResult(ctx context.Context, prefix, jobID string) (*AIJobResult, bool) {
	if db.Redis == nil || jobID == "" {
		return nil, false
	}
	raw, err := db.Redis.Get(ctx, prefix+jobID).Result()
	if err != nil {
		return nil, false
	}
	var out AIJobResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	return &out, true
}
