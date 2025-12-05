# StockChef - AI-Powered Stock Recommender for Indian Markets

StockChef is a Go-based stock recommendation application that analyzes news, market conditions, and fundamental data from screener.in to provide actionable stock recommendations for the Indian market.

## Features

- **Multi-Source Analysis**: Combines news sentiment, fundamental data, and market conditions
- **LLM Integration**: Supports Ollama (local), OpenAI, and Google Gemini for AI-powered analysis
- **Keyword Sentiment Analysis**: Built-in dictionary-based sentiment scoring (no external dependencies)
- **Screener.in Integration**: Web scraping and CSV upload support for fundamental data
- **News Aggregation**: Fetches from MoneyControl, Economic Times, LiveMint, and more
- **Web Dashboard**: Modern, responsive UI built with TailwindCSS
- **REST API**: Full-featured API for programmatic access

## Tech Stack

- **Go 1.22+** with Gin-Gonic web framework
- **GORM** with PostgreSQL
- **TailwindCSS** for styling (via CDN)
- **Viper** for configuration management

## Quick Start

### Prerequisites

- Go 1.22 or later
- Docker & Docker Compose (for PostgreSQL)
- (Optional) Ollama for local LLM

### Installation

1. Clone the repository:
```bash
cd stock-recommender
```

2. Copy the example environment file and configure:
```bash
cp env.example .env
# Edit .env with your settings (database password, API keys, etc.)
```

3. Start PostgreSQL with Docker:
```bash
docker compose up -d
```

4. Install dependencies and run:
```bash
go mod tidy
go run cmd/recommender/main.go
```

5. Open http://localhost:8081 in your browser.

### Stopping the Database

```bash
docker compose down        # Stop containers (data persists)
docker compose down -v     # Stop and remove data
```

### Security Notes

- **Never commit `.env` files** - They contain secrets and are in `.gitignore`
- Use `env.example` as a template for required environment variables
- For production, use proper secret management (Vault, AWS Secrets Manager, etc.)
- Database passwords and API keys should always come from environment variables

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `DB_USER` | Database user | postgres |
| `DB_PASSWORD` | Database password | postgres |
| `DB_NAME` | Database name | stock_recommender |
| `SERVER_PORT` | HTTP server port | 8080 |
| `LLM_PROVIDER` | LLM provider (ollama/openai/gemini) | ollama |
| `OLLAMA_URL` | Ollama API URL | http://localhost:11434 |
| `OLLAMA_MODEL` | Ollama model name | llama3 |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `USE_LLM` | Enable LLM analysis | true |
| `USE_KEYWORD_SENTIMENT` | Enable keyword sentiment | true |

### LLM Providers

#### Ollama (Local - Recommended)
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model
ollama pull llama3

# Set in config
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=llama3
```

#### OpenAI
```bash
LLM_PROVIDER=openai
OPENAI_API_KEY=your-api-key
OPENAI_MODEL=gpt-4o-mini
```

#### Google Gemini
```bash
LLM_PROVIDER=gemini
GEMINI_API_KEY=your-api-key
GEMINI_MODEL=gemini-pro
```

## API Endpoints

### Recommendations
- `GET /api/v1/recommendations` - List all recommendations
- `GET /api/v1/recommendations/:id` - Get single recommendation
- `POST /api/v1/analyze` - Analyze a stock (body: `{"symbol": "RELIANCE"}`)

### News
- `GET /api/v1/news` - List recent news
- `POST /api/v1/news/refresh` - Refresh news from RSS feeds

### Screener Data
- `POST /api/v1/screener/upload` - Upload screener.in CSV
- `GET /api/v1/screener/columns` - Get supported CSV columns

### Stocks
- `GET /api/v1/stocks` - List stocks
- `GET /api/v1/stocks/:symbol` - Get stock details

### Health
- `GET /api/v1/health` - Health check

## Web Dashboard

The application includes a modern web dashboard at http://localhost:8080 with:

- **Dashboard**: View all recommendations with action signals
- **News**: Browse market news with sentiment indicators
- **Upload**: Import screener.in CSV exports
- **Stock Analysis**: Analyze any stock symbol

## Development

### Project Structure
```
stock-recommender/
├── cmd/recommender/      # Main application entry point
├── internal/
│   ├── api/              # Gin handlers and routes
│   ├── analyzer/         # News fetching and analysis
│   ├── llm/              # LLM provider implementations
│   ├── recommender/      # Core recommendation engine
│   ├── screener/         # Screener.in scraper & CSV parser
│   ├── sentiment/        # Keyword-based sentiment analysis
│   └── storage/          # GORM models and repository
├── pkg/config/           # Configuration management
├── web/templates/        # HTML templates
├── configs/              # Configuration files
└── migrations/           # Database migrations
```

### Running Tests
```bash
go test ./...
```

### Building
```bash
make build
# or
go build -o bin/recommender cmd/recommender/main.go
```

## Disclaimer

This application is for educational and informational purposes only. Stock recommendations generated by this tool should not be considered as financial advice. Always do your own research and consult with a qualified financial advisor before making investment decisions.

## License

MIT License

