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