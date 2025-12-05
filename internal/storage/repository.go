package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Repository provides database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new repository with the given DSN.
func NewRepository(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&Stock{},
		&StockFundamental{},
		&News{},
		&Recommendation{},
		&MarketCondition{},
		&ScreenerUpload{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Repository{db: db}, nil
}

// DB returns the underlying GORM database instance.
func (r *Repository) DB() *gorm.DB {
	return r.db
}

// Close closes the database connection.
func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Stock operations

// CreateStock creates a new stock.
func (r *Repository) CreateStock(ctx context.Context, stock *Stock) error {
	return r.db.WithContext(ctx).Create(stock).Error
}

// GetStockBySymbol retrieves a stock by its symbol.
func (r *Repository) GetStockBySymbol(ctx context.Context, symbol string) (*Stock, error) {
	var stock Stock
	err := r.db.WithContext(ctx).Where("symbol = ?", symbol).First(&stock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &stock, err
}

// GetStockByID retrieves a stock by its ID.
func (r *Repository) GetStockByID(ctx context.Context, id uint) (*Stock, error) {
	var stock Stock
	err := r.db.WithContext(ctx).First(&stock, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &stock, err
}

// GetOrCreateStock gets or creates a stock by symbol.
func (r *Repository) GetOrCreateStock(ctx context.Context, symbol, name, exchange string) (*Stock, error) {
	stock, err := r.GetStockBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock != nil {
		return stock, nil
	}

	stock = &Stock{
		Symbol:   symbol,
		Name:     name,
		Exchange: exchange,
	}
	if err := r.CreateStock(ctx, stock); err != nil {
		return nil, err
	}
	return stock, nil
}

// ListStocks lists all stocks with optional filtering.
func (r *Repository) ListStocks(ctx context.Context, limit, offset int) ([]Stock, error) {
	var stocks []Stock
	query := r.db.WithContext(ctx)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Order("symbol ASC").Find(&stocks).Error
	return stocks, err
}

// UpdateStock updates a stock.
func (r *Repository) UpdateStock(ctx context.Context, stock *Stock) error {
	return r.db.WithContext(ctx).Save(stock).Error
}

// StockFundamental operations

// CreateFundamental creates a new stock fundamental record.
func (r *Repository) CreateFundamental(ctx context.Context, fundamental *StockFundamental) error {
	return r.db.WithContext(ctx).Create(fundamental).Error
}

// GetLatestFundamental retrieves the latest fundamental data for a stock.
func (r *Repository) GetLatestFundamental(ctx context.Context, stockID uint) (*StockFundamental, error) {
	var fundamental StockFundamental
	err := r.db.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("fetched_at DESC").
		First(&fundamental).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &fundamental, err
}

// News operations

// CreateNews creates a new news article.
func (r *Repository) CreateNews(ctx context.Context, news *News) error {
	return r.db.WithContext(ctx).Create(news).Error
}

// GetNewsByURL retrieves news by URL.
func (r *Repository) GetNewsByURL(ctx context.Context, url string) (*News, error) {
	var news News
	err := r.db.WithContext(ctx).Where("url = ?", url).First(&news).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &news, err
}

// ListRecentNews lists recent news articles.
func (r *Repository) ListRecentNews(ctx context.Context, limit int, since time.Time) ([]News, error) {
	var news []News
	query := r.db.WithContext(ctx).
		Where("published_at > ?", since).
		Order("published_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&news).Error
	return news, err
}

// ListUnanalyzedNews lists news that haven't been analyzed yet.
func (r *Repository) ListUnanalyzedNews(ctx context.Context, limit int) ([]News, error) {
	var news []News
	query := r.db.WithContext(ctx).
		Where("analyzed = ?", false).
		Order("published_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&news).Error
	return news, err
}

// UpdateNews updates a news article.
func (r *Repository) UpdateNews(ctx context.Context, news *News) error {
	return r.db.WithContext(ctx).Save(news).Error
}

// ListNewsByStockID lists news for a specific stock.
func (r *Repository) ListNewsByStockID(ctx context.Context, stockID uint, limit int) ([]News, error) {
	var news []News
	query := r.db.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("published_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&news).Error
	return news, err
}

// Recommendation operations

// CreateRecommendation creates a new recommendation.
func (r *Repository) CreateRecommendation(ctx context.Context, rec *Recommendation) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

// GetRecommendationByID retrieves a recommendation by ID.
func (r *Repository) GetRecommendationByID(ctx context.Context, id uint) (*Recommendation, error) {
	var rec Recommendation
	err := r.db.WithContext(ctx).Preload("Stock").First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &rec, err
}

// ListRecommendations lists recommendations with optional filters.
func (r *Repository) ListRecommendations(ctx context.Context, activeOnly bool, action Action, limit, offset int) ([]Recommendation, error) {
	var recs []Recommendation
	query := r.db.WithContext(ctx).Preload("Stock")

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&recs).Error
	return recs, err
}

// GetLatestRecommendationForStock gets the latest recommendation for a stock.
func (r *Repository) GetLatestRecommendationForStock(ctx context.Context, stockID uint) (*Recommendation, error) {
	var rec Recommendation
	err := r.db.WithContext(ctx).
		Preload("Stock").
		Where("stock_id = ?", stockID).
		Order("created_at DESC").
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &rec, err
}

// UpdateRecommendation updates a recommendation.
func (r *Repository) UpdateRecommendation(ctx context.Context, rec *Recommendation) error {
	return r.db.WithContext(ctx).Save(rec).Error
}

// DeactivateOldRecommendations deactivates recommendations older than the given duration.
func (r *Repository) DeactivateOldRecommendations(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Model(&Recommendation{}).
		Where("created_at < ? AND is_active = ?", cutoff, true).
		Update("is_active", false).Error
}

// MarketCondition operations

// CreateMarketCondition creates a new market condition record.
func (r *Repository) CreateMarketCondition(ctx context.Context, mc *MarketCondition) error {
	return r.db.WithContext(ctx).Create(mc).Error
}

// GetLatestMarketCondition gets the latest market condition for an index.
func (r *Repository) GetLatestMarketCondition(ctx context.Context, indexName string) (*MarketCondition, error) {
	var mc MarketCondition
	err := r.db.WithContext(ctx).
		Where("index_name = ?", indexName).
		Order("recorded_at DESC").
		First(&mc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &mc, err
}

// ScreenerUpload operations

// CreateScreenerUpload creates a new screener upload record.
func (r *Repository) CreateScreenerUpload(ctx context.Context, upload *ScreenerUpload) error {
	return r.db.WithContext(ctx).Create(upload).Error
}

// UpdateScreenerUpload updates a screener upload record.
func (r *Repository) UpdateScreenerUpload(ctx context.Context, upload *ScreenerUpload) error {
	return r.db.WithContext(ctx).Save(upload).Error
}

// ListScreenerUploads lists screener uploads.
func (r *Repository) ListScreenerUploads(ctx context.Context, limit int) ([]ScreenerUpload, error) {
	var uploads []ScreenerUpload
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&uploads).Error
	return uploads, err
}

