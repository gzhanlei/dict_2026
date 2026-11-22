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
- `config/app.yaml` - Runtime configuration (gitignored, never commit real credentials; template: `config/app.example.yaml`)
- `config/app.example.yaml` - Sanitized configuration template
- `config/dev.yaml` - Development configuration (if exists)

Key configuration sections:
- **database**: MySQL connection settings (host, port, username, password, dbname)
- **tables**: Table names for places and names (with date suffixes like `_260202`)
- **fields**: Column names used in queries (`name_for_query_*`, pre-cleaned `query_field_clean`, `name`, `nation`, `translation`)
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
- Parens in stored values: non-Chinese queries prefer the pre-cleaned column configured as `fields.query_field_clean` (default `name_for_query_clean`, a STORED generated column created by `scripts/add_name_for_query_clean.sql`), so the WHERE clause is a plain `LIKE` on the column. Wrapping the column in `REPLACE()` defeats all indexes and re-evaluates the function per row (a past cause of slow full-table scans)
- If the clean column is missing (MySQL error 1054, e.g. DDL not run yet or a rebuilt table without it), the service logs a warning and falls back to the legacy `REPLACE(REPLACE(query_field, '(', ' '), ')', ' ')` query for the rest of the process lifetime. Results are identical, only slower
- When rebuilding the date-suffixed tables, include the `name_for_query_clean` generated column and its index in the CREATE TABLE (see the SQL script)

### Test Patterns
- Unit tests use `t.Skip()` when database is unavailable
- E2E tests require build tag: `//go:build e2e`
- Real integration tests (with suffix `_real_test.go`) may require database setup
- Simple tests (with suffix `_simple_test.go`) are lightweight and don't require external dependencies

### Configuration Defaults
If config values are missing, the following defaults apply (see `config/config.go`):
- Server port: 8080
- Cache directory: `./cache`
- Cache expiration: 21600 seconds (6h)
- Log directory: `./logs`
- Global concurrent uncached searches: 4 (`security.max_concurrent_searches`)
- Per-IP concurrent uncached searches: 1 (`security.max_concurrent_per_ip`)

### Search Concurrency Guards
Uncached (database) searches pass through two non-blocking gates before hitting MySQL:
- **Global gate**: at most `security.max_concurrent_searches` concurrent DB searches process-wide
- **Per-IP gate**: at most `security.max_concurrent_per_ip` concurrent DB searches per client IP

When either gate is full the request fails fast with HTTP 429 (`ErrSearchBusy` in
`internal/service/guard.go`), protecting the shared database from concurrent full-table scans.
Cache hits bypass the gates entirely. Cumulative counters (requests, cache hits, DB searches,
busy rejects, DB errors, SQL sub-queries, active searches) are exposed on `/api/health`
under `metrics`, and every search logs one structured line (`search ip=... dur=...ms cache=...`)
for diagnosing crawler traffic.

### Cloudflare Turnstile
`/api/search` can be gated by Cloudflare Turnstile (human verification), configured in
`config/app.yaml` under `turnstile`:
- Enforcement is active only when `enabled: true` **and** a secret is present
  (`turnstile.secret_key` or env `TURNSTILE_SECRET`, which wins). `enabled` without a secret
  logs a warning at startup and fails open, so a config mistake can't take the site down.
- The frontend fetches `GET /api/config` (public values only: enabled/site_key/action) and, when
  enabled, loads the Turnstile script, renders an invisible widget (`static/js/app.js`), and sends
  the token as `cf-turnstile-response` in the search POST body. Tokens are single-use: the widget
  resets after every search (`afterSearchTurnstile`) and `ensureTurnstileToken` re-arms on expiry.
- Server-side verification (`internal/service/turnstile.go`) POSTs to the siteverify endpoint with
  a 10s timeout, then checks `success`, `action` (must equal `turnstile.action`), and `hostname`
  (must be in `turnstile.hostnames` allowlist; empty list skips the check). Any failure is 403,
  and rejection happens before query logging or any database work.
- The CSP in `internal/middleware/security.go` must keep `https://challenges.cloudflare.com`
  allowed under script-src/frame-src/connect-src, or the widget silently fails to load.
- Local testing uses Cloudflare's test keys (invisible always-pass sitekey `1x00000000000000000000BB`,
  always-pass secret `1x0000000000000000000000000000000AA`, always-fail secret `2x0000000000000000000000000000000AA`).
