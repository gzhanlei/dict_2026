//go:build e2e
// +build e2e

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

type E2ESearchRequest struct {
	Query string `json:"query"`
	Type  string `json:"type"`
}

type E2ESearchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Names  []interface{} `json:"names"`
		Places []interface{} `json:"places"`
		Cache  bool          `json:"cache"`
	} `json:"data"`
	Error string `json:"error"`
}

func TestE2EHealthCheck(t *testing.T) {
	baseURL := getBaseURL()
	
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("Failed to make health check request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
	}
}

func TestE2ESearchAPI(t *testing.T) {
	baseURL := getBaseURL()
	
	// 测试搜索API
	searchReq := E2ESearchRequest{
		Query: "test",
		Type:  "names",
	}

	reqBody, _ := json.Marshal(searchReq)
	resp, err := http.Post(baseURL+"/api/search", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		t.Fatalf("Failed to make search request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 检查响应格式
	var searchResp E2ESearchResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &searchResp); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if !searchResp.Success {
		t.Errorf("Expected success=true, got %v", searchResp.Success)
	}
}

func TestE2EFrontendPage(t *testing.T) {
	baseURL := getBaseURL()
	
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("Failed to get frontend page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 检查内容类型
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected text/html, got %s", contentType)
	}
}

func TestE2EStaticFiles(t *testing.T) {
	baseURL := getBaseURL()
	
	files := []string{
		"/static/css/style.css",
		"/static/js/app.js",
	}

	for _, file := range files {
		resp, err := http.Get(baseURL + file)
		if err != nil {
			t.Fatalf("Failed to get static file %s: %v", file, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for %s, got %d", file, resp.StatusCode)
		}
	}
}

func getBaseURL() string {
	baseURL := os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return baseURL
}

// 辅助函数：等待服务启动
func waitForServer(baseURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	start := time.Now()
	
	for time.Since(start) < timeout {
		resp, err := client.Get(baseURL + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("server not ready after timeout")
}