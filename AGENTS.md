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
- **Mainland-China availability caveat (2026-11-22)**: some CN carriers intermittently stall
  `challenges.cloudflare.com/cdn-cgi/challenge-platform/*` (the widget iframe) while other paths
  on the same host load fine — no error-callback fires, the token simply never arrives, and
  enforced Turnstile randomly blocks real users. Frontend mitigations: wait up to 10s for a
  token (covers deep-link races too), discard stale tokens (>240s old; siteverify rejects them
  as `timeout-or-duplicate`), auto-reload once on a 403. Enforcement was disabled after
  confirmed real-user impact; re-enable only with this tradeoff in mind.
- **Cache busting**: `static/index.html` references `app.js?v=<date><rev>`; bump the stamp every
  time `static/js/app.js` changes. `/static/` is served by nginx with `expires 5m` (dict vhost),
  so unstamped JS edits take up to 5 minutes to propagate; stamped ones are immediate.
- Local testing uses Cloudflare's test keys (invisible always-pass sitekey `1x00000000000000000000BB`,
  always-pass secret `1x0000000000000000000000000000000AA`, always-fail secret `2x0000000000000000000000000000000AA`).
  Test sitekeys skip domain verification, so they work on localhost/127.0.0.1; real sitekeys
  require the domain to be registered on the widget.

### queries 表（查询日志）保留策略
`/api/search` 每个通过校验的请求都会异步写一行 `queries`（含缓存命中——INSERT 在搜索执行前，
见 `SaveQueryLog`；Turnstile/限流/参数校验拒绝的不记，429/500 会记）。表只增不减，维护方案
（脚本已部署到服务器 `/data/www/dict20260412/scripts/`，root crontab 驱动）：
- `scripts/cleanup_queries.sh`：分批 DELETE 超过 180 天的明细（每日 04:00）。分批是避免大事务
  锁表/binlog 一次性膨胀；`RETENTION_DAYS`/`BATCH`/`BATCH_SLEEP` 可用环境变量覆盖。前提是
  `queries.query_time` 上有 `idx_query_time` 索引，否则每批 DELETE 都是全表扫描。
- `scripts/rollup_queries_daily.sh`：把"昨天"聚合成两张表（每日 00:05，均 REPLACE 幂等，
  重跑不会翻倍）：
  - `queries_daily`（day, query_type, cnt, distinct_ips, distinct_words）——每日总量趋势；
  - `queries_words_daily`（day, query_type, query_word, cnt, distinct_ips）——每词每日
    一行，"最热查询词"统计从这张表做（明细清理后历史仍在）：
    `SELECT query_word, SUM(cnt) FROM queries_words_daily WHERE query_word <> ''
     GROUP BY query_word ORDER BY 2 DESC LIMIT 20;`（`<> ''` 排除历史爬虫刷的空白词）。
    该表 collation 必须是 `utf8mb4_general_ci`（区分重音，与源表 utf8mb3_general_ci
    语义一致）；默认的 `utf8mb4_0900_ai_ci` 会把 jose/josé 这类重音变体折叠成同一个
    PK 键，REPLACE 时静默丢计数（2026-11-22 回填实测）。
- 服务器凭据在 `/root/.dict_maint.cnf`（0600，从 `config/app.yaml` 解析生成），脚本经
  `--defaults-extra-file` 使用，密码不进脚本与 crontab。
- InnoDB 的 DELETE 只标记删除，文件不立即缩小；空间会被新写入复用，稳态后表大小不再增长。
  首次大清理后如需立即回收，可低峰执行一次 `OPTIMIZE TABLE queries`。
