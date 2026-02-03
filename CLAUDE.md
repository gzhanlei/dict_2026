# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

```bash
# Build the application
go build -o dict_new .

# Run the application (requires MySQL with config/app.yaml settings)
./dict_new

# Run with custom port
PORT=9000 ./dict_new

# Run all unit tests
go test ./...

# Run tests for specific package
go test ./internal/service
go test ./internal/util
go test ./internal/handler

# Run single test
go test ./internal/service -run TestProcessQuery

# Run E2E tests (requires running server)
go test -tags=e2e -v ./e2e_test.go
TEST_BASE_URL=http://localhost:9000 go test -tags=e2e -v ./e2e_test.go

# Docker build
docker build -t dict_new .
docker run -p 8080:8080 dict_new
```

## Architecture Overview

This is a multilingual dictionary web service (Go/Gin) that searches person names and place names across Chinese, Japanese, and English translations stored in MySQL.

### Three-Tier Architecture

1. **Handler Layer** (`internal/handler/`) - Gin HTTP framework
   - `api.go`: Routes for `/api/search`, `/api/health`, `/:type/:query`
   - Request/response handling and validation

2. **Service Layer** (`internal/service/`) - Business logic
   - `services.go`: Dependency container (`Services` struct) that wires DB, Cache, Search
   - `search.go`: Core search logic with language detection and query processing
   - `cache.go`: File-based caching with expiration

3. **Database Layer** (`internal/database/`) - MySQL connection management
   - Connection pooling (25 max open, 5 max idle)

### Service Initialization Flow

`main.go` → `service.NewServices()` → loads config → creates DB connection → initializes cache → creates search service → handlers receive `*Services` via dependency injection.

### Key Implementation Details

**Language Detection** (`internal/util/`):
- `chinese.go`: CJK range detection
- `kana.go`: Japanese hiragana/katakana detection and romaji conversion

**Query Processing**:
- Japanese queries are converted to romaji for database matching
- Chinese queries search `translation` field with `LIKE %query%`
- English/romaji queries search `name_for_query_*` field with `LIKE '% query %'` (note: field values have leading/trailing spaces)
- Hyphens replaced with spaces before matching

**Multi-word Queries**:
- Split into full phrase + individual words
- Returns `tabs` array and `tab_results` map
- Results ranked: exact match → starts with → contains whole word → other

**Caching**:
- File-based cache in `./cache` directory
- Cache key: `processedQuery_searchType_language`
- Bypass with `noCache: true` in API request or `?cache=no` in URL

**Configuration**:
- `config/app.yaml` - Production config
- `config/dev.yaml` - Development config (if exists)
- Defaults: port 8080, cache dir `./cache`, expiration 3600s, log dir `./logs`

### Test Patterns

- Unit tests: `*_test.go` - skip DB tests when unavailable via `t.Skip()`
- E2E tests: `e2e_test.go` with build tag `//go:build e2e`
- Simple tests: `*_simple_test.go` - no external dependencies
- Real integration tests: `*_real_test.go` - require database setup
