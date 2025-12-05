// Package recommender provides the core recommendation engine.
package recommender

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/user/stock-recommender/internal/analyzer"
	"github.com/user/stock-recommender/internal/llm"
	"github.com/user/stock-recommender/internal/screener"
	"github.com/user/stock-recommender/internal/sentiment"
	"github.com/user/stock-recommender/internal/storage"
	"github.com/user/stock-recommender/pkg/config"
)

// Engine is the core recommendation engine.
type Engine struct {
	repo              *storage.Repository
	llmProvider       llm.Provider
	sentimentAnalyzer *sentiment.Analyzer
	newsFetcher       *analyzer.NewsFetcher
	screenerScraper   *screener.Scraper
	config            *config.Config
}

// NewEngine creates a new recommendation engine.
func NewEngine(
	repo *storage.Repository,
	llmProvider llm.Provider,
	cfg *config.Config,
) *Engine {
	return &Engine{
		repo:              repo,
		llmProvider:       llmProvider,
		sentimentAnalyzer: sentiment.NewAnalyzer(),
		newsFetcher:       analyzer.NewNewsFetcher(cfg.News.Sources),
		screenerScraper:   screener.NewScraper(cfg.Screener.BaseURL, cfg.Screener.ScrapeDelay),
		config:            cfg,
	}
}

// AnalysisResult represents the complete analysis result.
type AnalysisResult struct {
	Stock           *storage.Stock
	Fundamental     *storage.StockFundamental
	News            []analyzer.FetchedNews
	NewsSentiment   storage.SentimentScore
	NewsScore       float64
	KeywordAnalysis *sentiment.Result
	LLMAnalysis     *llm.AnalysisResponse
	Recommendation  *storage.Recommendation
	DataSources     []string
}

// AnalyzeStock performs a complete analysis of a stock.
func (e *Engine) AnalyzeStock(ctx context.Context, symbol string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		DataSources: []string{},
	}

	// Normalize symbol
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 1. Get or create stock
	stock, err := e.repo.GetStockBySymbol(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get stock: %w", err)
	}

	if stock == nil {
		// Try to fetch from screener
		if e.config.Screener.ScrapeEnabled {
			stockData, err := e.screenerScraper.FetchStock(ctx, symbol)
			if err != nil {
				// Screener failed, create stock with minimal info and continue
				fmt.Printf("  Note: Screener fetch failed for %s: %v\n", symbol, err)
				stock = &storage.Stock{
					Symbol:   symbol,
					Name:     symbol, // Use symbol as name
					Exchange: "NSE",
				}
				if err := e.repo.CreateStock(ctx, stock); err != nil {
					return nil, fmt.Errorf("failed to create stock: %w", err)
				}
			} else {
				stock = &storage.Stock{
					Symbol:   symbol,
					Name:     stockData.Name,
					Exchange: "NSE",
					Sector:   stockData.Sector,
					Industry: stockData.Industry,
				}

				if err := e.repo.CreateStock(ctx, stock); err != nil {
					return nil, fmt.Errorf("failed to create stock: %w", err)
				}

				// Save fundamentals
				fundamental := stockData.ToFundamental(stock.ID)
				if err := e.repo.CreateFundamental(ctx, fundamental); err != nil {
					fmt.Printf("Warning: failed to save fundamentals: %v\n", err)
				} else {
					result.Fundamental = fundamental
					result.DataSources = append(result.DataSources, "screener.in")
				}
			}
		} else {
			// Create stock with minimal info
			stock = &storage.Stock{
				Symbol:   symbol,
				Name:     symbol,
				Exchange: "NSE",
			}
			if err := e.repo.CreateStock(ctx, stock); err != nil {
				return nil, fmt.Errorf("failed to create stock: %w", err)
			}
		}
	}
	result.Stock = stock

	// 2. Get latest fundamentals if not already fetched
	if result.Fundamental == nil {
		fundamental, err := e.repo.GetLatestFundamental(ctx, stock.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get fundamentals: %w", err)
		}

		if fundamental == nil && e.config.Screener.ScrapeEnabled {
			// Fetch from screener
			stockData, err := e.screenerScraper.FetchStock(ctx, symbol)
			if err == nil {
				fundamental = stockData.ToFundamental(stock.ID)
				if err := e.repo.CreateFundamental(ctx, fundamental); err != nil {
					fmt.Printf("Warning: failed to save fundamentals: %v\n", err)
				}
				result.DataSources = append(result.DataSources, "screener.in")
			}
		}
		result.Fundamental = fundamental
	}

	// 3. Fetch and analyze news
	allNews, err := e.newsFetcher.FetchAll(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to fetch news: %v\n", err)
	} else {
		// Filter news for this stock
		stockNews := analyzer.FilterNewsBySymbol(allNews, symbol)
		result.News = stockNews
		result.NewsSentiment, result.NewsScore = analyzer.CalculateOverallSentiment(stockNews)
		result.DataSources = append(result.DataSources, "news_rss")

		// Save news to database
		for _, n := range stockNews {
			existing, _ := e.repo.GetNewsByURL(ctx, n.URL)
			if existing == nil {
				newsModel := n.ToNewsModel(&stock.ID)
				if err := e.repo.CreateNews(ctx, newsModel); err != nil {
					fmt.Printf("Warning: failed to save news: %v\n", err)
				}
			}
		}
	}

	// 4. Perform keyword sentiment analysis
	if e.config.Analysis.UseKeywordSentiment {
		var textToAnalyze strings.Builder
		for _, n := range result.News {
			textToAnalyze.WriteString(n.Title)
			textToAnalyze.WriteString(" ")
			textToAnalyze.WriteString(n.Description)
			textToAnalyze.WriteString(" ")
		}
		result.KeywordAnalysis = e.sentimentAnalyzer.Analyze(textToAnalyze.String())
		result.DataSources = append(result.DataSources, "keyword_sentiment")
	}

	// 5. Perform LLM analysis
	if e.config.Analysis.UseLLM && e.llmProvider != nil {
		llmReq := e.buildLLMRequest(result)
		llmResp, err := e.llmProvider.AnalyzeStock(ctx, llmReq)
		if err != nil {
			fmt.Printf("Warning: LLM analysis failed: %v\n", err)
		} else {
			result.LLMAnalysis = llmResp
			result.DataSources = append(result.DataSources, "llm_"+e.llmProvider.Name())
		}
	}

	// 6. Generate recommendation
	recommendation := e.generateRecommendation(result)
	result.Recommendation = recommendation

	// 7. Save recommendation
	if err := e.repo.CreateRecommendation(ctx, recommendation); err != nil {
		return nil, fmt.Errorf("failed to save recommendation: %w", err)
	}

	return result, nil
}

// buildLLMRequest builds an LLM analysis request from the analysis result.
func (e *Engine) buildLLMRequest(result *AnalysisResult) llm.AnalysisRequest {
	req := llm.AnalysisRequest{
		Symbol:    result.Stock.Symbol,
		StockName: result.Stock.Name,
	}

	if result.Fundamental != nil {
		req.CurrentPrice = result.Fundamental.CurrentPrice
		req.Fundamentals = map[string]float64{
			"Market Cap (Cr)":    result.Fundamental.MarketCap,
			"P/E Ratio":          result.Fundamental.StockPE,
			"Book Value":         result.Fundamental.BookValue,
			"ROE (%)":            result.Fundamental.ROE,
			"ROCE (%)":           result.Fundamental.ROCE,
			"Dividend Yield (%)": result.Fundamental.DividendYield,
			"Debt to Equity":     result.Fundamental.DebtToEquity,
			"EPS":                result.Fundamental.EPS,
			"Promoter Holding (%)": result.Fundamental.PromoterHolding,
			"52 Week High":       result.Fundamental.High52Week,
			"52 Week Low":        result.Fundamental.Low52Week,
		}
	}

	// Add news headlines
	for _, n := range result.News {
		if len(req.NewsHeadlines) < 10 {
			req.NewsHeadlines = append(req.NewsHeadlines, n.Title)
		}
	}

	// Add market sentiment
	if result.KeywordAnalysis != nil {
		req.MarketSentiment = string(result.KeywordAnalysis.Sentiment)
	}

	return req
}

// generateRecommendation generates a recommendation from the analysis result.
func (e *Engine) generateRecommendation(result *AnalysisResult) *storage.Recommendation {
	rec := &storage.Recommendation{
		StockID:  result.Stock.ID,
		IsActive: true,
	}

	// Determine action based on available data
	if result.LLMAnalysis != nil {
		// Use LLM recommendation
		rec.Action = storage.Action(result.LLMAnalysis.Action)
		rec.TargetPrice = result.LLMAnalysis.TargetPrice
		rec.StopLoss = result.LLMAnalysis.StopLoss
		rec.ConfidenceScore = result.LLMAnalysis.ConfidenceScore
		rec.LLMReasoning = result.LLMAnalysis.Reasoning
		rec.TimeHorizon = result.LLMAnalysis.TimeHorizon
		rec.RiskLevel = result.LLMAnalysis.RiskLevel
	} else if result.KeywordAnalysis != nil {
		// Fall back to keyword analysis
		switch result.KeywordAnalysis.Sentiment {
		case sentiment.Bullish:
			rec.Action = storage.ActionBuy
		case sentiment.Bearish:
			rec.Action = storage.ActionSell
		default:
			rec.Action = storage.ActionHold
		}
		rec.ConfidenceScore = result.KeywordAnalysis.Confidence * 100
	} else {
		rec.Action = storage.ActionHold
		rec.ConfidenceScore = 0
	}

	// Set entry price from fundamentals
	if result.Fundamental != nil {
		rec.EntryPrice = result.Fundamental.CurrentPrice

		// Calculate target and stop-loss if not set by LLM
		if rec.TargetPrice == 0 {
			// Simple 10% target
			rec.TargetPrice = rec.EntryPrice * 1.10
		}
		if rec.StopLoss == 0 {
			// Simple 5% stop-loss
			rec.StopLoss = rec.EntryPrice * 0.95
		}
	}

	// Build keyword analysis summary
	if result.KeywordAnalysis != nil {
		keywordJSON, _ := json.Marshal(map[string]interface{}{
			"sentiment":        result.KeywordAnalysis.Sentiment,
			"score":            result.KeywordAnalysis.Score,
			"bullish_keywords": result.KeywordAnalysis.BullishKeywords,
			"bearish_keywords": result.KeywordAnalysis.BearishKeywords,
			"confidence":       result.KeywordAnalysis.Confidence,
		})
		rec.KeywordAnalysis = string(keywordJSON)
	}

	// Build reasoning
	rec.Reasoning = e.buildReasoning(result)

	// Set data sources
	sourcesJSON, _ := json.Marshal(result.DataSources)
	rec.DataSources = string(sourcesJSON)

	// Set default time horizon and risk level if not set
	if rec.TimeHorizon == "" {
		rec.TimeHorizon = "medium_term"
	}
	if rec.RiskLevel == "" {
		rec.RiskLevel = "medium"
	}

	// Set expiry (7 days for short-term, 30 days for medium-term, 90 days for long-term)
	var expiry time.Time
	switch rec.TimeHorizon {
	case "short_term":
		expiry = time.Now().Add(7 * 24 * time.Hour)
	case "long_term":
		expiry = time.Now().Add(90 * 24 * time.Hour)
	default:
		expiry = time.Now().Add(30 * 24 * time.Hour)
	}
	rec.ExpiresAt = &expiry

	return rec
}

// buildReasoning builds a human-readable reasoning string.
func (e *Engine) buildReasoning(result *AnalysisResult) string {
	var reasons []string

	// Add fundamental analysis
	if result.Fundamental != nil {
		f := result.Fundamental

		// P/E analysis
		if f.StockPE > 0 {
			if f.StockPE < 15 {
				reasons = append(reasons, fmt.Sprintf("Attractively valued with P/E of %.1f", f.StockPE))
			} else if f.StockPE > 40 {
				reasons = append(reasons, fmt.Sprintf("Expensive valuation with P/E of %.1f", f.StockPE))
			}
		}

		// ROE analysis
		if f.ROE > 15 {
			reasons = append(reasons, fmt.Sprintf("Strong return on equity at %.1f%%", f.ROE))
		} else if f.ROE < 10 && f.ROE > 0 {
			reasons = append(reasons, fmt.Sprintf("Below average ROE at %.1f%%", f.ROE))
		}

		// Debt analysis
		if f.DebtToEquity < 0.5 {
			reasons = append(reasons, "Low debt levels indicate financial stability")
		} else if f.DebtToEquity > 1.5 {
			reasons = append(reasons, fmt.Sprintf("High debt to equity ratio of %.2f is a concern", f.DebtToEquity))
		}

		// Promoter holding
		if f.PromoterHolding > 60 {
			reasons = append(reasons, fmt.Sprintf("Strong promoter holding at %.1f%%", f.PromoterHolding))
		} else if f.PromoterHolding < 30 {
			reasons = append(reasons, fmt.Sprintf("Low promoter holding at %.1f%% may indicate weak conviction", f.PromoterHolding))
		}

		// 52-week range analysis
		if f.CurrentPrice > 0 && f.High52Week > 0 && f.Low52Week > 0 {
			range52 := f.High52Week - f.Low52Week
			if range52 > 0 {
				positionInRange := (f.CurrentPrice - f.Low52Week) / range52 * 100
				if positionInRange < 30 {
					reasons = append(reasons, "Trading near 52-week lows, potential value opportunity")
				} else if positionInRange > 80 {
					reasons = append(reasons, "Trading near 52-week highs, may face resistance")
				}
			}
		}
	}

	// Add news sentiment
	if len(result.News) > 0 {
		switch result.NewsSentiment {
		case storage.SentimentBullish:
			reasons = append(reasons, fmt.Sprintf("Positive news sentiment based on %d recent articles", len(result.News)))
		case storage.SentimentBearish:
			reasons = append(reasons, fmt.Sprintf("Negative news sentiment based on %d recent articles", len(result.News)))
		default:
			reasons = append(reasons, fmt.Sprintf("Neutral news sentiment based on %d recent articles", len(result.News)))
		}
	}

	// Add keyword analysis
	if result.KeywordAnalysis != nil {
		if len(result.KeywordAnalysis.BullishKeywords) > 0 {
			reasons = append(reasons, fmt.Sprintf("Bullish keywords found: %s", strings.Join(result.KeywordAnalysis.BullishKeywords[:min(3, len(result.KeywordAnalysis.BullishKeywords))], ", ")))
		}
		if len(result.KeywordAnalysis.BearishKeywords) > 0 {
			reasons = append(reasons, fmt.Sprintf("Bearish keywords found: %s", strings.Join(result.KeywordAnalysis.BearishKeywords[:min(3, len(result.KeywordAnalysis.BearishKeywords))], ", ")))
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Insufficient data for detailed analysis")
	}

	return strings.Join(reasons, ". ") + "."
}

// GetRecommendations retrieves recommendations with optional filters.
func (e *Engine) GetRecommendations(ctx context.Context, activeOnly bool, action storage.Action, limit, offset int) ([]storage.Recommendation, error) {
	return e.repo.ListRecommendations(ctx, activeOnly, action, limit, offset)
}

// GetRecommendationByID retrieves a single recommendation.
func (e *Engine) GetRecommendationByID(ctx context.Context, id uint) (*storage.Recommendation, error) {
	return e.repo.GetRecommendationByID(ctx, id)
}

// RefreshNews fetches and stores latest news.
func (e *Engine) RefreshNews(ctx context.Context) (int, error) {
	news, err := e.newsFetcher.FetchAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch news: %w", err)
	}

	count := 0
	for _, n := range news {
		existing, _ := e.repo.GetNewsByURL(ctx, n.URL)
		if existing == nil {
			newsModel := n.ToNewsModel(nil)
			if err := e.repo.CreateNews(ctx, newsModel); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// GetRecentNews retrieves recent news.
func (e *Engine) GetRecentNews(ctx context.Context, limit int, since time.Time) ([]storage.News, error) {
	return e.repo.ListRecentNews(ctx, limit, since)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

