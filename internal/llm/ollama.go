package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// OllamaRequest represents a request to Ollama API.
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse represents a response from Ollama API.
type OllamaResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// IsAvailable checks if Ollama is available.
func (p *OllamaProvider) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// AnalyzeStock analyzes a stock using Ollama.
func (p *OllamaProvider) AnalyzeStock(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
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

// AnalyzeSentiment analyzes sentiment using Ollama.
func (p *OllamaProvider) AnalyzeSentiment(ctx context.Context, req SentimentRequest) (*SentimentResponse, error) {
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

// generate sends a prompt to Ollama and returns the response.
func (p *OllamaProvider) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return ollamaResp.Response, nil
}

// parseJSONResponse extracts and parses JSON from the LLM response.
func parseJSONResponse(response string, v interface{}) error {
	// Try to find JSON in the response
	response = strings.TrimSpace(response)

	// Look for JSON object
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")

	if start == -1 || end == -1 || end < start {
		return fmt.Errorf("no JSON found in response: %s", response)
	}

	jsonStr := response[start : end+1]

	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w (json: %s)", err, jsonStr)
	}

	return nil
}

