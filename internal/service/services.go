package service

import (
	"fmt"

	"dict_new/config"
	"dict_new/internal/database"
)

type Services struct {
	DB     *database.Connection
	Cache  *CacheService
	Search *SearchService
	Config *config.Config
}

func NewServices() (*Services, error) {
	// 加载配置
	cfg, err := config.LoadConfig("config/app.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 创建数据库连接
	db, err := database.NewConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 创建缓存服务
	cache, err := NewCacheService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache service: %w", err)
	}

	// 创建搜索服务
	search, err := NewSearchService(db, cache, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create search service: %w", err)
	}

	return &Services{
		DB:     db,
		Cache:  cache,
		Search: search,
		Config: cfg,
	}, nil
}

func (s *Services) Close() error {
	var errors []error

	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close database: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors while closing services: %v", errors)
	}

	return nil
}