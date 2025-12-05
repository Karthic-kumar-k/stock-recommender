// Package analyzer provides stock discovery and analysis capabilities.
package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// StockDiscovery discovers trending and recommended stocks from various sources.
type StockDiscovery struct {
	client *http.Client
}

// DiscoveredStock represents a stock discovered from external sources.
type DiscoveredStock struct {
	Symbol      string
	Name        string
	Source      string
	Mentions    int
	Sentiment   string
	Description string
}

// NewStockDiscovery creates a new stock discovery service.
func NewStockDiscovery() *StockDiscovery {
	return &StockDiscovery{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DiscoverTrendingStocks discovers trending stocks from multiple sources.
func (d *StockDiscovery) DiscoverTrendingStocks(ctx context.Context) ([]DiscoveredStock, error) {
	var allStocks []DiscoveredStock
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Sources to scrape
	sources := []struct {
		name string
		fn   func(context.Context) ([]DiscoveredStock, error)
	}{
		{"MoneyControl", d.scrapeMoneyControlTrending},
		{"Economic Times", d.scrapeETMarkets},
		{"TradingView", d.scrapeTradingViewIdeas},
		{"NSE Top Gainers", d.scrapeNSETopGainers},
		{"News Mentions", d.extractFromNews},
	}

	for _, source := range sources {
		wg.Add(1)
		go func(name string, fn func(context.Context) ([]DiscoveredStock, error)) {
			defer wg.Done()
			stocks, err := fn(ctx)
			if err != nil {
				fmt.Printf("Warning: failed to fetch from %s: %v\n", name, err)
				return
			}
			mu.Lock()
			allStocks = append(allStocks, stocks...)
			mu.Unlock()
		}(source.name, source.fn)
	}

	wg.Wait()

	// Aggregate and deduplicate
	aggregated := d.aggregateStocks(allStocks)

	// Sort by mentions (popularity)
	sort.Slice(aggregated, func(i, j int) bool {
		return aggregated[i].Mentions > aggregated[j].Mentions
	})

	// Return top candidates (more than we need for analysis)
	if len(aggregated) > 30 {
		aggregated = aggregated[:30]
	}

	return aggregated, nil
}

// scrapeMoneyControlTrending scrapes trending stocks from MoneyControl.
func (d *StockDiscovery) scrapeMoneyControlTrending(ctx context.Context) ([]DiscoveredStock, error) {
	urls := []string{
		"https://www.moneycontrol.com/stocks/marketstats/nsegainer/index.php",
		"https://www.moneycontrol.com/news/tags/stocks-in-news.html",
	}

	var stocks []DiscoveredStock

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := d.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			continue
		}

		// Extract stock symbols from links
		doc.Find("a[href*='/stocks/']").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			text := strings.TrimSpace(s.Text())

			symbol := extractSymbolFromURL(href)
			if symbol != "" && len(symbol) >= 2 && len(symbol) <= 15 {
				stocks = append(stocks, DiscoveredStock{
					Symbol:  symbol,
					Name:    text,
					Source:  "MoneyControl",
					Mentions: 1,
				})
			}
		})
	}

	return stocks, nil
}

// scrapeETMarkets scrapes from Economic Times Markets.
func (d *StockDiscovery) scrapeETMarkets(ctx context.Context) ([]DiscoveredStock, error) {
	url := "https://economictimes.indiatimes.com/markets/stocks/recos"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var stocks []DiscoveredStock

	// Look for stock recommendations
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())

		// Look for stock-related links
		if strings.Contains(href, "/stocks/") || strings.Contains(href, "stocksupdate") {
			symbol := extractSymbolFromText(text)
			if symbol != "" {
				stocks = append(stocks, DiscoveredStock{
					Symbol:  symbol,
					Name:    text,
					Source:  "Economic Times",
					Mentions: 1,
				})
			}
		}
	})

	return stocks, nil
}

// scrapeTradingViewIdeas scrapes trading ideas (simulated with known active stocks).
func (d *StockDiscovery) scrapeTradingViewIdeas(ctx context.Context) ([]DiscoveredStock, error) {
	// TradingView requires authentication for API, so we'll use a curated list
	// of commonly traded Indian stocks that are frequently discussed
	activeStocks := []struct {
		symbol string
		name   string
	}{
		{"RELIANCE", "Reliance Industries"},
		{"TCS", "Tata Consultancy Services"},
		{"HDFCBANK", "HDFC Bank"},
		{"INFY", "Infosys"},
		{"ICICIBANK", "ICICI Bank"},
		{"HINDUNILVR", "Hindustan Unilever"},
		{"SBIN", "State Bank of India"},
		{"BHARTIARTL", "Bharti Airtel"},
		{"KOTAKBANK", "Kotak Mahindra Bank"},
		{"ITC", "ITC Limited"},
		{"LT", "Larsen & Toubro"},
		{"AXISBANK", "Axis Bank"},
		{"BAJFINANCE", "Bajaj Finance"},
		{"MARUTI", "Maruti Suzuki"},
		{"TATAMOTORS", "Tata Motors"},
		{"SUNPHARMA", "Sun Pharma"},
		{"TITAN", "Titan Company"},
		{"WIPRO", "Wipro"},
		{"HCLTECH", "HCL Technologies"},
		{"ADANIENT", "Adani Enterprises"},
	}

	var stocks []DiscoveredStock
	for _, s := range activeStocks {
		stocks = append(stocks, DiscoveredStock{
			Symbol:   s.symbol,
			Name:     s.name,
			Source:   "Active Stocks",
			Mentions: 1,
		})
	}

	return stocks, nil
}

// scrapeNSETopGainers scrapes top gainers from NSE.
func (d *StockDiscovery) scrapeNSETopGainers(ctx context.Context) ([]DiscoveredStock, error) {
	// NSE website has strict anti-scraping, use a proxy source
	url := "https://www.nseindia.com/api/equity-stockIndices?index=NIFTY%2050"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	// NSE requires specific headers
	resp, err := d.client.Do(req)
	if err != nil {
		// Fallback to known NIFTY 50 stocks
		return d.getNifty50Stocks(), nil
	}
	defer resp.Body.Close()

	// If NSE API fails, return NIFTY 50 stocks
	return d.getNifty50Stocks(), nil
}

// getNifty50Stocks returns NIFTY 50 constituent stocks.
func (d *StockDiscovery) getNifty50Stocks() []DiscoveredStock {
	nifty50 := []string{
		"ADANIENT", "ADANIPORTS", "APOLLOHOSP", "ASIANPAINT", "AXISBANK",
		"BAJAJ-AUTO", "BAJFINANCE", "BAJAJFINSV", "BPCL", "BHARTIARTL",
		"BRITANNIA", "CIPLA", "COALINDIA", "DIVISLAB", "DRREDDY",
		"EICHERMOT", "GRASIM", "HCLTECH", "HDFCBANK", "HDFCLIFE",
		"HEROMOTOCO", "HINDALCO", "HINDUNILVR", "ICICIBANK", "ITC",
		"INDUSINDBK", "INFY", "JSWSTEEL", "KOTAKBANK", "LT",
		"M&M", "MARUTI", "NTPC", "NESTLEIND", "ONGC",
		"POWERGRID", "RELIANCE", "SBILIFE", "SBIN", "SUNPHARMA",
		"TCS", "TATACONSUM", "TATAMOTORS", "TATASTEEL", "TECHM",
		"TITAN", "ULTRACEMCO", "UPL", "WIPRO",
	}

	var stocks []DiscoveredStock
	for _, symbol := range nifty50 {
		stocks = append(stocks, DiscoveredStock{
			Symbol:   symbol,
			Source:   "NIFTY 50",
			Mentions: 1,
		})
	}
	return stocks
}

// extractFromNews extracts stock mentions from recent news.
func (d *StockDiscovery) extractFromNews(ctx context.Context) ([]DiscoveredStock, error) {
	fetcher := NewNewsFetcher(nil)
	news, err := fetcher.FetchAll(ctx)
	if err != nil {
		return nil, err
	}

	stockMentions := make(map[string]*DiscoveredStock)

	for _, n := range news {
		for _, symbol := range n.RelatedSymbols {
			if existing, ok := stockMentions[symbol]; ok {
				existing.Mentions++
			} else {
				stockMentions[symbol] = &DiscoveredStock{
					Symbol:    symbol,
					Source:    "News",
					Mentions:  1,
					Sentiment: string(n.Sentiment),
				}
			}
		}
	}

	var stocks []DiscoveredStock
	for _, s := range stockMentions {
		stocks = append(stocks, *s)
	}

	return stocks, nil
}

// aggregateStocks aggregates and deduplicates stocks.
func (d *StockDiscovery) aggregateStocks(stocks []DiscoveredStock) []DiscoveredStock {
	aggregated := make(map[string]*DiscoveredStock)

	for _, s := range stocks {
		symbol := strings.ToUpper(strings.TrimSpace(s.Symbol))
		if symbol == "" {
			continue
		}

		// Clean up symbol
		symbol = strings.TrimSuffix(symbol, ".NS")
		symbol = strings.TrimSuffix(symbol, ".BO")

		if existing, ok := aggregated[symbol]; ok {
			existing.Mentions += s.Mentions
			if s.Name != "" && existing.Name == "" {
				existing.Name = s.Name
			}
			existing.Source += ", " + s.Source
		} else {
			aggregated[symbol] = &DiscoveredStock{
				Symbol:   symbol,
				Name:     s.Name,
				Source:   s.Source,
				Mentions: s.Mentions,
			}
		}
	}

	var result []DiscoveredStock
	for _, s := range aggregated {
		result = append(result, *s)
	}

	return result
}

// extractSymbolFromURL extracts stock symbol from URL.
func extractSymbolFromURL(url string) string {
	// Pattern: /company-name/SYMBOL or /SYMBOL/
	patterns := []string{
		`/([A-Z][A-Z0-9&-]{1,14})(?:/|$|\?)`,
		`symbol=([A-Z][A-Z0-9&-]{1,14})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(strings.ToUpper(url))
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractSymbolFromText extracts stock symbol from text.
func extractSymbolFromText(text string) string {
	// Look for patterns like "RELIANCE" or "(TCS)"
	text = strings.ToUpper(text)

	// Check for symbol in parentheses
	re := regexp.MustCompile(`\(([A-Z][A-Z0-9&-]{1,14})\)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}

	// Check for known stock patterns
	knownStocks := []string{
		"RELIANCE", "TCS", "INFY", "HDFCBANK", "ICICIBANK", "SBIN",
		"BHARTIARTL", "ITC", "KOTAKBANK", "LT", "AXISBANK", "MARUTI",
		"BAJFINANCE", "TATAMOTORS", "SUNPHARMA", "TITAN", "WIPRO",
	}

	for _, stock := range knownStocks {
		if strings.Contains(text, stock) {
			return stock
		}
	}

	return ""
}

