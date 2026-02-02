package service

import (
	"testing"

	"dict_new/config"
	"dict_new/internal/database"
)

func TestNewSearchService(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql",
		},
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_cache",
			Expiration: 3600,
		},
	}

	// 创建数据库连接
	db, err := database.NewConnection(cfg)
	if err != nil {
		t.Skipf("Skipping test, cannot connect to database: %v", err)
		return
	}
	defer db.Close()

	// 创建缓存服务
	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}

	// 创建搜索服务
	search, err := NewSearchService(db, cache, cfg)
	if err != nil {
		t.Fatalf("Failed to create search service: %v", err)
	}

	if search == nil {
		t.Error("Expected non-nil search service")
	}
}

func TestProcessQuery(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql",
		},
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_cache",
			Expiration: 3600,
		},
	}

	// 创建数据库连接
	db, err := database.NewConnection(cfg)
	if err != nil {
		t.Skipf("Skipping test, cannot connect to database: %v", err)
		return
	}
	defer db.Close()

	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}

	search, err := NewSearchService(db, cache, cfg)
	if err != nil {
		t.Fatalf("Failed to create search service: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"こんにちは", "konnichiha"}, // 日语转罗马字
		{"テスト", "tesuto"},
		{"你好", "你好"},           // 中文保持不变
		{"hello", "hello"},        // 英文保持不变
	}

	for _, test := range tests {
		result := search.processQuery(test.input)
		if result != test.expected {
			t.Errorf("processQuery(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_cache",
			Expiration: 3600,
		},
	}

	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}

	search := &SearchService{
		cache: cache,
		config: cfg,
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"你好", "zh"},
		{"こんにちは", "ja"},
		{"hello", "en"},
	}

	for _, test := range tests {
		result := search.detectLanguage(test.input)
		if result != test.expected {
			t.Errorf("detectLanguage(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestSplitQuery(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_cache",
			Expiration: 3600,
		},
	}

	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}

	search := &SearchService{
		cache: cache,
		config: cfg,
	}

	tests := []struct {
		input    string
		expected []string
	}{
		{"hello", []string{"hello"}},
		{"hello world", []string{"hello world", "hello", "world"}},
		{"one two three", []string{"one two three", "one", "two", "three"}},
		{"", []string{}},
	}

	for _, test := range tests {
		result := search.splitQuery(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("splitQuery(%q) length mismatch: got %d, expected %d", test.input, len(result), len(test.expected))
		} else {
			for i, v := range result {
				if v != test.expected[i] {
					t.Errorf("splitQuery(%q)[%d] = %q; expected %q", test.input, i, v, test.expected[i])
				}
			}
		}
	}
}

func TestMergeAndDeduplicateResults(t *testing.T) {
	a := SearchResult{Name: "A", Nation: "US", Translation: "a"}
	a2 := SearchResult{Name: "A", Nation: "US", Translation: "a"}
	b := SearchResult{Name: "B", Nation: "UK", Translation: "b"}

	tests := []struct {
		name     string
		slices   [][]SearchResult
		expected int
	}{
		{"empty", [][]SearchResult{}, 0},
		{"single", [][]SearchResult{{a}}, 1},
		{"dedupe same", [][]SearchResult{{a}, {a2}}, 1},
		{"merge two", [][]SearchResult{{a}, {b}}, 2},
		{"dedupe across", [][]SearchResult{{a, b}, {a}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeAndDeduplicateResults(tt.slices...)
			if len(got) != tt.expected {
				t.Errorf("mergeAndDeduplicateResults() len = %d, want %d", len(got), tt.expected)
			}
		})
	}
}