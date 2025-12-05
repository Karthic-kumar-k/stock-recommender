// Package analyzer provides news fetching and analysis capabilities.
package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/user/stock-recommender/internal/sentiment"
	"github.com/user/stock-recommender/internal/storage"
)

// NewsFetcher fetches news from RSS feeds.
type NewsFetcher struct {
	parser    *gofeed.Parser
	sources   []string
	analyzer  *sentiment.Analyzer
}

// NewsSource represents a news source configuration.
type NewsSource struct {
	Name string
	URL  string
}

// DefaultNewsSources returns the default Indian financial news sources.
func DefaultNewsSources() []NewsSource {
	return []NewsSource{
		{Name: "MoneyControl", URL: "https://www.moneycontrol.com/rss/latestnews.xml"},
		{Name: "Economic Times Markets", URL: "https://economictimes.indiatimes.com/markets/rssfeeds/1977021501.cms"},
		{Name: "Economic Times Stocks", URL: "https://economictimes.indiatimes.com/markets/stocks/rssfeeds/2146842.cms"},
		{Name: "LiveMint Markets", URL: "https://www.livemint.com/rss/markets"},
		{Name: "Business Standard", URL: "https://www.business-standard.com/rss/markets-106.rss"},
	}
}

// NewNewsFetcher creates a new news fetcher.
func NewNewsFetcher(sources []string) *NewsFetcher {
	if len(sources) == 0 {
		defaultSources := DefaultNewsSources()
		for _, s := range defaultSources {
			sources = append(sources, s.URL)
		}
	}

	return &NewsFetcher{
		parser:   gofeed.NewParser(),
		sources:  sources,
		analyzer: sentiment.NewAnalyzer(),
	}
}

// FetchedNews represents a fetched news item.
type FetchedNews struct {
	Title          string
	Description    string
	Content        string
	URL            string
	Source         string
	PublishedAt    time.Time
	Sentiment      storage.SentimentScore
	SentimentScore float64
	Keywords       []string
	RelatedSymbols []string
}

// FetchAll fetches news from all configured sources.
func (f *NewsFetcher) FetchAll(ctx context.Context) ([]FetchedNews, error) {
	var allNews []FetchedNews

	for _, source := range f.sources {
		news, err := f.fetchFromSource(ctx, source)
		if err != nil {
			// Log error but continue with other sources
			fmt.Printf("Warning: failed to fetch from %s: %v\n", source, err)
			continue
		}
		allNews = append(allNews, news...)
	}

	return allNews, nil
}

// fetchFromSource fetches news from a single RSS source.
func (f *NewsFetcher) fetchFromSource(ctx context.Context, url string) ([]FetchedNews, error) {
	feed, err := f.parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	var news []FetchedNews

	for _, item := range feed.Items {
		// Parse publication time
		publishedAt := time.Now()
		if item.PublishedParsed != nil {
			publishedAt = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishedAt = *item.UpdatedParsed
		}

		// Get content
		content := item.Content
		if content == "" {
			content = item.Description
		}

		// Analyze sentiment
		textToAnalyze := item.Title + " " + item.Description
		sentimentResult := f.analyzer.Analyze(textToAnalyze)

		// Convert sentiment
		var sentimentScore storage.SentimentScore
		switch sentimentResult.Sentiment {
		case sentiment.Bullish:
			sentimentScore = storage.SentimentBullish
		case sentiment.Bearish:
			sentimentScore = storage.SentimentBearish
		default:
			sentimentScore = storage.SentimentNeutral
		}

		// Extract related stock symbols
		relatedSymbols := extractStockSymbols(item.Title + " " + item.Description)

		// Combine keywords
		keywords := append(sentimentResult.BullishKeywords, sentimentResult.BearishKeywords...)

		news = append(news, FetchedNews{
			Title:          item.Title,
			Description:    stripHTML(item.Description),
			Content:        stripHTML(content),
			URL:            item.Link,
			Source:         getSourceName(url),
			PublishedAt:    publishedAt,
			Sentiment:      sentimentScore,
			SentimentScore: sentimentResult.Score,
			Keywords:       keywords,
			RelatedSymbols: relatedSymbols,
		})
	}

	return news, nil
}

// ToNewsModel converts FetchedNews to storage.News model.
func (n *FetchedNews) ToNewsModel(stockID *uint) *storage.News {
	return &storage.News{
		StockID:        stockID,
		Title:          n.Title,
		Description:    n.Description,
		Content:        n.Content,
		URL:            n.URL,
		Source:         n.Source,
		PublishedAt:    n.PublishedAt,
		Sentiment:      n.Sentiment,
		SentimentScore: n.SentimentScore,
		Keywords:       strings.Join(n.Keywords, ","),
		Analyzed:       true,
	}
}

// getSourceName extracts a friendly name from URL.
func getSourceName(url string) string {
	switch {
	case strings.Contains(url, "moneycontrol"):
		return "MoneyControl"
	case strings.Contains(url, "economictimes"):
		return "Economic Times"
	case strings.Contains(url, "livemint"):
		return "LiveMint"
	case strings.Contains(url, "business-standard"):
		return "Business Standard"
	case strings.Contains(url, "reuters"):
		return "Reuters"
	case strings.Contains(url, "ndtv"):
		return "NDTV Profit"
	default:
		return "Unknown"
	}
}

// stripHTML removes HTML tags from text.
func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}

// extractStockSymbols extracts potential stock symbols from text.
func extractStockSymbols(text string) []string {
	// Common Indian stock patterns
	patterns := []string{
		// NSE/BSE symbols in parentheses
		`\(([A-Z]{2,10})\)`,
		// Symbols with exchange prefix
		`(?:NSE|BSE):\s*([A-Z]{2,10})`,
		// Known large-cap stocks
		`\b(RELIANCE|TCS|INFY|HDFC|ICICI|SBIN|BHARTIARTL|ITC|KOTAKBANK|LT|HCLTECH|WIPRO|AXISBANK|MARUTI|BAJFINANCE|TATASTEEL|TATAMOTORS|SUNPHARMA|NTPC|ONGC|POWERGRID|COALINDIA|ADANIENT|ADANIPORTS|ULTRACEMCO|TITAN|NESTLEIND|ASIANPAINT|BAJAJFINSV|TECHM|HINDALCO|JSWSTEEL|GRASIM|DIVISLAB|DRREDDY|CIPLA|EICHERMOT|HEROMOTOCO|BRITANNIA|HINDUNILVR|HDFCLIFE|SBILIFE|INDUSINDBK|BPCL|M&M|TATACONSUM)\b`,
	}

	symbolMap := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbol := strings.ToUpper(match[1])
				if len(symbol) >= 2 && len(symbol) <= 15 {
					symbolMap[symbol] = true
				}
			}
		}
	}

	var symbols []string
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols
}

// FilterNewsBySymbol filters news items that mention a specific symbol.
func FilterNewsBySymbol(news []FetchedNews, symbol string) []FetchedNews {
	symbol = strings.ToUpper(symbol)
	var filtered []FetchedNews

	for _, n := range news {
		// Check if symbol is in related symbols
		for _, s := range n.RelatedSymbols {
			if s == symbol {
				filtered = append(filtered, n)
				break
			}
		}

		// Also check if symbol appears in title or description
		if strings.Contains(strings.ToUpper(n.Title), symbol) ||
			strings.Contains(strings.ToUpper(n.Description), symbol) {
			// Avoid duplicates
			found := false
			for _, f := range filtered {
				if f.URL == n.URL {
					found = true
					break
				}
			}
			if !found {
				filtered = append(filtered, n)
			}
		}
	}

	return filtered
}

// CalculateOverallSentiment calculates the overall sentiment from multiple news items.
func CalculateOverallSentiment(news []FetchedNews) (storage.SentimentScore, float64) {
	if len(news) == 0 {
		return storage.SentimentNeutral, 0
	}

	var totalScore float64
	for _, n := range news {
		totalScore += n.SentimentScore
	}

	avgScore := totalScore / float64(len(news))

	var sentiment storage.SentimentScore
	switch {
	case avgScore > 0.1:
		sentiment = storage.SentimentBullish
	case avgScore < -0.1:
		sentiment = storage.SentimentBearish
	default:
		sentiment = storage.SentimentNeutral
	}

	return sentiment, avgScore
}

