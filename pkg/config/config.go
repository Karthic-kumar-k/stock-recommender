// Package config provides configuration management for the stock recommender.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Server   ServerConfig   `mapstructure:"server"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Analysis AnalysisConfig `mapstructure:"analysis"`
	News     NewsConfig     `mapstructure:"news"`
	Screener ScreenerConfig `mapstructure:"screener"`
}

// AppConfig holds application-level configuration.
type AppConfig struct {
	Env      string `mapstructure:"env"`
	LogLevel string `mapstructure:"log_level"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN returns the database connection string.
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	Provider string       `mapstructure:"provider"` // ollama, openai, gemini
	Ollama   OllamaConfig `mapstructure:"ollama"`
	OpenAI   OpenAIConfig `mapstructure:"openai"`
	Gemini   GeminiConfig `mapstructure:"gemini"`
}

// OllamaConfig holds Ollama-specific configuration.
type OllamaConfig struct {
	URL   string `mapstructure:"url"`
	Model string `mapstructure:"model"`
}

// OpenAIConfig holds OpenAI-specific configuration.
type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

// GeminiConfig holds Gemini-specific configuration.
type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

// AnalysisConfig holds analysis configuration.
type AnalysisConfig struct {
	UseLLM              bool `mapstructure:"use_llm"`
	UseKeywordSentiment bool `mapstructure:"use_keyword_sentiment"`
}

// NewsConfig holds news fetching configuration.
type NewsConfig struct {
	FetchInterval time.Duration `mapstructure:"fetch_interval"`
	Sources       []string      `mapstructure:"sources"`
}

// ScreenerConfig holds screener.in configuration.
type ScreenerConfig struct {
	BaseURL       string        `mapstructure:"base_url"`
	ScrapeEnabled bool          `mapstructure:"scrape_enabled"`
	ScrapeDelay   time.Duration `mapstructure:"scrape_delay"`
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	// Load .env file if it exists (don't error if not found)
	envFiles := []string{".env", ".env.local"}
	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			if err := godotenv.Load(envFile); err != nil {
				fmt.Printf("Warning: could not load %s: %v\n", envFile, err)
			} else {
				fmt.Printf("Loaded environment from %s\n", envFile)
			}
		}
	}

	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			fmt.Printf("Warning: could not read config file: %v\n", err)
		}
	} else {
		// Look for config in default locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		if err := v.ReadInConfig(); err != nil {
			fmt.Printf("Warning: could not read config file: %v\n", err)
		}
	}

	// Read from environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables
	bindEnvVars(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values.
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.env", "development")
	v.SetDefault("app.log_level", "debug")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "stock_recommender")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")

	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")

	// LLM defaults
	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.ollama.url", "http://localhost:11434")
	v.SetDefault("llm.ollama.model", "llama2")
	v.SetDefault("llm.openai.model", "gpt-4o-mini")
	v.SetDefault("llm.gemini.model", "gemini-pro")

	// Analysis defaults
	v.SetDefault("analysis.use_llm", true)
	v.SetDefault("analysis.use_keyword_sentiment", true)

	// News defaults
	v.SetDefault("news.fetch_interval", "15m")
	v.SetDefault("news.sources", []string{
		"https://www.moneycontrol.com/rss/latestnews.xml",
		"https://economictimes.indiatimes.com/markets/rssfeeds/1977021501.cms",
	})

	// Screener defaults
	v.SetDefault("screener.base_url", "https://www.screener.in")
	v.SetDefault("screener.scrape_enabled", true)
	v.SetDefault("screener.scrape_delay", "2s")
}

// bindEnvVars binds environment variables to config keys.
func bindEnvVars(v *viper.Viper) {
	// App
	_ = v.BindEnv("app.env", "APP_ENV")
	_ = v.BindEnv("app.log_level", "LOG_LEVEL")

	// Database
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.port", "DB_PORT")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.dbname", "DB_NAME")
	_ = v.BindEnv("database.sslmode", "DB_SSLMODE")

	// Server
	_ = v.BindEnv("server.port", "SERVER_PORT")

	// LLM
	_ = v.BindEnv("llm.provider", "LLM_PROVIDER")
	_ = v.BindEnv("llm.ollama.url", "OLLAMA_URL")
	_ = v.BindEnv("llm.ollama.model", "OLLAMA_MODEL")
	_ = v.BindEnv("llm.openai.api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("llm.openai.model", "OPENAI_MODEL")
	_ = v.BindEnv("llm.gemini.api_key", "GEMINI_API_KEY")
	_ = v.BindEnv("llm.gemini.model", "GEMINI_MODEL")

	// Analysis
	_ = v.BindEnv("analysis.use_llm", "USE_LLM")
	_ = v.BindEnv("analysis.use_keyword_sentiment", "USE_KEYWORD_SENTIMENT")
}

// IsDevelopment returns true if the app is in development mode.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true if the app is in production mode.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

