package aiservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"thanawy-backend/internal/application/services"
)

// GenerateContentWithMessages allows passing full message history to the AI
func (s *AIService) GenerateContentWithMessages(ctx context.Context, messages []map[string]interface{}, model string) (string, error) {
	if !s.enabled {
		return "", errors.New(errAINotEnabled)
	}

	if model == "" {
		model = "deepseek/deepseek-v4-flash:free"
	}

	// Use circuit breaker to prevent cascading failures
	service := services.GetCircuitBreakerService()

	var result string
	err := service.CallExternalAPI("ai-service-provider", func() error {
		var reply string
		var err error

		switch s.provider {
		case "openrouter", "openai":
			reply, err = s.callOpenAICompatibleWithMessages(ctx, messages, model)
		case "gemini":
			reply, err = s.callGeminiWithMessages(ctx, messages, model)
		default:
			err = fmt.Errorf("unsupported AI provider: %s", s.provider)
		}

		if err == nil {
			result = reply
		}
		return err
	})

	return result, err
}

// callAI is the unified method to call the configured AI provider
func (s *AIService) callAI(ctx context.Context, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	switch s.provider {
	case "openrouter", "openai":
		return s.callOpenAICompatible(ctx, systemPrompt, userMessage, temperature, maxTokens)
	case "gemini":
		return s.callGemini(ctx, systemPrompt, userMessage, temperature, maxTokens)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", s.provider)
	}
}

// callOpenAICompatible calls OpenAI or OpenRouter API with circuit breaker protection
func (s *AIService) callOpenAICompatible(ctx context.Context, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	// Use circuit breaker to prevent cascading failures
	service := services.GetCircuitBreakerService()

	var apiResult string
	err := service.CallExternalAPI("openai-openrouter", func() error {
		payload := map[string]interface{}{
			"model": "deepseek/deepseek-v4-flash:free", // Default for OpenRouter
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMessage},
			},
			"temperature": temperature,
			"max_tokens":  maxTokens,
			"stream":      false,
		}

		var jsonData []byte
		var err error

		jsonData, err = json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, strings.NewReader(string(jsonData)))
		if err != nil {
			return err
		}

		req.Header.Set(headerContentType, contentTypeJSON)
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		req.Header.Set("HTTP-Referer", "https://thanawy.net")
		req.Header.Set("X-Title", "Thanawy Educational Platform")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		if result.Error != nil {
			return fmt.Errorf("AI API error: %s", result.Error.Message)
		}

		if len(result.Choices) == 0 {
			return fmt.Errorf("no response from AI")
		}

		resultContent := result.Choices[0].Message.Content
		apiResult = resultContent
		return nil
	})

	return apiResult, err
}

func (s *AIService) callOpenAICompatibleWithMessages(ctx context.Context, messages []map[string]interface{}, model string) (string, error) {
	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  2000,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}

	req.Header.Set(headerContentType, contentTypeJSON)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("HTTP-Referer", "https://thanawy.net")
	req.Header.Set("X-Title", "Thanawy Educational Platform")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI API status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return result.Choices[0].Message.Content, nil
}

func (s *AIService) callGeminiWithMessages(ctx context.Context, messages []map[string]interface{}, _ string) (string, error) {
	// Simple mapping for Gemini (concatenating history)
	var prompt strings.Builder
	for _, m := range messages {
		role := m["role"].(string)
		content := m["content"].(string)
		switch role {
		case "system":
			prompt.WriteString("System: ")
			prompt.WriteString(content)
			prompt.WriteString("\n")
		case "user":
			prompt.WriteString("User: ")
			prompt.WriteString(content)
			prompt.WriteString("\n")
		default:
			prompt.WriteString("Assistant: ")
			prompt.WriteString(content)
			prompt.WriteString("\n")
		}
	}

	return s.callGemini(ctx, "You are an educational assistant.", prompt.String(), 0.7, 2000)
}

// callGemini calls Google Gemini API with circuit breaker protection
func (s *AIService) callGemini(ctx context.Context, systemPrompt, userMessage string, temperature float64, maxTokens int) (string, error) {
	// Use circuit breaker to prevent cascading failures
	service := services.GetCircuitBreakerService()

	var apiResult string
	err := service.CallExternalAPI("gemini-api", func() error {
		url := s.apiURL + "?key=" + s.apiKey

		payload := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"parts": []map[string]string{
						{"text": systemPrompt + "\n\n" + userMessage},
					},
				},
			},
			"generationConfig": map[string]interface{}{
				"temperature":     temperature,
				"maxOutputTokens": maxTokens,
			},
		}

		var jsonData []byte
		var err error

		jsonData, err = json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			return err
		}

		req.Header.Set(headerContentType, contentTypeJSON)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		if len(result.Candidates) == 0 {
			return fmt.Errorf("no response from Gemini")
		}

		resultContent := result.Candidates[0].Content.Parts[0].Text
		apiResult = resultContent
		return nil
	})

	return apiResult, err
}
