package llm

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// IsAvailable checks if OpenAI is available.
func (p *OpenAIProvider) IsAvailable(ctx context.Context) bool {
	// Try a simple request to check availability
	_, err := p.client.ListModels(ctx)
	return err == nil
}

// AnalyzeStock analyzes a stock using OpenAI.
func (p *OpenAIProvider) AnalyzeStock(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
	prompt := buildStockAnalysisPrompt(req)

	response, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate analysis: %w", err)
	}

	// Parse JSON response
	var analysisResp AnalysisResponse
	if err := parseJSONResponse(response, &analysisResp); err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	return &analysisResp, nil
}

// AnalyzeSentiment analyzes sentiment using OpenAI.
func (p *OpenAIProvider) AnalyzeSentiment(ctx context.Context, req SentimentRequest) (*SentimentResponse, error) {
	prompt := buildSentimentPrompt(req)

	response, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate sentiment analysis: %w", err)
	}

	// Parse JSON response
	var sentimentResp SentimentResponse
	if err := parseJSONResponse(response, &sentimentResp); err != nil {
		return nil, fmt.Errorf("failed to parse sentiment response: %w", err)
	}

	return &sentimentResp, nil
}

// complete sends a prompt to OpenAI and returns the response.
func (p *OpenAIProvider) complete(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: p.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a professional Indian stock market analyst. Always respond with valid JSON only.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			Temperature: 0.7,
			MaxTokens:   2000,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

