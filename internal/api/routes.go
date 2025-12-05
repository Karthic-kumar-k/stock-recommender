// Package api provides the REST API server.
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/user/stock-recommender/internal/recommender"
	"github.com/user/stock-recommender/internal/screener"
	"github.com/user/stock-recommender/internal/storage"
	"github.com/user/stock-recommender/pkg/config"
)

// Server represents the API server.
type Server struct {
	router    *gin.Engine
	engine    *recommender.Engine
	repo      *storage.Repository
	csvParser *screener.CSVParser
	config    *config.Config
}

// NewServer creates a new API server.
func NewServer(engine *recommender.Engine, repo *storage.Repository, cfg *config.Config) *Server {
	s := &Server{
		engine:    engine,
		repo:      repo,
		csvParser: screener.NewCSVParser(),
		config:    cfg,
	}

	s.setupRouter()
	return s
}

// setupRouter sets up the Gin router with all routes.
func (s *Server) setupRouter() {
	if s.config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Enable CORS
	r.Use(corsMiddleware())

	// Serve static files
	r.Static("/static", "./web/static")

	// Set custom template functions
	r.SetFuncMap(templateFuncs())

	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")

	// Web routes
	r.GET("/", s.handleDashboard)
	r.GET("/recommendation/:id", s.handleRecommendationDetail)
	r.GET("/news", s.handleNewsPage)
	r.GET("/upload", s.handleUploadPage)

	// API v1 routes
	api := r.Group("/api/v1")
	{
		// Health check
		api.GET("/health", s.handleHealth)

		// Recommendations
		api.GET("/recommendations", s.handleListRecommendations)
		api.GET("/recommendations/:id", s.handleGetRecommendation)

		// Analysis
		api.POST("/analyze", s.handleAnalyzeStock)

		// Daily Picks - AI-powered stock discovery
		api.POST("/daily-picks", s.handleGenerateDailyPicks)
		api.GET("/daily-picks", s.handleGetDailyPicks)
		api.GET("/daily-picks/stream", s.handleStreamDailyPicks)
		api.GET("/daily-picks/filters", s.handleGetDailyPicksFilters)

		// Screener CSV upload
		api.POST("/screener/upload", s.handleScreenerUpload)
		api.GET("/screener/columns", s.handleGetSupportedColumns)

		// News
		api.GET("/news", s.handleListNews)
		api.POST("/news/refresh", s.handleRefreshNews)

		// Stocks
		api.GET("/stocks", s.handleListStocks)
		api.GET("/stocks/:symbol", s.handleGetStock)
	}

	s.router = r
}

// Router returns the Gin router.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Run starts the server.
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// corsMiddleware adds CORS headers.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// templateFuncs returns custom template functions.
func templateFuncs() map[string]interface{} {
	return map[string]interface{}{
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"add": func(a, b float64) float64 {
			return a + b
		},
		"split": func(s, sep string) []string {
			if s == "" {
				return []string{}
			}
			return strings.Split(s, sep)
		},
		"gt": func(a, b interface{}) bool {
			return toFloat(a) > toFloat(b)
		},
		"lt": func(a, b interface{}) bool {
			return toFloat(a) < toFloat(b)
		},
		"gte": func(a, b interface{}) bool {
			return toFloat(a) >= toFloat(b)
		},
		"lte": func(a, b interface{}) bool {
			return toFloat(a) <= toFloat(b)
		},
	}
}

// toFloat converts various numeric types to float64.
func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case uint:
		return float64(n)
	case uint64:
		return float64(n)
	case uint32:
		return float64(n)
	default:
		return 0
	}
}

