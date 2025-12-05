// Package main is the entry point for the stock recommender application.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/stock-recommender/internal/api"
	"github.com/user/stock-recommender/internal/llm"
	"github.com/user/stock-recommender/internal/recommender"
	"github.com/user/stock-recommender/internal/storage"
	"github.com/user/stock-recommender/pkg/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                     StockChef                            ║")
	fmt.Println("║         AI-Powered Stock Recommender for India            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Initialize database
	fmt.Println("→ Connecting to database...")
	repo, err := storage.NewRepository(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()
	fmt.Println("  ✓ Database connected")

	// Initialize LLM provider
	var llmProvider llm.Provider
	if cfg.Analysis.UseLLM {
		fmt.Printf("→ Initializing LLM provider (%s)...\n", cfg.LLM.Provider)
		llmProvider, err = llm.NewProvider(&cfg.LLM)
		if err != nil {
			log.Printf("  ⚠ Warning: Failed to initialize LLM provider: %v", err)
			log.Println("  → Continuing with keyword sentiment analysis only")
		} else {
			fmt.Printf("  ✓ LLM provider initialized (%s)\n", llmProvider.Name())
		}
	}

	// Initialize recommendation engine
	fmt.Println("→ Initializing recommendation engine...")
	engine := recommender.NewEngine(repo, llmProvider, cfg)
	fmt.Println("  ✓ Recommendation engine ready")

	// Initialize API server
	fmt.Println("→ Starting API server...")
	server := api.NewServer(engine, repo, cfg)

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		fmt.Println("\n→ Shutting down gracefully...")
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("  ✓ Server running at http://localhost%s\n", addr)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	if err := server.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
