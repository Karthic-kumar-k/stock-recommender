// Package llm provides LLM provider interfaces and implementations.
package llm

import (
	"context"
	"fmt"

	"github.com/user/stock-recommender/pkg/config"
)

// AnalysisRequest represents a request for stock analysis.
type AnalysisRequest struct {
	Symbol         string            `json:"symbol"`
	StockName      string            `json:"stock_name"`
	CurrentPrice   float64           `json:"current_price"`
	Fundamentals   map[string]float64 `json:"fundamentals"`
	NewsHeadlines  []string          `json:"news_headlines"`
	MarketSentiment string           `json:"market_sentiment"`
}

// AnalysisResponse represents the LLM's analysis response.
type AnalysisResponse struct {
	Action          string  `json:"action"`           // BUY, SELL, HOLD
	TargetPrice     float64 `json:"target_price"`
	StopLoss        float64 `json:"stop_loss"`
	ConfidenceScore float64 `json:"confidence_score"` // 0-100
	Reasoning       string  `json:"reasoning"`
	TimeHorizon     string  `json:"time_horizon"`     // short_term, medium_term, long_term
	RiskLevel       string  `json:"risk_level"`       // low, medium, high
	KeyFactors      []string `json:"key_factors"`
}

// SentimentRequest represents a request for sentiment analysis.
type SentimentRequest struct {
	Text   string `json:"text"`
	Symbol string `json:"symbol,omitempty"`
}

// SentimentResponse represents the sentiment analysis response.
type SentimentResponse struct {
	Sentiment string  `json:"sentiment"` // BULLISH, BEARISH, NEUTRAL
	Score     float64 `json:"score"`     // -1 to 1
	Keywords  []string `json:"keywords"`
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// AnalyzeStock analyzes a stock and returns recommendations.
	AnalyzeStock(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error)

	// AnalyzeSentiment analyzes the sentiment of text.
	AnalyzeSentiment(ctx context.Context, req SentimentRequest) (*SentimentResponse, error)

	// IsAvailable checks if the provider is available.
	IsAvailable(ctx context.Context) bool
}

// NewProvider creates a new LLM provider based on configuration.
func NewProvider(cfg *config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "ollama":
		return NewOllamaProvider(cfg.Ollama.URL, cfg.Ollama.Model), nil
	case "openai":
		if cfg.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return NewOpenAIProvider(cfg.OpenAI.APIKey, cfg.OpenAI.Model), nil
	case "gemini":
		if cfg.Gemini.APIKey == "" {
			return nil, fmt.Errorf("Gemini API key is required")
		}
		return NewGeminiProvider(cfg.Gemini.APIKey, cfg.Gemini.Model), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}

// buildStockAnalysisPrompt creates the prompt for stock analysis.
func buildStockAnalysisPrompt(req AnalysisRequest) string {
	prompt := fmt.Sprintf(`You are a professional Indian stock market analyst. Analyze the following stock and provide a recommendation.

Stock: %s (%s)
Current Price: ₹%.2f

Fundamentals:
`, req.StockName, req.Symbol, req.CurrentPrice)

	for key, value := range req.Fundamentals {
		prompt += fmt.Sprintf("- %s: %.2f\n", key, value)
	}

	if len(req.NewsHeadlines) > 0 {
		prompt += "\nRecent News Headlines:\n"
		for _, headline := range req.NewsHeadlines {
			prompt += fmt.Sprintf("- %s\n", headline)
		}
	}

	if req.MarketSentiment != "" {
		prompt += fmt.Sprintf("\nOverall Market Sentiment: %s\n", req.MarketSentiment)
	}

	prompt += `
Based on the above information, provide your analysis in the following JSON format:
{
  "action": "BUY" or "SELL" or "HOLD",
  "target_price": <number>,
  "stop_loss": <number>,
  "confidence_score": <0-100>,
  "reasoning": "<detailed explanation>",
  "time_horizon": "short_term" or "medium_term" or "long_term",
  "risk_level": "low" or "medium" or "high",
  "key_factors": ["factor1", "factor2", ...]
}

Respond ONLY with the JSON, no additional text.`

	return prompt
}

// buildSentimentPrompt creates the prompt for sentiment analysis.
func buildSentimentPrompt(req SentimentRequest) string {
	prompt := fmt.Sprintf(`Analyze the sentiment of the following text related to the Indian stock market.

Text: "%s"
`, req.Text)

	if req.Symbol != "" {
		prompt += fmt.Sprintf("Stock Symbol: %s\n", req.Symbol)
	}

	prompt += `
Provide your analysis in the following JSON format:
{
  "sentiment": "BULLISH" or "BEARISH" or "NEUTRAL",
  "score": <-1 to 1, where -1 is very bearish and 1 is very bullish>,
  "keywords": ["keyword1", "keyword2", ...]
}

Respond ONLY with the JSON, no additional text.`

	return prompt
}

