package worker

import "time"

// =============================================================================
// Exam generation background job — shared constants and types
// =============================================================================

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
