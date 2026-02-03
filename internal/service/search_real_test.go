package service

import (
	"testing"

	"dict_new/config"
	"dict_new/internal/database"
	"dict_new/internal/util"
)

func TestRealSearch(t *testing.T) {
	// 使用开发环境配置进行测试
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql", // 使用系统数据库进行连接测试
		},
		Tables: config.TablesConfig{
			Places: "user",
			Names:  "db",
		},
		Fields: config.FieldsConfig{
			QueryField:      "Host",
			NameField:       "User",
			NationField:     "Password",
			TranslationField: "Select_priv",
		},
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_search_real_cache",
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

	// 测试查询处理 (use util functions directly)
	lang := util.DetectLanguage("hello")
	result := util.ProcessQueryByLanguage("hello", lang)
	if result != "hello" {
		t.Errorf("Expected 'hello', got '%s'", result)
	}

	// 测试语言检测
	if lang != "en" {
		t.Errorf("Expected 'en', got '%s'", lang)
	}

	// 测试查询分割
	parts := search.splitQuery("hello world")
	expected := []string{"hello world", "hello", "world"}
	if len(parts) != len(expected) {
		t.Errorf("Expected %d parts, got %d", len(expected), len(parts))
	} else {
		for i, part := range parts {
			if part != expected[i] {
				t.Errorf("Expected part[%d] = '%s', got '%s'", i, expected[i], part)
			}
		}
	}
}