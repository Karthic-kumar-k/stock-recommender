package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiProvider implements the Provider interface for Google Gemini.
type GeminiProvider struct {
	apiKey string
	model  string
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
	}
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// IsAvailable checks if Gemini is available.
func (p *GeminiProvider) IsAvailable(ctx context.Context) bool {
	client, err := genai.NewClient(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		return false
	}
	defer client.Close()
	return true
}

// AnalyzeStock analyzes a stock using Gemini.
func (p *GeminiProvider) AnalyzeStock(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
	prompt := buildStockAnalysisPrompt(req)

	response, err := p.generate(ctx, prompt)
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

// AnalyzeSentiment analyzes sentiment using Gemini.
func (p *GeminiProvider) AnalyzeSentiment(ctx context.Context, req SentimentRequest) (*SentimentResponse, error) {
	prompt := buildSentimentPrompt(req)

	response, err := p.generate(ctx, prompt)
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

// generate sends a prompt to Gemini and returns the response.
func (p *GeminiProvider) generate(ctx context.Context, prompt string) (string, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(p.model)
	model.SetTemperature(0.7)
	model.SetMaxOutputTokens(2000)

	// Set system instruction
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text("You are a professional Indian stock market analyst. Always respond with valid JSON only."),
		},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	// Extract text from response
	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result += string(text)
		}
	}

	return result, nil
}

