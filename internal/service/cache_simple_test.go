package service

import (
	"os"
	"testing"

	"dict_new/config"
)

func TestCacheSimpleOperations(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_cache_simple",
			Expiration: 3600,
		},
	}

	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}
	defer os.RemoveAll(cfg.Cache.Directory)

	// 测试缓存键生成
	key1 := cache.GenerateKey("hello", "names", "en")
	key2 := cache.GenerateKey("hello", "names", "en")
	
	if key1 != key2 {
		t.Errorf("Expected same key for same input, got %s and %s", key1, key2)
	}

	// 测试基本缓存操作
	testData := &CacheData{
		Query:    "test",
		Type:     "names",
		Language: "en",
		Data:     map[string]interface{}{"names": []interface{}{"test1"}},
	}

	err = cache.Set(key1, testData)
	if err != nil {
		t.Errorf("Failed to set cache: %v", err)
	}

	result, err := cache.Get(key1)
	if err != nil {
		t.Errorf("Failed to get cache: %v", err)
	}

	if result.Query != testData.Query {
		t.Errorf("Expected query %s, got %s", testData.Query, result.Query)
	}

	// 测试清理
	err = cache.Clear()
	if err != nil {
		t.Errorf("Failed to clear cache: %v", err)
	}
}

// TestCacheEmptyTabResultsRoundtrip 回归：both 模式下无人名结果（tab map为空）的缓存，
// 序列化时 TabResults 不能被 omitempty 省略，否则回读为 nil 被 hasBothTabData
// 判为旧格式缓存而每次强制回库（爬虫查地名时缓存完全失效的根因）
func TestCacheEmptyTabResultsRoundtrip(t *testing.T) {
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Directory:  "/tmp/test_cache_tabs",
			Expiration: 3600,
		},
	}
	cache, err := NewCacheService(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache service: %v", err)
	}
	defer os.RemoveAll(cfg.Cache.Directory)

	key := cache.GenerateKey("shreveport", "both", "en")
	emptyTabs := make(map[string][]SearchResult) // 非nil空map
	err = cache.Set(key, &CacheData{
		Query:      "shreveport",
		Type:       "both",
		Language:   "en",
		Data:       map[string]interface{}{"places": []SearchResult{{Name: "Shreveport"}}},
		TabResults: emptyTabs,
	})
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	got, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}
	if !hasBothTabData(got) {
		t.Error("empty TabResults map should survive JSON roundtrip and pass hasBothTabData")
	}
}