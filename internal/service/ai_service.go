package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

type TextGenerationRequest struct {
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	Prompt       string   `json:"prompt"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	Temperature  *float64 `json:"temperature,omitempty"`
	Stream       bool     `json:"stream"`
}

type TextGenerationResponse struct {
	Text string `json:"text"`
}

type ImageItem struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type,omitempty"`
}

type ImageGenerationRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Size     string `json:"size,omitempty"`
	Count    *int   `json:"count,omitempty"`
	Quality  string `json:"quality,omitempty"`
	Format   string `json:"format,omitempty"`
}

type ImageGenerationResponse struct {
	Images []ImageItem `json:"images"`
}

type AIService struct {
	config *config.Config
	http   *common.HTTPClient
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{
		config: cfg,
		http:   common.NewHTTPClient(cfg.AIWrapperTimeoutMS),
	}
}

func (s *AIService) ensureConfigured() error {
	if s.config.AIWrapperBaseURL == "" {
		return utils.InternalServerError("AI service is not configured. Set AI_WRAPPER_BASE_URL")
	}
	return nil
}

func (s *AIService) mapGoogleQuotaMessage(statusCode int, upstreamMessage string) string {
	if statusCode != 429 {
		return ""
	}
	lower := strings.ToLower(upstreamMessage)
	if strings.Contains(lower, "resource_exhausted") || strings.Contains(lower, "quota exceeded") || strings.Contains(lower, "rate-limits") {
		if strings.Contains(lower, "image") {
			return "Google free tier image generation is not available for the current key/model. Use text generation only or enable paid Google AI quota."
		}
		return "Google AI quota exceeded for the current key/model. Retry later or use a lower-cost model."
	}
	return ""
}

func (s *AIService) GenerateText(ctx context.Context, req TextGenerationRequest) (*TextGenerationResponse, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/api/v1/text", strings.TrimRight(s.config.AIWrapperBaseURL, "/"))
	body, status, err := s.http.PostJSON(ctx, reqURL, req, nil)
	if err != nil {
		return nil, utils.BadGateway("AI service is currently unavailable")
	}

	if status >= 400 {
		var errData struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errData)
		msg := errData.Error
		if msg == "" {
			msg = errData.Message
		}
		if msg == "" {
			msg = string(body)
		}

		mapped := s.mapGoogleQuotaMessage(status, msg)
		if mapped != "" {
			return nil, utils.NewApiError(status, mapped)
		}
		return nil, utils.NewApiError(status, fmt.Sprintf("AI provider request failed: %s", msg))
	}

	var resp TextGenerationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse AI text response")
	}
	return &resp, nil
}

func (s *AIService) GenerateImage(ctx context.Context, req ImageGenerationRequest) (*ImageGenerationResponse, error) {
	if err := s.ensureConfigured(); err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/api/v1/image", strings.TrimRight(s.config.AIWrapperBaseURL, "/"))
	body, status, err := s.http.PostJSON(ctx, reqURL, req, nil)
	if err != nil {
		return nil, utils.BadGateway("AI service is currently unavailable")
	}

	if status >= 400 {
		var errData struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errData)
		msg := errData.Error
		if msg == "" {
			msg = errData.Message
		}
		if msg == "" {
			msg = string(body)
		}

		mapped := s.mapGoogleQuotaMessage(status, msg)
		if mapped != "" {
			return nil, utils.NewApiError(status, mapped)
		}
		return nil, utils.NewApiError(status, fmt.Sprintf("AI provider request failed: %s", msg))
	}

	var resp ImageGenerationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, utils.InternalServerError("Failed to parse AI image response")
	}
	return &resp, nil
}
