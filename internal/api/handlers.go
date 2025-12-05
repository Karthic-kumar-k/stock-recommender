package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/stock-recommender/internal/recommender"
	"github.com/user/stock-recommender/internal/storage"
)

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// handleHealth handles the health check endpoint.
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// AnalyzeRequest represents a stock analysis request.
type AnalyzeRequest struct {
	Symbol string `json:"symbol" binding:"required"`
}

// handleAnalyzeStock handles stock analysis requests.
func (s *Server) handleAnalyzeStock(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	result, err := s.engine.AnalyzeStock(c.Request.Context(), req.Symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendation": result.Recommendation,
		"stock":          result.Stock,
		"fundamental":    result.Fundamental,
		"news_count":     len(result.News),
		"news_sentiment": result.NewsSentiment,
		"data_sources":   result.DataSources,
	})
}

// handleListRecommendations handles listing recommendations.
func (s *Server) handleListRecommendations(c *gin.Context) {
	// Parse query parameters
	activeOnly := c.DefaultQuery("active", "true") == "true"
	action := storage.Action(c.Query("action"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	recommendations, err := s.engine.GetRecommendations(c.Request.Context(), activeOnly, action, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations": recommendations,
		"count":           len(recommendations),
		"limit":           limit,
		"offset":          offset,
	})
}

// handleGetRecommendation handles getting a single recommendation.
func (s *Server) handleGetRecommendation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recommendation ID"})
		return
	}

	recommendation, err := s.engine.GetRecommendationByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if recommendation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "recommendation not found"})
		return
	}

	c.JSON(http.StatusOK, recommendation)
}

// handleScreenerUpload handles screener.in CSV uploads.
func (s *Server) handleScreenerUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	// Create upload record
	upload := &storage.ScreenerUpload{
		Filename:    header.Filename,
		Status:      "processing",
		ProcessedAt: time.Now(),
	}
	if err := s.repo.CreateScreenerUpload(c.Request.Context(), upload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload record"})
		return
	}

	// Parse CSV
	stocks, err := s.csvParser.Parse(file)
	if err != nil {
		upload.Status = "failed"
		upload.ErrorMessage = err.Error()
		s.repo.UpdateScreenerUpload(c.Request.Context(), upload)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process stocks
	created := 0
	updated := 0
	for _, parsed := range stocks {
		stock, err := s.repo.GetOrCreateStock(c.Request.Context(), parsed.Symbol, parsed.Name, "NSE")
		if err != nil {
			continue
		}

		parsed.Fundamental.StockID = stock.ID
		if err := s.repo.CreateFundamental(c.Request.Context(), parsed.Fundamental); err == nil {
			created++
		} else {
			updated++
		}
	}

	upload.Status = "completed"
	upload.RecordsCount = len(stocks)
	s.repo.UpdateScreenerUpload(c.Request.Context(), upload)

	c.JSON(http.StatusOK, gin.H{
		"message":         "CSV processed successfully",
		"total_records":   len(stocks),
		"created":         created,
		"updated":         updated,
		"upload_id":       upload.ID,
	})
}

// handleGetSupportedColumns returns supported CSV columns.
func (s *Server) handleGetSupportedColumns(c *gin.Context) {
	columns := s.csvParser.GetSupportedColumns()
	c.JSON(http.StatusOK, gin.H{"columns": columns})
}

// handleListNews handles listing news.
func (s *Server) handleListNews(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	hoursAgo, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	if limit > 200 {
		limit = 200
	}

	since := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)
	news, err := s.engine.GetRecentNews(c.Request.Context(), limit, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"news":  news,
		"count": len(news),
	})
}

// handleRefreshNews handles refreshing news from RSS feeds.
func (s *Server) handleRefreshNews(c *gin.Context) {
	count, err := s.engine.RefreshNews(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "News refreshed successfully",
		"new_articles": count,
	})
}

// handleListStocks handles listing stocks.
func (s *Server) handleListStocks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 200 {
		limit = 200
	}

	stocks, err := s.repo.ListStocks(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stocks": stocks,
		"count":  len(stocks),
	})
}

// DailyPicksRequest represents the request for daily picks with filters.
type DailyPicksRequest struct {
	MinPrice        float64  `json:"min_price"`
	MaxPrice        float64  `json:"max_price"`
	MinMarketCap    float64  `json:"min_market_cap"`
	MaxMarketCap    float64  `json:"max_market_cap"`
	MinPE           float64  `json:"min_pe"`
	MaxPE           float64  `json:"max_pe"`
	MinConfidence   float64  `json:"min_confidence"`
	RiskLevels      []string `json:"risk_levels"`
	TimeHorizons    []string `json:"time_horizons"`
	Sectors         []string `json:"sectors"`
	MinROE          float64  `json:"min_roe"`
	MaxDebtToEquity float64  `json:"max_debt_to_equity"`
}

// handleGenerateDailyPicks handles generating daily stock picks.
func (s *Server) handleGenerateDailyPicks(c *gin.Context) {
	var req DailyPicksRequest
	
	// Try to bind JSON body (optional)
	_ = c.ShouldBindJSON(&req)

	// Convert request to filter
	var filter *recommender.DailyPicksFilter
	if req.MinPrice > 0 || req.MaxPrice > 0 || req.MinMarketCap > 0 || req.MaxMarketCap > 0 ||
		req.MinPE > 0 || req.MaxPE > 0 || req.MinConfidence > 0 || len(req.RiskLevels) > 0 ||
		len(req.TimeHorizons) > 0 || len(req.Sectors) > 0 || req.MinROE > 0 || req.MaxDebtToEquity > 0 {
		filter = &recommender.DailyPicksFilter{
			MinPrice:        req.MinPrice,
			MaxPrice:        req.MaxPrice,
			MinMarketCap:    req.MinMarketCap,
			MaxMarketCap:    req.MaxMarketCap,
			MinPE:           req.MinPE,
			MaxPE:           req.MaxPE,
			MinConfidence:   req.MinConfidence,
			RiskLevels:      req.RiskLevels,
			TimeHorizons:    req.TimeHorizons,
			Sectors:         req.Sectors,
			MinROE:          req.MinROE,
			MaxDebtToEquity: req.MaxDebtToEquity,
		}
	}

	result, err := s.engine.GenerateDailyPicksWithFilter(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleGetDailyPicks returns cached daily picks or generates new ones.
func (s *Server) handleGetDailyPicks(c *gin.Context) {
	// Check for cached results
	cached, found := s.engine.GetCachedDailyPicks(c.Request.Context())
	if found {
		c.JSON(http.StatusOK, cached)
		return
	}

	// Generate fresh picks
	result, err := s.engine.GenerateDailyPicks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleGetDailyPicksFilters returns available filter options.
func (s *Server) handleGetDailyPicksFilters(c *gin.Context) {
	filters := s.engine.GetAvailableFilters()
	c.JSON(http.StatusOK, filters)
}

// handleGetStock handles getting a single stock.
func (s *Server) handleGetStock(c *gin.Context) {
	symbol := c.Param("symbol")

	stock, err := s.repo.GetStockBySymbol(c.Request.Context(), symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if stock == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stock not found"})
		return
	}

	// Get latest fundamental
	fundamental, _ := s.repo.GetLatestFundamental(c.Request.Context(), stock.ID)

	// Get latest recommendation
	recommendation, _ := s.repo.GetLatestRecommendationForStock(c.Request.Context(), stock.ID)

	// Get recent news
	news, _ := s.repo.ListNewsByStockID(c.Request.Context(), stock.ID, 10)

	c.JSON(http.StatusOK, gin.H{
		"stock":          stock,
		"fundamental":    fundamental,
		"recommendation": recommendation,
		"news":           news,
	})
}

// Web page handlers

// handleDashboard renders the main dashboard.
func (s *Server) handleDashboard(c *gin.Context) {
	recommendations, _ := s.engine.GetRecommendations(c.Request.Context(), true, "", 20, 0)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":           "Stock Recommender",
		"recommendations": recommendations,
	})
}

// handleRecommendationDetail renders the recommendation detail page.
func (s *Server) handleRecommendationDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid recommendation ID"})
		return
	}

	recommendation, err := s.engine.GetRecommendationByID(c.Request.Context(), uint(id))
	if err != nil || recommendation == nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Recommendation not found"})
		return
	}

	// Get stock fundamentals
	fundamental, _ := s.repo.GetLatestFundamental(c.Request.Context(), recommendation.StockID)

	// Get related news
	news, _ := s.repo.ListNewsByStockID(c.Request.Context(), recommendation.StockID, 10)

	c.HTML(http.StatusOK, "recommendation.html", gin.H{
		"title":          recommendation.Stock.Name + " - Recommendation",
		"recommendation": recommendation,
		"fundamental":    fundamental,
		"news":           news,
	})
}

// handleNewsPage renders the news page.
func (s *Server) handleNewsPage(c *gin.Context) {
	since := time.Now().Add(-48 * time.Hour)
	news, _ := s.engine.GetRecentNews(c.Request.Context(), 100, since)

	c.HTML(http.StatusOK, "news.html", gin.H{
		"title": "Market News",
		"news":  news,
	})
}

// handleUploadPage renders the CSV upload page.
func (s *Server) handleUploadPage(c *gin.Context) {
	columns := s.csvParser.GetSupportedColumns()

	c.HTML(http.StatusOK, "upload.html", gin.H{
		"title":   "Upload Screener Data",
		"columns": columns,
	})
}

