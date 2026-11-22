package service

import (
	"fmt"
	"log"

	"dict_new/config"
	"dict_new/internal/database"
)

type Services struct {
	DB        *database.Connection
	Cache     *CacheService
	Search    *SearchService
	Turnstile *TurnstileService
	Config    *config.Config
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

	// 启动后台过期缓存清理
	cache.StartCleanup()

	// 创建搜索服务
	search, err := NewSearchService(db, cache, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create search service: %w", err)
	}

	// 创建 Turnstile 校验服务
	turnstile := NewTurnstileService(cfg, "")

	if cfg.Turnstile.Enabled && cfg.TurnstileSecret() == "" {
		// 配置不完整：校验会跳过（fail-open），提前打日志避免误以为已受保护
		log.Printf("警告: turnstile.enabled=true 但未配置 secret（检查 config/app.yaml 的 turnstile.secret_key 或环境变量 TURNSTILE_SECRET），人机校验未生效")
	}

	return &Services{
		DB:        db,
		Cache:     cache,
		Search:    search,
		Turnstile: turnstile,
		Config:    cfg,
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