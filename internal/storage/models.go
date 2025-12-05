// Package storage provides database models and repository functions.
package storage

import (
	"time"

	"gorm.io/gorm"
)

// Action represents the recommended action for a stock.
type Action string

const (
	ActionBuy  Action = "BUY"
	ActionSell Action = "SELL"
	ActionHold Action = "HOLD"
)

// SentimentScore represents the sentiment analysis result.
type SentimentScore string

const (
	SentimentBullish SentimentScore = "BULLISH"
	SentimentBearish SentimentScore = "BEARISH"
	SentimentNeutral SentimentScore = "NEUTRAL"
)

// Stock represents a stock entity with fundamental data.
type Stock struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Symbol    string         `gorm:"uniqueIndex;size:20;not null" json:"symbol"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Exchange  string         `gorm:"size:10;default:NSE" json:"exchange"`
	Sector    string         `gorm:"size:100" json:"sector"`
	Industry  string         `gorm:"size:100" json:"industry"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Fundamentals    []StockFundamental  `gorm:"foreignKey:StockID" json:"fundamentals,omitempty"`
	News            []News              `gorm:"foreignKey:StockID" json:"news,omitempty"`
	Recommendations []Recommendation    `gorm:"foreignKey:StockID" json:"recommendations,omitempty"`
}

// StockFundamental holds fundamental data from screener.in
type StockFundamental struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	StockID              uint           `gorm:"index;not null" json:"stock_id"`
	MarketCap            float64        `json:"market_cap"`
	CurrentPrice         float64        `json:"current_price"`
	High52Week           float64        `json:"high_52_week"`
	Low52Week            float64        `json:"low_52_week"`
	StockPE              float64        `json:"stock_pe"`
	BookValue            float64        `json:"book_value"`
	DividendYield        float64        `json:"dividend_yield"`
	ROCE                 float64        `json:"roce"`
	ROE                  float64        `json:"roe"`
	FaceValue            float64        `json:"face_value"`
	EPS                  float64        `json:"eps"`
	DebtToEquity         float64        `json:"debt_to_equity"`
	PromoterHolding      float64        `json:"promoter_holding"`
	PledgedPercentage    float64        `json:"pledged_percentage"`
	RevenueGrowth3Y      float64        `json:"revenue_growth_3y"`
	ProfitGrowth3Y       float64        `json:"profit_growth_3y"`
	PriceToBook          float64        `json:"price_to_book"`
	IntrinsicValue       float64        `json:"intrinsic_value"`
	GrahamNumber         float64        `json:"graham_number"`
	PEGRatio             float64        `json:"peg_ratio"`
	Source               string         `gorm:"size:50" json:"source"` // screener_scrape, csv_upload
	FetchedAt            time.Time      `json:"fetched_at"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// News represents a news article.
type News struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	StockID        *uint          `gorm:"index" json:"stock_id,omitempty"`
	Title          string         `gorm:"size:500;not null" json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	Content        string         `gorm:"type:text" json:"content"`
	URL            string         `gorm:"size:1000;uniqueIndex" json:"url"`
	Source         string         `gorm:"size:100" json:"source"`
	PublishedAt    time.Time      `gorm:"index" json:"published_at"`
	Sentiment      SentimentScore `gorm:"size:20" json:"sentiment"`
	SentimentScore float64        `json:"sentiment_score"` // -1 to 1
	Keywords       string         `gorm:"type:text" json:"keywords"`
	Analyzed       bool           `gorm:"default:false" json:"analyzed"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Stock *Stock `gorm:"foreignKey:StockID" json:"stock,omitempty"`
}

// Recommendation represents a stock recommendation.
type Recommendation struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	StockID         uint           `gorm:"index;not null" json:"stock_id"`
	Action          Action         `gorm:"size:10;not null" json:"action"`
	EntryPrice      float64        `json:"entry_price"`
	TargetPrice     float64        `json:"target_price"`
	StopLoss        float64        `json:"stop_loss"`
	ConfidenceScore float64        `json:"confidence_score"` // 0 to 100
	Reasoning       string         `gorm:"type:text" json:"reasoning"`
	LLMReasoning    string         `gorm:"type:text" json:"llm_reasoning"`
	KeywordAnalysis string         `gorm:"type:text" json:"keyword_analysis"`
	DataSources     string         `gorm:"type:text" json:"data_sources"` // JSON array of sources used
	TimeHorizon     string         `gorm:"size:50" json:"time_horizon"`   // short_term, medium_term, long_term
	RiskLevel       string         `gorm:"size:20" json:"risk_level"`     // low, medium, high
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Stock Stock `gorm:"foreignKey:StockID" json:"stock"`
}

// MarketCondition represents overall market conditions.
type MarketCondition struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	IndexName      string         `gorm:"size:50;not null" json:"index_name"` // NIFTY50, SENSEX
	IndexValue     float64        `json:"index_value"`
	Change         float64        `json:"change"`
	ChangePercent  float64        `json:"change_percent"`
	Sentiment      SentimentScore `gorm:"size:20" json:"sentiment"`
	VIX            float64        `json:"vix"`
	AdvanceDecline float64        `json:"advance_decline"` // ratio
	FIIActivity    float64        `json:"fii_activity"`    // net buy/sell in crores
	DIIActivity    float64        `json:"dii_activity"`    // net buy/sell in crores
	RecordedAt     time.Time      `gorm:"index" json:"recorded_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// ScreenerUpload tracks CSV uploads from screener.in
type ScreenerUpload struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Filename      string         `gorm:"size:255" json:"filename"`
	RecordsCount  int            `json:"records_count"`
	ProcessedAt   time.Time      `json:"processed_at"`
	Status        string         `gorm:"size:20" json:"status"` // pending, processing, completed, failed
	ErrorMessage  string         `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

