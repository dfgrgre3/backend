package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	aiservice "thanawy-backend/internal/domain/ai/service"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/hibiken/asynq"
)

// =============================================================================
// ExamGenerationHandler — asynq worker
// =============================================================================
//
// Flow:
//  1. HTTP POST /api/ai/exam  → AIExamProxy()  (returns 202 + jobId immediately)
//  2. Asynq worker picks up the task and runs ExamGenerationHandler.ProcessTask
//  3. Result is written to Redis:  exam:result:<jobId>  (TTL 30 minutes)
//  4. Frontend polls  GET /api/ai/exam/status/:jobId  every ~1.5s
//
// If Redis is not configured (e.g. serverless cold start) the HTTP handler
// gracefully falls back to the synchronous path.

// ExamGenerationHandler processes exam generation tasks in the worker pool.
type ExamGenerationHandler struct {
	aiService *aiservice.AIService
}

// NewExamGenerationHandler constructs a handler with the singleton AI service.
func NewExamGenerationHandler() *ExamGenerationHandler {
	return &ExamGenerationHandler{aiService: aiservice.GetAIService()}
}

// ProcessTask is the asynq entrypoint. It runs in the worker pool, not in the
// Gin server, so it MUST NOT block the request thread.
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
		// persisted a failed status so the client won't loop forever.
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

// resolveSubjectID looks up a real Subject by name (Arabic or English) so the
// generated Exam record can carry a valid subject_id.
func resolveSubjectID(ctx context.Context, subjectName string) (string, bool) {
	name := strings.TrimSpace(subjectName)
	if name == "" {
		return "", false
	}
	var subject models.Subject
	if err := db.WriteDB().WithContext(ctx).
		Select("id").
		Where("name = ? OR name_ar = ?", name, name).
		First(&subject).Error; err != nil {
		return "", false
	}
	return subject.ID, true
}

// persistExamStub creates an Exam record and Question rows so the user can
// save and re-take the generated exam later. Returns the new examID.
func (h *ExamGenerationHandler) persistExamStub(ctx context.Context, p ExamGenerationPayload, questions []ExamQuestion) (string, error) {
	if db.WriteDB() == nil {
		return "", nil
	}

	subjectID, ok := resolveSubjectID(ctx, p.Subject)
	if !ok {
		log.Printf("[ExamWorker] job=%s: no Subject matches name %q, skipping exam stub persistence", p.JobID, p.Subject)
		return "", nil
	}

	yearInt, _ := strconv.Atoi(p.Year)
	exam := models.Exam{
		SubjectID:  subjectID,
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

// JWTSecret returns the secret used to sign JWTs in the API layer. The polling
// endpoint reuses this to verify that the caller owns the job. Exposed for
// handlers (handlers package) to call without a circular import.
func JWTSecret() []byte {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	return []byte(secret)
}
