package worker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// AI prompt building and response parsing
// =============================================================================

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
	for _, q := range envelope.Questions {
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
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid questions after normalisation")
	}
	if expectedCount > 0 && len(out) > expectedCount {
		out = out[:expectedCount]
	}
	return out, nil
}

// logParseDebug is a no-op placeholder kept for future debugging hooks.
func logParseDebug(_ string) {
	// intentionally empty
}

// suppress unused-variable linter warning on logParseDebug
var _ = logParseDebug
