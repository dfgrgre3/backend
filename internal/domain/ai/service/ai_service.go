package aiservice

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

const errAINotEnabled = "AI service is not enabled"
const contentTypeJSON = "application/json"
const headerContentType = "Content-Type"

// AIService wraps the configured AI provider (OpenRouter, Gemini or OpenAI)
// and exposes the platform's AI-powered features.
//
// Its methods are split across several files in this package (all sharing
// the aiservice package and the AIService receiver), grouped by area:
// this file (construction/config/validation), ai_service_content.go
// (generic content generation/review/quiz), ai_service_recommendations.go
// (study recommendations), ai_service_risk.go (student risk analysis),
// ai_service_providers.go (provider dispatch + raw HTTP calls) and
// ai_service_logging.go (interaction logging).
type AIService struct {
	apiKey   string
	apiURL   string
	enabled  bool
	provider string // "openrouter", "gemini", "openai"
}

// safeString safely dereferences a string pointer, returning empty string if nil
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var (
	aiServiceInstance *AIService
	aiServiceOnce     sync.Once
)

// GetAIService returns a thread-safe singleton instance of AIService.
// Initialization is protected by sync.Once to prevent race conditions.
func GetAIService() *AIService {
	aiServiceOnce.Do(func() {
		provider := os.Getenv("AI_PROVIDER")
		if provider == "" {
			provider = "openrouter" // default
		}

		apiKey := ""
		apiURL := ""
		enabled := false

		switch provider {
		case "openrouter":
			apiKey = os.Getenv("OPENROUTER_API_KEY")
			apiURL = "https://openrouter.ai/api/v1/chat/completions"
			enabled = apiKey != ""
		case "gemini":
			apiKey = os.Getenv("GEMINI_API_KEY")
			apiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
			enabled = apiKey != ""
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
			apiURL = "https://api.openai.com/v1/chat/completions"
			enabled = apiKey != ""
		}

		aiServiceInstance = &AIService{
			apiKey:   apiKey,
			apiURL:   apiURL,
			enabled:  enabled,
			provider: provider,
		}
	})
	return aiServiceInstance
}

// DefaultModel returns the configured model for the active provider. Provider
// model names are not interchangeable, so callers should not hard-code a model
// from another provider (for example, a Google model when using OpenRouter).
func (s *AIService) DefaultModel(multimodal bool) string {
	switch s.provider {
	case "openrouter":
		if multimodal {
			if model := os.Getenv("OPENROUTER_VISION_MODEL"); model != "" {
				return model
			}
		}
		if model := os.Getenv("OPENROUTER_MODEL"); model != "" {
			return model
		}
		return "openrouter/free"
	case "openai":
		if model := os.Getenv("OPENAI_MODEL"); model != "" {
			return model
		}
		return "gpt-4o-mini"
	case "gemini":
		return "gemini-2.0-flash"
	default:
		return ""
	}
}

func (s *AIService) resolveModel(model string) string {
	if s.provider == "openrouter" {
		return s.DefaultModel(strings.Contains(model, "vision"))
	}
	if model == "" {
		return s.DefaultModel(false)
	}
	return model
}

// ValidateInput validates and sanitizes user input for AI requests
func ValidateAIInput(message string, maxLength int) (string, error) {
	if message == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	// Check length
	if utf8.RuneCountInString(message) > maxLength {
		return "", fmt.Errorf("message exceeds maximum length of %d characters", maxLength)
	}

	// Remove any potentially dangerous content (basic sanitization)
	sanitized := strings.TrimSpace(message)
	if len(sanitized) == 0 {
		return "", fmt.Errorf("message cannot be empty after trimming")
	}
	return sanitized, nil
}
