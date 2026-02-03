package service

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dict_new/config"
)

type CacheData struct {
	Query               string                   `json:"query"`
	Type                string                   `json:"type"`
	Language            string                   `json:"language"`
	Timestamp           int64                    `json:"timestamp"`
	Data                map[string]interface{}   `json:"data"`
	Tabs                []string                 `json:"tabs,omitempty"`
	TabResults          map[string][]SearchResult `json:"tab_results,omitempty"`
	FullQueryNoRecord   string                   `json:"full_query_no_record,omitempty"`
}

type CacheService struct {
	config     *config.Config
	cacheDir   string
	expiration int64
}

func NewCacheService(cfg *config.Config) (*CacheService, error) {
	// 确保缓存目录存在
	cacheDir := cfg.Cache.Directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &CacheService{
		config:     cfg,
		cacheDir:   cacheDir,
		expiration: int64(cfg.Cache.Expiration),
	}, nil
}

// 生成缓存键（文件名，如 8efa226d....json）
func (c *CacheService) GenerateKey(query, searchType, language string) string {
	keyStr := fmt.Sprintf("%s_%s_%s", query, searchType, language)
	hash := md5.Sum([]byte(keyStr))
	return fmt.Sprintf("%x.json", hash)
}

// keyPath 按键前两字符分子目录，避免单目录文件过多导致限制或性能问题
func (c *CacheService) keyPath(key string) string {
	if len(key) >= 2 {
		return filepath.Join(c.cacheDir, key[:2], key)
	}
	return filepath.Join(c.cacheDir, key)
}

// 获取缓存（先查子目录，再回退到根目录以兼容旧缓存）
func (c *CacheService) Get(key string) (*CacheData, error) {
	filePath := c.keyPath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		// 兼容：旧缓存可能在根目录
		filePath = filepath.Join(c.cacheDir, key)
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read cache file: %w", err)
		}
	}

	var cacheData CacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	// 检查是否过期
	if c.isExpired(cacheData.Timestamp) {
		os.Remove(filePath) // 删除过期文件
		return nil, fmt.Errorf("cache data expired")
	}

	return &cacheData, nil
}

// 设置缓存
func (c *CacheService) Set(key string, data *CacheData) error {
	data.Timestamp = time.Now().Unix()
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	filePath := c.keyPath(key)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create cache subdir: %w", err)
	}
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// 检查缓存是否过期
func (c *CacheService) isExpired(timestamp int64) bool {
	return time.Now().Unix()-timestamp > c.expiration
}

// listCacheFiles returns all cache files from both subdirectory and flat layouts
func (c *CacheService) listCacheFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(c.cacheDir, "*", "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list cache files: %w", err)
	}
	flat, _ := filepath.Glob(filepath.Join(c.cacheDir, "*.json"))
	return append(files, flat...), nil
}

// 清理过期缓存（子目录 + 根目录旧文件）
func (c *CacheService) CleanExpired() error {
	files, err := c.listCacheFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue // 跳过读取失败的文件
		}

		var cacheData CacheData
		if err := json.Unmarshal(data, &cacheData); err != nil {
			os.Remove(file) // 删除损坏的文件
			continue
		}

		if c.isExpired(cacheData.Timestamp) {
			os.Remove(file) // 删除过期的文件
		}
	}

	return nil
}

// 清空所有缓存（子目录 + 根目录旧文件）
func (c *CacheService) Clear() error {
	files, err := c.listCacheFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		os.Remove(file)
	}

	return nil
}