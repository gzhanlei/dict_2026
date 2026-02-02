# AGENTS.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

This is a multilingual dictionary web service built with Go and Gin framework. It searches person names and place names across Chinese, Japanese, and English translations stored in MySQL. The service automatically detects input language (Chinese, Japanese, or English) and performs language-specific query processing including Japanese kana-to-romaji conversion.

## Build and Run

### Development
```bash
# Build the application
go build -o dict_new .

# Run the application (requires MySQL running with config/app.yaml settings)
./dict_new

# Run with custom port
PORT=9000 ./dict_new
```

### Docker
```bash
# Build Docker image
docker build -t dict_new .

# Run in Docker
docker run -p 8080:8080 dict_new
```

## Testing

### Unit Tests
```bash
# Run all unit tests
go test ./...

# Run tests for specific package
go test ./internal/service
go test ./internal/util
go test ./internal/handler

# Run single test
go test ./internal/service -run TestProcessQuery

# Run tests with verbose output
go test -v ./...
```

### E2E Tests
```bash
# E2E tests require the server to be running first
# Start the server in one terminal, then run:
go test -tags=e2e -v ./e2e_test.go

# Or set custom base URL:
TEST_BASE_URL=http://localhost:9000 go test -tags=e2e -v ./e2e_test.go
```

## Configuration

Configuration is managed via YAML files in `config/`:
- `config/app.yaml` - Production configuration
- `config/dev.yaml` - Development configuration (if exists)

Key configuration sections:
- **database**: MySQL connection settings (host, port, username, password, dbname)
- **tables**: Table names for places and names (with date suffixes like `_260202`)
- **fields**: Column names used in queries (`name_for_query_*`, `name`, `nation`, `translation`)
- **cache**: File-based cache directory and expiration time
- **log**: Log file directory

## Architecture

### Service Layer Pattern
The application follows a three-tier architecture:
1. **Handler Layer** (`internal/handler`) - HTTP request/response handling with Gin
2. **Service Layer** (`internal/service`) - Business logic (search, caching)
3. **Database Layer** (`internal/database`) - MySQL connection management

### Service Initialization Flow
The `Services` struct in `internal/service/services.go` acts as a dependency container:
1. Loads configuration from `config/app.yaml`
2. Creates database connection with connection pooling (25 max open, 5 max idle)
3. Initializes file-based cache service
4. Creates search service with dependencies injected

All services are initialized in `main.go` and passed to handlers via dependency injection.

### Multilingual Search Logic

**Language Detection** (`internal/util`):
- `chinese.go`: Detects Chinese characters (CJK ranges)
- `kana.go`: Detects Japanese (hiragana, katakana) and converts to romaji

**Query Processing Flow**:
1. Input query is language-detected (zh/ja/en)
2. Japanese queries are converted to romaji for database matching
3. Chinese queries search the `translation` field with `LIKE %query%`
4. English/romaji queries search the `name_for_query` field with `LIKE % query %` (note: field values have spaces on both sides)
5. Multi-word queries (e.g., "George Washington") are split: full phrase + individual words
6. Results are ranked: exact match → starts with → contains as whole word → other

**Tab Results Feature**:
When a multi-word query is entered, the API returns:
- `tabs`: Array of query parts (full query + individual words)
- `tab_results`: Map of results for each tab
- If the full query returns no results but individual words do, those are merged and deduplicated

### Caching Strategy
File-based cache in `./cache` directory:
- Cache keys generated from: `processedQuery_searchType_language`
- Cache includes both raw results and tab-separated results
- Cache can be bypassed with `noCache: true` in API request or `?cache=no` in URL

### URL Routing
The app supports both API and direct URL routing:
- `/api/search` - POST endpoint for search
- `/api/health` - GET endpoint for health checks
- `/:type/:query` - GET endpoint that serves the HTML page (JavaScript handles the search)
  - Types: `names`, `places`, `both` (also accepts `person`, `place` which are normalized)

## Important Implementation Details

### Database Query Format
The `name_for_query_*` field in the database stores values with **leading and trailing spaces**. Therefore:
- English/romaji queries must use `LIKE '% query %'` pattern (spaces on both sides)
- Do not use `LIKE '%query%'` for the query field or results will be incorrect
- Hyphens in queries are replaced with spaces before matching (e.g., "New-York" → "New York")

### Test Patterns
- Unit tests use `t.Skip()` when database is unavailable
- E2E tests require build tag: `//go:build e2e`
- Real integration tests (with suffix `_real_test.go`) may require database setup
- Simple tests (with suffix `_simple_test.go`) are lightweight and don't require external dependencies

### Configuration Defaults
If config values are missing, the following defaults apply (see `config/config.go`):
- Server port: 8080
- Cache directory: `./cache`
- Cache expiration: 3600 seconds
- Log directory: `./logs`
