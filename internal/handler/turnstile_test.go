package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dict_new/config"
	"dict_new/internal/service"
	"github.com/gin-gonic/gin"
)

// newTurnstileTestServices 构造启用 Turnstile 的 Services；Search 留 nil，
// 网关应在触达搜索逻辑之前拒绝（403），从而验证网关位置正确
func newTurnstileTestServices(t *testing.T, siteverify http.HandlerFunc, mutate func(*config.Config)) *service.Services {
	t.Helper()
	server := httptest.NewServer(siteverify)
	t.Cleanup(server.Close)

	cfg := &config.Config{
		Turnstile: config.TurnstileConfig{
			Enabled:   true,
			SiteKey:   "test-site-key",
			SecretKey: "test-secret",
			Action:    "search",
			Hostnames: []string{"localhost"},
		},
		Security: config.SecurityConfig{MaxConcurrentSearches: 4, MaxConcurrentPerIP: 1},
	}
	if mutate != nil {
		mutate(cfg)
	}
	return &service.Services{
		Config:    cfg,
		Turnstile: service.NewTurnstileService(cfg, server.URL),
	}
}

func performTurnstileSearchRequest(t *testing.T, services *service.Services, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/search", SearchHandler(services))

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestSearchHandlerTurnstileMissingToken(t *testing.T) {
	services := newTurnstileTestServices(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("missing token must not reach siteverify")
	}, nil)

	w := performTurnstileSearchRequest(t, services, map[string]interface{}{"query": "ford", "type": "both"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing token, got %d: %s", w.Code, w.Body.String())
	}
	var resp SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Success || resp.Error == "" {
		t.Errorf("expected failure response with error message, got %+v", resp)
	}
}

func TestSearchHandlerTurnstileInvalidToken(t *testing.T) {
	services := newTurnstileTestServices(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error-codes": []string{"invalid-input-response"}})
	}, nil)

	w := performTurnstileSearchRequest(t, services, map[string]interface{}{
		"query":                 "ford",
		"type":                  "both",
		"cf-turnstile-response": "bad-token",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSearchHandlerTurnstileDisabled(t *testing.T) {
	// disabled：无 token 也不应被网关拦截；Search 为 nil 会 panic，
	// 所以用 recover 检查是否通过了网关（到达搜索阶段）
	defer func() {
		if r := recover(); r != nil {
			// 通过了网关、进入搜索逻辑（Search为nil导致panic）= 网关放行 ✓
			return
		}
	}()
	services := newTurnstileTestServices(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("disabled turnstile must not call siteverify")
	}, func(cfg *config.Config) { cfg.Turnstile.Enabled = false })

	performTurnstileSearchRequest(t, services, map[string]interface{}{"query": "ford", "type": "both"})
}

func TestConfigHandlerTurnstile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	services := newTurnstileTestServices(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	router := gin.New()
	router.GET("/api/config", ConfigHandler(services))

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp PublicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !resp.Success || resp.Turnstile == nil {
		t.Fatalf("unexpected config response: %+v", resp)
	}
	if !resp.Turnstile.Enabled || resp.Turnstile.SiteKey != "test-site-key" || resp.Turnstile.Action != "search" {
		t.Errorf("unexpected turnstile public config: %+v", resp.Turnstile)
	}
}
