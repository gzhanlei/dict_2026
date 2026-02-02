package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

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