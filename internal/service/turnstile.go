package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dict_new/config"
)

// TurnstileVerifyURL Cloudflare siteverify 端点（服务端校验，绝不能从浏览器调用）
const TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// ErrTurnstileRejected token 校验未通过（无效/过期/已使用/action或hostname不匹配），handler 应返回 403
var ErrTurnstileRejected = errors.New("turnstile verification failed")

// turnstileVerifyResponse siteverify 响应
type turnstileVerifyResponse struct {
	Success  bool     `json:"success"`
	Action   string   `json:"action"`
	Hostname string   `json:"hostname"`
	Errors   []string `json:"error-codes"`
}

// TurnstileService Cloudflare Turnstile 服务端校验（siteverify）
type TurnstileService struct {
	config    *config.Config
	verifyURL string
	client    *http.Client
}

// NewTurnstileService 创建校验服务；verifyURL 一般留空使用官方端点，测试时传入 httptest 地址
func NewTurnstileService(cfg *config.Config, verifyURL string) *TurnstileService {
	if verifyURL == "" {
		verifyURL = TurnstileVerifyURL
	}
	// 10秒超时：siteverify 偶发抖动时不能让 /api/search 长时间挂起
	return &TurnstileService{
		config:    cfg,
		verifyURL: verifyURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Enforced 是否启用强制校验：enabled 且 secret 已配置。
// enabled 但缺 secret 属配置不完整：跳过校验并告警（fail-open），避免配置失误导致全站 403
func (t *TurnstileService) Enforced() bool {
	return t.config.Turnstile.Enabled && t.config.TurnstileSecret() != ""
}

// Verify 校验前端提交的 token；通过返回 nil，未通过返回 ErrTurnstileRejected，
// 网络等基础设施故障返回其他错误（handler 同样按 403 拒绝，宁可错杀不放行爬虫）
func (t *TurnstileService) Verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: token is empty", ErrTurnstileRejected)
	}

	form := url.Values{}
	form.Set("secret", t.config.TurnstileSecret())
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("turnstile: build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile: siteverify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile: siteverify returned HTTP %d", resp.StatusCode)
	}

	var result turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("turnstile: siteverify response decode failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("%w: siteverify errors=%v", ErrTurnstileRejected, result.Errors)
	}

	// action 必须与前端 data-action 一致，防止其他surface的token挪用
	if expected := t.config.Turnstile.Action; expected != "" && result.Action != expected {
		return fmt.Errorf("%w: action mismatch, want %q got %q", ErrTurnstileRejected, expected, result.Action)
	}

	// hostname 白名单：配置了才校验（widget 允许的域名以 Cloudflare 控制台为准）
	if hosts := t.config.Turnstile.Hostnames; len(hosts) > 0 {
		allowed := false
		for _, h := range hosts {
			if result.Hostname == h {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: hostname %q not allowed", ErrTurnstileRejected, result.Hostname)
		}
	}

	return nil
}

// PublicConfig 前端渲染所需信息（site key 为公开值，secret 绝不下发）
type PublicConfig struct {
	Enabled bool   `json:"enabled"`
	SiteKey string `json:"site_key,omitempty"`
	Action  string `json:"action,omitempty"`
}

// GetPublicConfig 返回给前端的公开配置
func (t *TurnstileService) GetPublicConfig() PublicConfig {
	out := PublicConfig{Enabled: t.Enforced()}
	if out.Enabled {
		out.SiteKey = t.config.Turnstile.SiteKey
		out.Action = t.config.Turnstile.Action
	}
	return out
}
