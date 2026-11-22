package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database   DatabaseConfig   `yaml:"database"`
	Tables     TablesConfig     `yaml:"tables"`
	Fields     FieldsConfig     `yaml:"fields"`
	Server     ServerConfig     `yaml:"server"`
	Cache      CacheConfig      `yaml:"cache"`
	Log        LogConfig        `yaml:"log"`
	Security   SecurityConfig   `yaml:"security"`
	Turnstile  TurnstileConfig  `yaml:"turnstile"`
}

type TurnstileConfig struct {
	// Enabled 为 true 且 secret 非空时，/api/search 强制校验 Turnstile token
	Enabled   bool     `yaml:"enabled"`
	SiteKey   string   `yaml:"site_key"`    // 前端渲染用（公开值）
	SecretKey string   `yaml:"secret_key"`  // 建议留空改用环境变量 TURNSTILE_SECRET
	Action    string   `yaml:"action"`      // 前端 data-action，校验时必须匹配
	Hostnames []string `yaml:"hostnames"`   // siteverify 返回的 hostname 白名单，为空则不校验
}

type SecurityConfig struct {
	TrustedProxies []string `yaml:"trusted_proxies"` // 可信代理IP列表，为空时信任所有
	RateLimit      int      `yaml:"rate_limit"`      // 每分钟请求限制，0表示不限制
	MaxConcurrentSearches int `yaml:"max_concurrent_searches"` // 全局同时进行的未命中缓存搜索上限，<=0用默认4
	MaxConcurrentPerIP    int `yaml:"max_concurrent_per_ip"`   // 单IP同时进行的未命中缓存搜索上限，<=0用默认1
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type TablesConfig struct {
	Places string `yaml:"places"`
	Names  string `yaml:"names"`
}

type FieldsConfig struct {
	QueryField       string `yaml:"query_field"`
	QueryFieldClean  string `yaml:"query_field_clean"` // 预清洗查询列（括号已替换为空格），为空时回退到对query_field套REPLACE的查询
	NameField        string `yaml:"name_field"`
	NationField      string `yaml:"nation_field"`
	TranslationField string `yaml:"translation_field"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type CacheConfig struct {
	Directory string `yaml:"directory"`
	Expiration int   `yaml:"expiration"`
}

type LogConfig struct {
	Directory string `yaml:"directory"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Cache.Directory == "" {
		config.Cache.Directory = "./cache"
	}
	if config.Cache.Expiration == 0 {
		config.Cache.Expiration = 21600
	}
	if config.Log.Directory == "" {
		config.Log.Directory = "./logs"
	}
	// 并发闸门上限：0 视为未配置，使用默认值（全局4、单IP1）
	if config.Security.MaxConcurrentSearches == 0 {
		config.Security.MaxConcurrentSearches = 4
	}
	if config.Security.MaxConcurrentPerIP == 0 {
		config.Security.MaxConcurrentPerIP = 1
	}

	return &config, nil
}

func (c *Config) GetDSN() string {
	// 优先从环境变量读取密码，避免明文密码暴露在配置文件中
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = c.Database.Password // 兜底：使用配置文件中的密码
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.Username,
		password,
		c.Database.Host,
		c.Database.Port,
		c.Database.DBName)
}

// TurnstileSecret 返回 Turnstile secret key，环境变量 TURNSTILE_SECRET 优先于配置文件
func (c *Config) TurnstileSecret() string {
	if secret := os.Getenv("TURNSTILE_SECRET"); secret != "" {
		return secret
	}
	return c.Turnstile.SecretKey
}