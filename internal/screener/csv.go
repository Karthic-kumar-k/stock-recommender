package screener

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/user/stock-recommender/internal/storage"
)

// CSVParser parses screener.in CSV exports.
type CSVParser struct{}

// NewCSVParser creates a new CSV parser.
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// ParsedStock represents a stock parsed from CSV.
type ParsedStock struct {
	Symbol      string
	Name        string
	Fundamental *storage.StockFundamental
}

// Parse parses a screener.in CSV export.
func (p *CSVParser) Parse(reader io.Reader) ([]ParsedStock, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column index map
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[normalizeColumnName(col)] = i
	}

	var stocks []ParsedStock

	// Read data rows
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		stock, err := p.parseRow(record, colIndex)
		if err != nil {
			// Skip invalid rows but continue parsing
			continue
		}

		stocks = append(stocks, *stock)
	}

	return stocks, nil
}

// parseRow parses a single CSV row.
func (p *CSVParser) parseRow(record []string, colIndex map[string]int) (*ParsedStock, error) {
	getValue := func(names ...string) string {
		for _, name := range names {
			if idx, ok := colIndex[name]; ok && idx < len(record) {
				return strings.TrimSpace(record[idx])
			}
		}
		return ""
	}

	getFloat := func(names ...string) float64 {
		s := getValue(names...)
		return parseCSVNumber(s)
	}

	symbol := getValue("symbol", "ticker", "code", "scrip")
	name := getValue("name", "company", "companyname", "stockname")

	if symbol == "" && name == "" {
		return nil, fmt.Errorf("no symbol or name found")
	}

	// If symbol is empty, try to derive from name
	if symbol == "" {
		symbol = strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	}

	fundamental := &storage.StockFundamental{
		MarketCap:         getFloat("marketcap", "mcap", "market_cap", "marketcapitalization"),
		CurrentPrice:      getFloat("currentprice", "cmp", "price", "ltp", "lastprice"),
		High52Week:        getFloat("52weekhigh", "high52", "52whigh", "yearhigh"),
		Low52Week:         getFloat("52weeklow", "low52", "52wlow", "yearlow"),
		StockPE:           getFloat("pe", "pricetoearnings", "peratio", "stockpe"),
		BookValue:         getFloat("bookvalue", "bvps", "bookvaluepershare"),
		DividendYield:     getFloat("dividendyield", "divyield", "yield"),
		ROCE:              getFloat("roce", "returnonceemployed"),
		ROE:               getFloat("roe", "returnonequity"),
		FaceValue:         getFloat("facevalue", "fv"),
		EPS:               getFloat("eps", "earningspershare"),
		DebtToEquity:      getFloat("debttoequity", "de", "debtratio"),
		PromoterHolding:   getFloat("promoterholding", "promoter", "promoterholdingpercent"),
		PledgedPercentage: getFloat("pledged", "pledgedpercent", "pledgedpercentage"),
		RevenueGrowth3Y:   getFloat("revenuegrowth3y", "salesgrowth3y", "revenue3y"),
		ProfitGrowth3Y:    getFloat("profitgrowth3y", "patgrowth3y", "profit3y"),
		PriceToBook:       getFloat("pricetobook", "pb", "pbratio"),
		PEGRatio:          getFloat("peg", "pegratio"),
		Source:            "csv_upload",
		FetchedAt:         time.Now(),
	}

	// Calculate derived metrics if not present
	if fundamental.PriceToBook == 0 && fundamental.CurrentPrice > 0 && fundamental.BookValue > 0 {
		fundamental.PriceToBook = fundamental.CurrentPrice / fundamental.BookValue
	}

	// Graham Number
	if fundamental.EPS > 0 && fundamental.BookValue > 0 {
		fundamental.GrahamNumber = sqrt(22.5 * fundamental.EPS * fundamental.BookValue)
	}

	return &ParsedStock{
		Symbol:      normalizeSymbol(symbol),
		Name:        name,
		Fundamental: fundamental,
	}, nil
}

// normalizeColumnName normalizes a column name for matching.
func normalizeColumnName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "%", "")
	return name
}

// parseCSVNumber parses a number from CSV cell.
func parseCSVNumber(s string) float64 {
	if s == "" || s == "-" || s == "N/A" || s == "NA" {
		return 0
	}

	// Remove common formatting
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "₹", "")
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSpace(s)

	// Handle multipliers
	multiplier := 1.0
	if strings.HasSuffix(s, "Cr") || strings.HasSuffix(s, "cr") {
		multiplier = 10000000 // 1 Crore = 10 million
		s = strings.TrimSuffix(strings.TrimSuffix(s, "Cr"), "cr")
	} else if strings.HasSuffix(s, "L") || strings.HasSuffix(s, "l") {
		multiplier = 100000 // 1 Lakh
		s = strings.TrimSuffix(strings.TrimSuffix(s, "L"), "l")
	} else if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		multiplier = 1000
		s = strings.TrimSuffix(strings.TrimSuffix(s, "K"), "k")
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return value * multiplier
}

// ValidateCSV validates a CSV file before parsing.
func (p *CSVParser) ValidateCSV(reader io.Reader) error {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Check for required columns
	colIndex := make(map[string]bool)
	for _, col := range header {
		colIndex[normalizeColumnName(col)] = true
	}

	// At minimum, we need a symbol or name column
	hasIdentifier := colIndex["symbol"] || colIndex["ticker"] || colIndex["code"] ||
		colIndex["name"] || colIndex["company"] || colIndex["companyname"]

	if !hasIdentifier {
		return fmt.Errorf("CSV must contain at least one of: Symbol, Ticker, Code, Name, Company")
	}

	return nil
}

// GetSupportedColumns returns a list of supported column names.
func (p *CSVParser) GetSupportedColumns() []string {
	return []string{
		"Symbol / Ticker / Code",
		"Name / Company",
		"Market Cap",
		"Current Price / CMP / LTP",
		"52 Week High",
		"52 Week Low",
		"P/E / Stock PE",
		"Book Value / BVPS",
		"Dividend Yield",
		"ROCE",
		"ROE",
		"Face Value",
		"EPS",
		"Debt to Equity",
		"Promoter Holding",
		"Pledged Percentage",
		"Revenue Growth 3Y",
		"Profit Growth 3Y",
		"Price to Book / P/B",
		"PEG Ratio",
	}
}

