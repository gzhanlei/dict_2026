package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dict_new/config"
	"dict_new/internal/service"
	"github.com/gin-gonic/gin"
)

func TestHealthHandlerSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)

	services := &service.Services{}

	// 创建测试路由
	router := gin.New()
	router.GET("/api/health", HealthHandler(services))

	// 创建请求
	req, _ := http.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 检查状态码
	if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusOK {
		t.Errorf("Expected status %d or %d, got %d", http.StatusOK, http.StatusServiceUnavailable, w.Code)
	}

	// 检查响应体不为空
	if w.Body.Len() == 0 {
		t.Error("Expected non-empty response body")
	}
}

func TestSetupRoutesSimple(t *testing.T) {
	gin.SetMode(gin.TestMode)

	services := &service.Services{}
	router := gin.New()

	// 设置路由
	result := SetupRoutes(router, services)

	if result == nil {
		t.Error("Expected non-nil router")
	}
}

// TestSearchHandlerQueryValidation 测试查询词长度验证
func TestSearchHandlerQueryValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建带配置的服务
	services := &service.Services{
		Config: &config.Config{
			Security: config.SecurityConfig{
				TrustedProxies: []string{},
			},
		},
	}

	router := gin.New()
	router.POST("/api/search", SearchHandler(services))

	tests := []struct {
		name           string
		query          string
		searchType     string
		wantStatus     int
		shouldBeError  bool
	}{
		{
			name:          "空查询",
			query:         "",
			searchType:    "both",
			wantStatus:    http.StatusBadRequest,
			shouldBeError: true,
		},
		{
			name:          "仅空格",
			query:         "   ",
			searchType:    "both",
			wantStatus:    http.StatusBadRequest,
			shouldBeError: true,
		},
		{
			name:          "单个字符",
			query:         "a",
			searchType:    "both",
			wantStatus:    http.StatusBadRequest,
			shouldBeError: true,
		},
		{
			name:          "空格包裹的单个字符",
			query:         "  a  ",
			searchType:    "both",
			wantStatus:    http.StatusBadRequest,
			shouldBeError: true,
		},
		{
			name:          "无效搜索类型",
			query:         "test query",
			searchType:    "invalid",
			wantStatus:    http.StatusBadRequest,
			shouldBeError: true,
		},
		// 注意：有效查询的测试需要完整的service mock，这里只测试验证失败的情况
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"query": tt.query,
				"type":  tt.searchType,
			}
			jsonBody, _ := json.Marshal(reqBody)

			req, _ := http.NewRequest("POST", "/api/search", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.shouldBeError {
				var resp SearchResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Success {
					t.Error("Expected error response, got success")
				}
				if resp.Error == "" {
					t.Error("Expected error message in response")
				}
			}
		})
	}
}

// TestQueryValidationHelper 测试查询词验证逻辑
// 注意：验证基于字符数量（Rune count），不是字节长度
func TestQueryValidationHelper(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"空字符串", "", false},
		{"仅空格", "   ", false},
		{"单个字符", "a", false},
		{"两个字符", "ab", true},
		{"单个中文", "中", false},
		{"两个中文", "中国", true},
		{"空格包裹的单个字符", "  a  ", false},
		{"空格包裹的两个字符", "  ab  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟后端验证逻辑：trim后检查字符数（不是字节数）
			trimmed := strings.TrimSpace(tt.query)
			// 计算Unicode字符数（不是字节数）
			runeCount := len([]rune(trimmed))
			result := runeCount >= 2
			if result != tt.expected {
				t.Errorf("Query %q: expected %v, got %v (trimmed=%q, runes=%d)",
					tt.query, tt.expected, result, trimmed, runeCount)
			}
		})
	}
}