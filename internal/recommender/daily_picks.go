package recommender

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/user/stock-recommender/internal/analyzer"
	"github.com/user/stock-recommender/internal/storage"
)

// DailyPick represents a daily stock pick with full analysis.
type DailyPick struct {
	Rank            int                      `json:"rank"`
	Symbol          string                   `json:"symbol"`
	Name            string                   `json:"name"`
	Sector          string                   `json:"sector,omitempty"`
	Action          string                   `json:"action"`
	EntryPrice      float64                  `json:"entry_price"`
	TargetPrice     float64                  `json:"target_price"`
	StopLoss        float64                  `json:"stop_loss"`
	ConfidenceScore float64                  `json:"confidence_score"`
	Reasoning       string                   `json:"reasoning"`
	TimeHorizon     string                   `json:"time_horizon"`
	RiskLevel       string                   `json:"risk_level"`
	Sources         []string                 `json:"sources"`
	MarketCap       float64                  `json:"market_cap,omitempty"`
	PE              float64                  `json:"pe,omitempty"`
	ROE             float64                  `json:"roe,omitempty"`
	Recommendation  *storage.Recommendation  `json:"recommendation,omitempty"`
}

// DailyPicksFilter contains filter criteria for daily picks.
type DailyPicksFilter struct {
	MinPrice        float64  `json:"min_price"`
	MaxPrice        float64  `json:"max_price"`
	MinMarketCap    float64  `json:"min_market_cap"`    // In Crores
	MaxMarketCap    float64  `json:"max_market_cap"`    // In Crores
	MinPE           float64  `json:"min_pe"`
	MaxPE           float64  `json:"max_pe"`
	MinConfidence   float64  `json:"min_confidence"`    // 0-100
	RiskLevels      []string `json:"risk_levels"`       // low, medium, high
	TimeHorizons    []string `json:"time_horizons"`     // short_term, medium_term, long_term
	Sectors         []string `json:"sectors"`
	MinROE          float64  `json:"min_roe"`
	MaxDebtToEquity float64  `json:"max_debt_to_equity"`
}

// DailyPicksResult contains the daily picks analysis result.
type DailyPicksResult struct {
	GeneratedAt    time.Time    `json:"generated_at"`
	Picks          []DailyPick  `json:"picks"`
	TotalAnalyzed  int          `json:"total_analyzed"`
	MarketSentiment string      `json:"market_sentiment"`
}

// GenerateDailyPicks discovers and analyzes stocks to generate top 10 daily picks.
func (e *Engine) GenerateDailyPicks(ctx context.Context) (*DailyPicksResult, error) {
	return e.GenerateDailyPicksWithFilter(ctx, nil)
}

// GenerateDailyPicksWithFilter discovers and analyzes stocks with optional filters.
func (e *Engine) GenerateDailyPicksWithFilter(ctx context.Context, filter *DailyPicksFilter) (*DailyPicksResult, error) {
	result := &DailyPicksResult{
		GeneratedAt: time.Now(),
		Picks:       []DailyPick{},
	}

	// Step 1: Discover trending stocks from multiple sources
	fmt.Println("→ Discovering trending stocks...")
	discovery := analyzer.NewStockDiscovery()
	candidates, err := discovery.DiscoverTrendingStocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover stocks: %w", err)
	}
	fmt.Printf("  Found %d candidate stocks\n", len(candidates))

	if len(candidates) == 0 {
		return result, nil
	}

	// Step 2: Analyze each candidate (with concurrency limit)
	fmt.Println("→ Analyzing candidates...")
	
	type analysisResult struct {
		symbol   string
		name     string
		sources  string
		analysis *AnalysisResult
		err      error
	}

	results := make(chan analysisResult, len(candidates))
	sem := make(chan struct{}, 1) // Limit to 1 concurrent analysis to avoid rate limiting
	var wg sync.WaitGroup

	// Limit candidates to avoid too many requests
	maxCandidates := 15
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}

	for _, candidate := range candidates {
		wg.Add(1)
		go func(c analyzer.DiscoveredStock) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			// Skip if context is done
			select {
			case <-ctx.Done():
				return
			default:
			}

			analysis, err := e.AnalyzeStock(ctx, c.Symbol)
			results <- analysisResult{
				symbol:   c.Symbol,
				name:     c.Name,
				sources:  c.Source,
				analysis: analysis,
				err:      err,
			}
		}(candidate)
	}

	// Close results channel when all analyses complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var analyzedStocks []DailyPick
	for r := range results {
		result.TotalAnalyzed++
		
		if r.err != nil {
			fmt.Printf("  ⚠ Failed to analyze %s: %v\n", r.symbol, r.err)
			continue
		}

		if r.analysis == nil || r.analysis.Recommendation == nil {
			continue
		}

		rec := r.analysis.Recommendation
		
		// Only include BUY recommendations
		if rec.Action != storage.ActionBuy {
			continue
		}

		name := r.name
		sector := ""
		if r.analysis.Stock != nil {
			if name == "" {
				name = r.analysis.Stock.Name
			}
			sector = r.analysis.Stock.Sector
		}

		pick := DailyPick{
			Symbol:          r.symbol,
			Name:            name,
			Sector:          sector,
			Action:          string(rec.Action),
			EntryPrice:      rec.EntryPrice,
			TargetPrice:     rec.TargetPrice,
			StopLoss:        rec.StopLoss,
			ConfidenceScore: rec.ConfidenceScore,
			Reasoning:       rec.Reasoning,
			TimeHorizon:     rec.TimeHorizon,
			RiskLevel:       rec.RiskLevel,
			Sources:         []string{r.sources},
			Recommendation:  rec,
		}

		// Add fundamental data if available
		if r.analysis.Fundamental != nil {
			pick.MarketCap = r.analysis.Fundamental.MarketCap
			pick.PE = r.analysis.Fundamental.StockPE
			pick.ROE = r.analysis.Fundamental.ROE
		}

		// Add LLM reasoning if available
		if rec.LLMReasoning != "" {
			pick.Reasoning = rec.LLMReasoning
		}

		// Apply filters
		if filter != nil && !e.passesFilter(pick, r.analysis.Fundamental, filter) {
			continue
		}

		analyzedStocks = append(analyzedStocks, pick)
	}

	// Step 3: Rank by confidence score
	sort.Slice(analyzedStocks, func(i, j int) bool {
		return analyzedStocks[i].ConfidenceScore > analyzedStocks[j].ConfidenceScore
	})

	// Step 4: Take top 10
	topPicks := analyzedStocks
	if len(topPicks) > 10 {
		topPicks = topPicks[:10]
	}

	// Assign ranks
	for i := range topPicks {
		topPicks[i].Rank = i + 1
	}

	result.Picks = topPicks

	// Determine overall market sentiment
	result.MarketSentiment = e.determineMarketSentiment(analyzedStocks)

	fmt.Printf("  ✓ Generated %d daily picks\n", len(result.Picks))

	return result, nil
}

// determineMarketSentiment determines overall market sentiment from analyzed stocks.
func (e *Engine) determineMarketSentiment(picks []DailyPick) string {
	if len(picks) == 0 {
		return "NEUTRAL"
	}

	var totalConfidence float64
	buyCount := 0

	for _, p := range picks {
		totalConfidence += p.ConfidenceScore
		if p.Action == "BUY" {
			buyCount++
		}
	}

	avgConfidence := totalConfidence / float64(len(picks))
	buyRatio := float64(buyCount) / float64(len(picks))

	if avgConfidence > 60 && buyRatio > 0.5 {
		return "BULLISH"
	} else if avgConfidence < 40 || buyRatio < 0.3 {
		return "BEARISH"
	}
	return "NEUTRAL"
}

// GetCachedDailyPicks returns cached daily picks if available and fresh.
func (e *Engine) GetCachedDailyPicks(ctx context.Context) (*DailyPicksResult, bool) {
	// For now, we don't cache - always generate fresh
	// In production, you'd want to cache results for a few hours
	return nil, false
}

// passesFilter checks if a pick passes all filter criteria.
func (e *Engine) passesFilter(pick DailyPick, fundamental *storage.StockFundamental, filter *DailyPicksFilter) bool {
	// Price filter
	if filter.MinPrice > 0 && pick.EntryPrice < filter.MinPrice {
		return false
	}
	if filter.MaxPrice > 0 && pick.EntryPrice > filter.MaxPrice {
		return false
	}

	// Confidence filter
	if filter.MinConfidence > 0 && pick.ConfidenceScore < filter.MinConfidence {
		return false
	}

	// Risk level filter
	if len(filter.RiskLevels) > 0 && !containsString(filter.RiskLevels, pick.RiskLevel) {
		return false
	}

	// Time horizon filter
	if len(filter.TimeHorizons) > 0 && !containsString(filter.TimeHorizons, pick.TimeHorizon) {
		return false
	}

	// Sector filter
	if len(filter.Sectors) > 0 && pick.Sector != "" && !containsStringInsensitive(filter.Sectors, pick.Sector) {
		return false
	}

	// Fundamental filters (only if fundamental data available)
	if fundamental != nil {
		// Market cap filter (in Crores)
		if filter.MinMarketCap > 0 && fundamental.MarketCap < filter.MinMarketCap {
			return false
		}
		if filter.MaxMarketCap > 0 && fundamental.MarketCap > filter.MaxMarketCap {
			return false
		}

		// P/E filter
		if filter.MinPE > 0 && fundamental.StockPE > 0 && fundamental.StockPE < filter.MinPE {
			return false
		}
		if filter.MaxPE > 0 && fundamental.StockPE > 0 && fundamental.StockPE > filter.MaxPE {
			return false
		}

		// ROE filter
		if filter.MinROE > 0 && fundamental.ROE < filter.MinROE {
			return false
		}

		// Debt to Equity filter
		if filter.MaxDebtToEquity > 0 && fundamental.DebtToEquity > filter.MaxDebtToEquity {
			return false
		}
	}

	return true
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// containsStringInsensitive checks if a slice contains a string (case-insensitive).
func containsStringInsensitive(slice []string, s string) bool {
	s = strings.ToLower(s)
	for _, item := range slice {
		if strings.ToLower(item) == s || strings.Contains(strings.ToLower(s), strings.ToLower(item)) {
			return true
		}
	}
	return false
}

// GetAvailableFilters returns the available filter options.
func (e *Engine) GetAvailableFilters() map[string]interface{} {
	return map[string]interface{}{
		"risk_levels":   []string{"low", "medium", "high"},
		"time_horizons": []string{"short_term", "medium_term", "long_term"},
		"sectors": []string{
			"Technology", "Financial Services", "Healthcare", "Consumer Goods",
			"Automobile", "Energy", "Metals & Mining", "Pharma", "Banking",
			"IT", "FMCG", "Telecom", "Infrastructure", "Real Estate",
		},
		"price_ranges": []map[string]interface{}{
			{"label": "Under ₹100", "min": 0, "max": 100},
			{"label": "₹100 - ₹500", "min": 100, "max": 500},
			{"label": "₹500 - ₹1000", "min": 500, "max": 1000},
			{"label": "₹1000 - ₹5000", "min": 1000, "max": 5000},
			{"label": "Above ₹5000", "min": 5000, "max": 0},
		},
		"market_cap_ranges": []map[string]interface{}{
			{"label": "Small Cap (< ₹5,000 Cr)", "min": 0, "max": 5000},
			{"label": "Mid Cap (₹5,000 - ₹20,000 Cr)", "min": 5000, "max": 20000},
			{"label": "Large Cap (> ₹20,000 Cr)", "min": 20000, "max": 0},
		},
	}
}

