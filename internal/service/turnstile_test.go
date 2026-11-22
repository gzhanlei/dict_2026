package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"dict_new/config"
)

// newTestTurnstile 创建指向 httptest 假 siteverify 的校验服务
func newTestTurnstile(t *testing.T, handler http.HandlerFunc, mutate func(*config.Config)) (*TurnstileService, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := &config.Config{
		Turnstile: config.TurnstileConfig{
			Enabled:   true,
			SiteKey:   "test-site-key",
			SecretKey: "test-secret",
			Action:    "search",
			Hostnames: []string{"dict.hanlei.name"},
		},
	}
	if mutate != nil {
		mutate(cfg)
	}
	return NewTurnstileService(cfg, server.URL), server
}

func writeSiteverifyResponse(w http.ResponseWriter, resp turnstileVerifyResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func TestTurnstileVerifySuccess(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		// 校验请求参数完整性（secret/response 必填，来自前端表单编码）
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form failed: %v", err)
		}
		if r.Form.Get("secret") != "test-secret" {
			t.Errorf("secret not sent, got %q", r.Form.Get("secret"))
		}
		if r.Form.Get("response") != "valid-token" {
			t.Errorf("token not sent, got %q", r.Form.Get("response"))
		}
		if r.Form.Get("remoteip") != "1.2.3.4" {
			t.Errorf("remoteip not sent, got %q", r.Form.Get("remoteip"))
		}
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: true, Action: "search", Hostname: "dict.hanlei.name"})
	}, nil)

	if err := svc.Verify(context.Background(), "valid-token", "1.2.3.4"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestTurnstileVerifyActionMismatch(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: true, Action: "other-surface", Hostname: "dict.hanlei.name"})
	}, nil)
	err := svc.Verify(context.Background(), "token", "")
	if !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected ErrTurnstileRejected for action mismatch, got %v", err)
	}
}

func TestTurnstileVerifyHostnameNotAllowed(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: true, Action: "search", Hostname: "evil.example.com"})
	}, nil)
	err := svc.Verify(context.Background(), "token", "")
	if !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected ErrTurnstileRejected for hostname not in allowlist, got %v", err)
	}
}

func TestTurnstileVerifyHostnameCheckSkippedWhenNotConfigured(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: true, Action: "search", Hostname: "anything.example.com"})
	}, func(c *config.Config) { c.Turnstile.Hostnames = nil })

	if err := svc.Verify(context.Background(), "token", ""); err != nil {
		t.Fatalf("expected hostname check skipped, got %v", err)
	}
}

func TestTurnstileVerifySiteverifyFailure(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: false, Errors: []string{"invalid-input-response"}})
	}, nil)
	err := svc.Verify(context.Background(), "spent-token", "")
	if !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected ErrTurnstileRejected for success=false, got %v", err)
	}
}

func TestTurnstileVerifyEmptyTokenFastFail(t *testing.T) {
	svc, server := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("empty token must not reach siteverify")
	}, nil)
	err := svc.Verify(context.Background(), "  ", "")
	if !errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected ErrTurnstileRejected for empty token, got %v", err)
	}
	server.Close()
}

func TestTurnstileVerifyHTTPError(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}, nil)
	// 网络层故障返回普通 error（非 ErrTurnstileRejected），handler 同样按 403 拒绝
	err := svc.Verify(context.Background(), "token", "")
	if err == nil || errors.Is(err, ErrTurnstileRejected) {
		t.Fatalf("expected infra error, got %v", err)
	}
}

func TestTurnstileEnforced(t *testing.T) {
	noop := func(w http.ResponseWriter, r *http.Request) {}

	// enabled + secret → 校验生效
	svc, _ := newTestTurnstile(t, noop, nil)
	if !svc.Enforced() {
		t.Error("expected enforced when enabled with secret")
	}

	// enabled 但缺 secret → fail-open（跳过校验）
	svc, _ = newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {}, func(c *config.Config) {
		c.Turnstile.SecretKey = ""
	})
	if svc.Enforced() {
		t.Error("expected not enforced when secret missing")
	}

	// disabled → 不校验
	svc, _ = newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {}, func(c *config.Config) {
		c.Turnstile.Enabled = false
	})
	if svc.Enforced() {
		t.Error("expected not enforced when disabled")
	}
}

func TestTurnstileSecretFromEnv(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("secret") != "env-secret" {
			t.Errorf("expected env secret used, got %q", r.Form.Get("secret"))
		}
		writeSiteverifyResponse(w, turnstileVerifyResponse{Success: true, Action: "search", Hostname: "dict.hanlei.name"})
	}, func(c *config.Config) { c.Turnstile.SecretKey = "yaml-secret" })

	t.Setenv("TURNSTILE_SECRET", "env-secret")
	if err := svc.Verify(context.Background(), "token", ""); err != nil {
		t.Fatalf("expected env TURNSTILE_SECRET to override yaml, got %v", err)
	}
	t.Setenv("TURNSTILE_SECRET", "")
}

func TestTurnstileGetPublicConfig(t *testing.T) {
	svc, _ := newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	pc := svc.GetPublicConfig()
	if !pc.Enabled || pc.SiteKey != "test-site-key" || pc.Action != "search" {
		t.Errorf("unexpected public config: %+v", pc)
	}

	svc, _ = newTestTurnstile(t, func(w http.ResponseWriter, r *http.Request) {}, func(c *config.Config) {
		c.Turnstile.Enabled = false
	})
	pc = svc.GetPublicConfig()
	if pc.Enabled || pc.SiteKey != "" {
		t.Errorf("disabled config should not expose site key: %+v", pc)
	}
}
