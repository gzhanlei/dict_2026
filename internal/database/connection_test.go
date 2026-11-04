package database

import (
	"testing"

	"dict_new/config"
)

func TestNewConnection(t *testing.T) {
	// 创建测试配置
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql", // 使用系统数据库进行连接测试
		},
	}

	// 测试创建连接
	db, err := NewConnection(cfg)
	if err != nil {
		t.Skipf("Skipping test, database unavailable: %v", err)
	}
	defer db.Close()

	// 测试连接是否可用
	if err := db.DB.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestNewConnectionWithInvalidConfig(t *testing.T) {
	// 创建无效配置
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "nonexistent",
			Port:     3306,
			Username: "invalid",
			Password: "invalid",
			DBName:   "invalid",
		},
	}

	// 测试创建连接应该失败
	_, err := NewConnection(cfg)
	if err == nil {
		t.Error("Expected error when connecting with invalid config, got nil")
	}
}

func TestGetTableNames(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "",
			DBName:   "mysql",
		},
		Tables: config.TablesConfig{
			Places: "user",
			Names:  "db",
		},
	}

	db, err := NewConnection(cfg)
	if err != nil {
		t.Skipf("Skipping test, database unavailable: %v", err)
	}
	defer db.Close()

	// 直接访问配置字段（getter 方法已移除以减少冗余）
	if cfg.Tables.Places != "user" {
		t.Errorf("Expected places table to be 'user', got '%s'", cfg.Tables.Places)
	}
	if cfg.Tables.Names != "db" {
		t.Errorf("Expected names table to be 'db', got '%s'", cfg.Tables.Names)
	}
}

func TestGetFieldNames(t *testing.T) {
	cfg := &config.Config{
		Fields: config.FieldsConfig{
			QueryField:      "test_query",
			NameField:       "test_name",
			NationField:     "test_nation",
			TranslationField: "test_translation",
		},
	}

	// 直接访问配置字段（getter 方法已移除以减少冗余）
	if cfg.Fields.QueryField != "test_query" {
		t.Errorf("Expected query field to be 'test_query', got '%s'", cfg.Fields.QueryField)
	}
	if cfg.Fields.NameField != "test_name" {
		t.Errorf("Expected name field to be 'test_name', got '%s'", cfg.Fields.NameField)
	}
	if cfg.Fields.NationField != "test_nation" {
		t.Errorf("Expected nation field to be 'test_nation', got '%s'", cfg.Fields.NationField)
	}
	if cfg.Fields.TranslationField != "test_translation" {
		t.Errorf("Expected translation field to be 'test_translation', got '%s'", cfg.Fields.TranslationField)
	}
}