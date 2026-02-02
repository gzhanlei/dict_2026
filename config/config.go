package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Tables   TablesConfig   `yaml:"tables"`
	Fields   FieldsConfig   `yaml:"fields"`
	Server   ServerConfig   `yaml:"server"`
	Cache    CacheConfig    `yaml:"cache"`
	Log      LogConfig      `yaml:"log"`
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
	QueryField      string `yaml:"query_field"`
	NameField       string `yaml:"name_field"`
	NationField     string `yaml:"nation_field"`
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
		config.Cache.Expiration = 3600
	}
	if config.Log.Directory == "" {
		config.Log.Directory = "./logs"
	}

	return &config, nil
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.Username,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.DBName)
}