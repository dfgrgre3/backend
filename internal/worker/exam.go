package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// =============================================================================
// Exam generation background job
// =============================================================================
// This file moves the heavy AI exam generation work OFF the Gin request thread.
//
// Flow:
//   1. HTTP POST /api/ai/exam  → AIExamProxy()  (returns 202 + jobId immediately)
//   2. Asynq worker picks up the task and runs ExamGenerationHandler.ProcessTask
//   3. Result is written to Redis:  exam:result:<jobId>  (TTL 30 minutes)
//   4. Frontend polls  GET /api/ai/exam/status/:jobId  every ~1.5s
//
// If Redis is not configured (e.g. serverless cold start) the HTTP handler
// gracefully falls back to the synchronous path.

const (
	TypeExamGeneration = "exam:generate"

	// ExamJobStatusProcessing means the AI is still working.
	ExamJobStatusProcessing = "processing"
	// ExamJobStatusCompleted means questions are ready.
	ExamJobStatusCompleted = "completed"
	// ExamJobStatusFailed means something went wrong.
	ExamJobStatusFailed = "failed"

	// examResultTTL is how long the polling result lives in Redis.
	examResultTTL = 30 * time.Minute
	// examResultKeyPrefix is prepended to the jobId when reading/writing Redis.
	examResultKeyPrefix = "exam:result:"
)

// ExamGenerationPayload is the input to the background job.
type ExamGenerationPayload struct {
	JobID         string `json:"jobId"`
	UserID        string `json:"userId"`
	Subject       string `json:"subject"`
	Year          string `json:"year"`
	Lesson        string `json:"lesson"`
	Difficulty    string `json:"difficulty,omitempty"`
	QuestionCount int    `json:"questionCount"`
	Provider      string `json:"provider"`
}

// ExamQuestion is the structured question we send to the frontend.
type ExamQuestion struct {
	Question      string   `json:"question"`
	Type          string   `json:"type"` // multiple_choice | true_false | short_answer
	Options       []string `json:"options,omitempty"`
	CorrectAnswer string   `json:"correctAnswer"`
	Explanation   string   `json:"explanation"`
}

// ExamGenerationResult is what we store in Redis and return on poll.
type ExamGenerationResult struct {
	Status    string         `json:"status"`
	JobID     string         `json:"jobId"`
	Questions []ExamQuestion `json:"questions,omitempty"`
	ExamID    string         `json:"examId,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// NewExamGenerationTask creates an asynq task from the payload.
func NewExamGenerationTask(payload ExamGenerationPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeExamGeneration, data,
		asynq.MaxRetry(2),
		asynq.Timeout(180*time.Second),
	), nil
}

// EnqueueExamGeneration enqueues the job. Returns the jobId. If Redis is
// unavailable it returns ("", nil) so the caller can fall back.
func EnqueueExamGeneration(payload ExamGenerationPayload) (string, error) {
	if payload.JobID == "" {
		payload.JobID = uuid.New().String()
	}
	task, err := NewExamGenerationTask(payload)
	if err != nil {
		return "", err
	}

	cl := GetClient()
	if cl == nil {
		return "", nil // Redis unavailable → caller must run synchronously
	}

	if _, err := cl.Enqueue(task, asynq.Queue("ai_critical")); err != nil {
		return "", fmt.Errorf("enqueue exam generation task: %w", err)
	}
	return payload.JobID, nil
}

// SetExamJobResult stores the job state in Redis so the polling endpoint can
// read it. Silently no-ops if Redis is unavailable.
func SetExamJobResult(ctx context.Context, result ExamGenerationResult) {
	if db.Redis == nil {
		return
	}
	if result.JobID == "" {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ExamJob] failed to marshal result for %s: %v", result.JobID, err)
		return
	}
	if err := db.Redis.Set(ctx, examResultKeyPrefix+result.JobID, data, examResultTTL).Err(); err != nil {
		log.Printf("[ExamJob] failed to write redis result for %s: %v", result.JobID, err)
	}
}

// GetExamJobResult reads the current state of a job. Returns (nil, false) if
// the key does not exist or Redis is down.
func GetExamJobResult(ctx context.Context, jobID string) (*ExamGenerationResult, bool) {
	if db.Redis == nil || jobID == "" {
		return nil, false
	}
	raw, err := db.Redis.Get(ctx, examResultKeyPrefix+jobID).Result()
	if err != nil {
		return nil, false
	}
	var out ExamGenerationResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, false
	}
	return &out, true
}

// =============================================================================
// Handler
// =============================================================================

// ExamGenerationHandler processes exam generation tasks in the worker pool.
type ExamGenerationHandler struct {
	aiService *services.AIService
}

// NewExamGenerationHandler constructs a handler with the singleton AI service.
func NewExamGenerationHandler() *ExamGenerationHandler {
	return &ExamGenerationHandler{aiService: services.GetAIService()}
}

// ProcessTask is the asynq entrypoint. It MUST NOT block the request thread
// because it runs in the worker pool, not in the Gin server.
func (h *ExamGenerationHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p ExamGenerationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf(jsonUnmarshalFailedFmt, err, asynq.SkipRetry)
	}

	log.Printf("[ExamWorker] start job=%s user=%s subject=%s lesson=%s count=%d",
		p.JobID, p.UserID, p.Subject, p.Lesson, p.QuestionCount)

	// Tell the client we have started.
	SetExamJobResult(ctx, ExamGenerationResult{
		Status: ExamJobStatusProcessing,
		JobID:  p.JobID,
	})

	questions, err := h.generateQuestions(ctx, p)
	if err != nil {
		log.Printf("[ExamWorker] job=%s failed: %v", p.JobID, err)
		SetExamJobResult(ctx, ExamGenerationResult{
			Status: ExamJobStatusFailed,
			JobID:  p.JobID,
			Error:  err.Error(),
		})
		// Returning the error tells asynq to retry (if MaxRetry > 0). We also
		// persisted a failed status, so the client won't loop forever.
		return err
	}

	// Optionally persist a stub exam record so the user can save the result
	// later. This is best-effort — failure here does not fail the job.
	examID := ""
	if db.WriteDB() != nil {
		examID, _ = h.persistExamStub(ctx, p, questions)
	}

	SetExamJobResult(ctx, ExamGenerationResult{
		Status:    ExamJobStatusCompleted,
		JobID:     p.JobID,
		Questions: questions,
		ExamID:    examID,
	})

	log.Printf("[ExamWorker] job=%s completed with %d questions", p.JobID, len(questions))
	return nil
}

// generateQuestions calls the AI and parses the response. The provider-aware
// system prompt forces the model to return strict JSON, which we then validate.
func (h *ExamGenerationHandler) generateQuestions(ctx context.Context, p ExamGenerationPayload) ([]ExamQuestion, error) {
	difficulty := p.Difficulty
	if difficulty == "" {
		difficulty = "متوسط"
	}

	systemPrompt := "أنت معلم خبير في المناهج الدراسية. مهمتك إنشاء أسئلة امتحانية بصيغة JSON حصراً، بدون أي نص إضافي قبل أو بعد الـ JSON."
	userPrompt := buildExamPrompt(p.Subject, p.Year, p.Lesson, difficulty, p.QuestionCount)

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	model := "google/gemini-2.0-flash-001"
	if p.Provider == "openai" {
		model = "openai/gpt-4o-mini"
	}

	raw, err := h.aiService.GenerateContentWithMessages(ctx, messages, model)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	questions, err := parseExamJSON(raw, p.QuestionCount)
	if err != nil {
		return nil, fmt.Errorf("parse AI response: %w", err)
	}
	return questions, nil
}

// buildExamPrompt returns the user message that asks the model for JSON.
func buildExamPrompt(subject, year, lesson, difficulty string, count int) string {
	return fmt.Sprintf(`أنشئ %d سؤالاً امتحانياً حول "%s" للصف %s في مادة %s.
مستوى الصعوبة: %s.

أعد النتيجة **حصراً** كائن JSON صالح بالشكل التالي، بدون أي شرح أو Markdown:
{
  "questions": [
    {
      "question": "نص السؤال",
      "type": "multiple_choice" | "true_false" | "short_answer",
      "options": ["خيار 1", "خيار 2", "خيار 3", "خيار 4"],
      "correctAnswer": "الإجابة الصحيحة",
      "explanation": "شرح مختصر"
    }
  ]
}

قواعد:
- استخدم multiple_choice للأسئلة الاختيارية مع 4 خيارات.
- استخدم true_false لأسئلة الصح/الخطأ، correctAnswer يكون "صح" أو "خطأ".
- options يكون مصفوفة فارغة [] لأسئلة true_false و short_answer.
- لا تضف أي نص خارج كائن JSON.`,
		count, lesson, year, subject, difficulty)
}

// jsonArrayRegex captures a JSON object that contains a "questions" array.
var jsonArrayRegex = regexp.MustCompile(`(?s)\{.*"questions"\s*:\s*\[.*\].*\}`)

// parseExamJSON extracts the JSON block from the model output and validates it.
func parseExamJSON(raw string, expectedCount int) ([]ExamQuestion, error) {
	candidate := strings.TrimSpace(raw)
	// Strip ```json fences if the model added them.
	candidate = strings.TrimPrefix(candidate, "```json")
	candidate = strings.TrimPrefix(candidate, "```")
	candidate = strings.TrimSuffix(candidate, "```")
	candidate = strings.TrimSpace(candidate)

	// The model occasionally returns prose around the JSON. Extract the
	// first JSON object that contains a "questions" array.
	if !strings.HasPrefix(candidate, "{") {
		match := jsonArrayRegex.FindString(raw)
		if match == "" {
			return nil, fmt.Errorf("AI did not return a JSON object containing 'questions'")
		}
		candidate = match
	}

	var envelope struct {
		Questions []struct {
			Question      string   `json:"question"`
			Type          string   `json:"type"`
			Options       []string `json:"options"`
			CorrectAnswer string   `json:"correctAnswer"`
			Explanation   string   `json:"explanation"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(candidate), &envelope); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(envelope.Questions) == 0 {
		return nil, fmt.Errorf("AI returned 0 questions")
	}

	out := make([]ExamQuestion, 0, len(envelope.Questions))
	for i, q := range envelope.Questions {
		qtype := strings.ToLower(strings.TrimSpace(q.Type))
		if qtype != "multiple_choice" && qtype != "true_false" && qtype != "short_answer" {
			qtype = "short_answer"
		}
		if qtype == "multiple_choice" && len(q.Options) == 0 {
			// Defensive: skip malformed MCQ rather than send empty options
			continue
		}
		if qtype == "true_false" {
			q.CorrectAnswer = strings.TrimSpace(q.CorrectAnswer)
			if q.CorrectAnswer != "صح" && q.CorrectAnswer != "خطأ" {
				// Normalize common English variants
				lower := strings.ToLower(q.CorrectAnswer)
				if lower == "true" || lower == "correct" || lower == "yes" {
					q.CorrectAnswer = "صح"
				} else {
					q.CorrectAnswer = "خطأ"
				}
			}
		}
		out = append(out, ExamQuestion{
			Question:      strings.TrimSpace(q.Question),
			Type:          qtype,
			Options:       q.Options,
			CorrectAnswer: strings.TrimSpace(q.CorrectAnswer),
			Explanation:   strings.TrimSpace(q.Explanation),
		})
		_ = i
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid questions after normalisation")
	}
	if expectedCount > 0 && len(out) > expectedCount {
		out = out[:expectedCount]
	}
	return out, nil
}

// persistExamStub creates an Exam record and a list of Question rows so the
// user can save and re-take the generated exam later. Returns the new examID.
func (h *ExamGenerationHandler) persistExamStub(ctx context.Context, p ExamGenerationPayload, questions []ExamQuestion) (string, error) {
	if db.WriteDB() == nil {
		return "", nil
	}

	yearInt, _ := strconv.Atoi(p.Year)
	exam := models.Exam{
		SubjectID:  p.Subject, // caller passes a subject name; we still create a record for traceability
		Title:      fmt.Sprintf("امتحان %s - %s (%s)", p.Subject, p.Lesson, p.Year),
		Type:       models.ExamTypeQuiz,
		Difficulty: p.Difficulty,
		IsActive:   true,
		Duration:   len(questions) * 2, // 2 minutes per question
	}
	if yearInt > 0 {
		exam.Description = fmt.Sprintf("الصف %s", p.Year)
	}

	if err := db.WriteDB().WithContext(ctx).Create(&exam).Error; err != nil {
		return "", fmt.Errorf("create exam: %w", err)
	}

	rows := make([]models.Question, 0, len(questions))
	for _, q := range questions {
		optionsJSON, _ := json.Marshal(q.Options)
		rows = append(rows, models.Question{
			ExamID:  exam.ID,
			Text:    q.Question,
			Type:    q.Type,
			Options: string(optionsJSON),
			Answer:  q.CorrectAnswer,
		})
	}
	if err := db.WriteDB().WithContext(ctx).Create(&rows).Error; err != nil {
		return "", fmt.Errorf("create questions: %w", err)
	}
	return exam.ID, nil
}

// =============================================================================
// Env helper
// =============================================================================

// jwtSecret returns the secret used to sign JWTs in the API layer. The polling
// endpoint reuses this to verify that the caller owns the job. Exposed for
// handlers (handlers package) to call without a circular import.
func JWTSecret() []byte {
	if s := strings.TrimSpace(os.Getenv("JWT_SECRET")); s != "" {
		return []byte(s)
	}
	return []byte("thanawy-default-secret-change-me")
}
