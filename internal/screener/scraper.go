// Package screener provides screener.in data fetching capabilities.
package screener

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/user/stock-recommender/internal/storage"
)

// Scraper fetches stock data from screener.in
type Scraper struct {
	baseURL     string
	client      *http.Client
	scrapeDelay time.Duration
	lastRequest time.Time
	mu          sync.Mutex
}

var scraperMu sync.Mutex

// NewScraper creates a new screener scraper.
func NewScraper(baseURL string, scrapeDelay time.Duration) *Scraper {
	if scrapeDelay < 3*time.Second {
		scrapeDelay = 3 * time.Second // Minimum 3 second delay to avoid rate limiting
	}
	return &Scraper{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		scrapeDelay: scrapeDelay,
	}
}

// StockData represents scraped stock data.
type StockData struct {
	Symbol            string
	Name              string
	Sector            string
	Industry          string
	MarketCap         float64
	CurrentPrice      float64
	High52Week        float64
	Low52Week         float64
	StockPE           float64
	BookValue         float64
	DividendYield     float64
	ROCE              float64
	ROE               float64
	FaceValue         float64
	EPS               float64
	DebtToEquity      float64
	PromoterHolding   float64
	PledgedPercentage float64
	RevenueGrowth3Y   float64
	ProfitGrowth3Y    float64
	PriceToBook       float64
	IntrinsicValue    float64
	GrahamNumber      float64
	PEGRatio          float64
}

// FetchStock fetches stock data from screener.in
func (s *Scraper) FetchStock(ctx context.Context, symbol string) (*StockData, error) {
	// Normalize symbol (remove .NS or .BO suffix if present)
	symbol = normalizeSymbol(symbol)

	// Rate limiting - ensure minimum delay between requests
	s.mu.Lock()
	elapsed := time.Since(s.lastRequest)
	if elapsed < s.scrapeDelay {
		sleepTime := s.scrapeDelay - elapsed
		s.mu.Unlock()
		select {
		case <-time.After(sleepTime):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		s.mu.Lock()
	}
	s.lastRequest = time.Now()
	s.mu.Unlock()

	url := fmt.Sprintf("%s/company/%s/", s.baseURL, symbol)

	// Retry with exponential backoff
	var resp *http.Response
	var err error
	maxRetries := 3
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("failed to create request: %w", reqErr)
		}

		// Set headers to mimic browser
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err = s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page: %w", err)
		}

		// If rate limited, wait and retry
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			backoff := time.Duration(attempt+1) * 5 * time.Second
			select {
			case <-time.After(backoff):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		break
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("screener returned status %d for symbol %s", resp.StatusCode, symbol)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	data := &StockData{
		Symbol: symbol,
	}

	// Parse company name
	data.Name = strings.TrimSpace(doc.Find("h1.margin-0").First().Text())

	// Parse sector and industry from company info
	doc.Find(".company-info a").Each(func(i int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		text := strings.TrimSpace(sel.Text())
		if strings.Contains(href, "/sector/") {
			data.Sector = text
		} else if strings.Contains(href, "/industry/") {
			data.Industry = text
		}
	})

	// Parse key metrics from the ratios section
	doc.Find("#top-ratios li").Each(func(i int, sel *goquery.Selection) {
		name := strings.TrimSpace(sel.Find(".name").Text())
		valueStr := strings.TrimSpace(sel.Find(".value").Text())

		value := parseNumber(valueStr)

		switch {
		case strings.Contains(name, "Market Cap"):
			data.MarketCap = value
		case strings.Contains(name, "Current Price"):
			data.CurrentPrice = value
		case strings.Contains(name, "High / Low"):
			parts := strings.Split(valueStr, "/")
			if len(parts) == 2 {
				data.High52Week = parseNumber(strings.TrimSpace(parts[0]))
				data.Low52Week = parseNumber(strings.TrimSpace(parts[1]))
			}
		case strings.Contains(name, "Stock P/E"):
			data.StockPE = value
		case strings.Contains(name, "Book Value"):
			data.BookValue = value
		case strings.Contains(name, "Dividend Yield"):
			data.DividendYield = value
		case strings.Contains(name, "ROCE"):
			data.ROCE = value
		case strings.Contains(name, "ROE"):
			data.ROE = value
		case strings.Contains(name, "Face Value"):
			data.FaceValue = value
		}
	})

	// Parse additional data from data tables
	doc.Find("table.data-table tbody tr").Each(func(i int, sel *goquery.Selection) {
		cells := sel.Find("td")
		if cells.Length() >= 2 {
			label := strings.TrimSpace(cells.First().Text())
			valueStr := strings.TrimSpace(cells.Last().Text())
			value := parseNumber(valueStr)

			switch {
			case strings.Contains(label, "EPS"):
				data.EPS = value
			case strings.Contains(label, "Debt to equity"):
				data.DebtToEquity = value
			case strings.Contains(label, "Promoter holding"):
				data.PromoterHolding = value
			case strings.Contains(label, "Pledged"):
				data.PledgedPercentage = value
			case strings.Contains(label, "PEG Ratio"):
				data.PEGRatio = value
			}
		}
	})

	// Calculate derived metrics
	if data.CurrentPrice > 0 && data.BookValue > 0 {
		data.PriceToBook = data.CurrentPrice / data.BookValue
	}

	// Graham Number = sqrt(22.5 * EPS * Book Value)
	if data.EPS > 0 && data.BookValue > 0 {
		data.GrahamNumber = sqrt(22.5 * data.EPS * data.BookValue)
	}

	return data, nil
}

// ToFundamental converts StockData to StockFundamental model.
func (d *StockData) ToFundamental(stockID uint) *storage.StockFundamental {
	return &storage.StockFundamental{
		StockID:           stockID,
		MarketCap:         d.MarketCap,
		CurrentPrice:      d.CurrentPrice,
		High52Week:        d.High52Week,
		Low52Week:         d.Low52Week,
		StockPE:           d.StockPE,
		BookValue:         d.BookValue,
		DividendYield:     d.DividendYield,
		ROCE:              d.ROCE,
		ROE:               d.ROE,
		FaceValue:         d.FaceValue,
		EPS:               d.EPS,
		DebtToEquity:      d.DebtToEquity,
		PromoterHolding:   d.PromoterHolding,
		PledgedPercentage: d.PledgedPercentage,
		RevenueGrowth3Y:   d.RevenueGrowth3Y,
		ProfitGrowth3Y:    d.ProfitGrowth3Y,
		PriceToBook:       d.PriceToBook,
		IntrinsicValue:    d.IntrinsicValue,
		GrahamNumber:      d.GrahamNumber,
		PEGRatio:          d.PEGRatio,
		Source:            "screener_scrape",
		FetchedAt:         time.Now(),
	}
}

// normalizeSymbol removes exchange suffixes from symbol.
func normalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.TrimSuffix(symbol, ".NS")
	symbol = strings.TrimSuffix(symbol, ".BO")
	symbol = strings.TrimSuffix(symbol, ".NSE")
	symbol = strings.TrimSuffix(symbol, ".BSE")
	return symbol
}

// parseNumber extracts a number from a string.
func parseNumber(s string) float64 {
	// Remove currency symbols, commas, and percentage signs
	s = strings.ReplaceAll(s, "₹", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, "Cr.", "")
	s = strings.ReplaceAll(s, "Cr", "")
	s = strings.TrimSpace(s)

	// Handle multipliers
	multiplier := 1.0
	if strings.HasSuffix(s, "K") {
		multiplier = 1000
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "L") {
		multiplier = 100000
		s = strings.TrimSuffix(s, "L")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1000000
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1000000000
		s = strings.TrimSuffix(s, "B")
	}

	// Extract number using regex
	re := regexp.MustCompile(`[-+]?[0-9]*\.?[0-9]+`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}

	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}

	return value * multiplier
}

// sqrt calculates square root.
func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	// Newton's method
	z := x / 2
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// SearchStocks searches for stocks on screener.in
func (s *Scraper) SearchStocks(ctx context.Context, query string) ([]string, error) {
	url := fmt.Sprintf("%s/api/company/search/?q=%s", s.baseURL, query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	// Parse the response - screener returns HTML snippets
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var symbols []string
	doc.Find("a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists && strings.Contains(href, "/company/") {
			parts := strings.Split(strings.Trim(href, "/"), "/")
			if len(parts) >= 2 {
				symbols = append(symbols, parts[len(parts)-1])
			}
		}
	})

	return symbols, nil
}

