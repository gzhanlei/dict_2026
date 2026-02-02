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
		t.Fatalf("Failed to create database connection: %v", err)
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
		t.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	conn := &Connection{DB: db.DB, Config: cfg}

	// 测试获取表名
	placesTable := conn.GetPlacesTable()
	namesTable := conn.GetNamesTable()

	if placesTable != "user" {
		t.Errorf("Expected places table to be 'user', got '%s'", placesTable)
	}
	if namesTable != "db" {
		t.Errorf("Expected names table to be 'db', got '%s'", namesTable)
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

	conn := &Connection{Config: cfg}

	// 测试获取字段名
	queryField := conn.GetQueryField()
	nameField := conn.GetNameField()
	nationField := conn.GetNationField()
	translationField := conn.GetTranslationField()

	if queryField != "test_query" {
		t.Errorf("Expected query field to be 'test_query', got '%s'", queryField)
	}
	if nameField != "test_name" {
		t.Errorf("Expected name field to be 'test_name', got '%s'", nameField)
	}
	if nationField != "test_nation" {
		t.Errorf("Expected nation field to be 'test_nation', got '%s'", nationField)
	}
	if translationField != "test_translation" {
		t.Errorf("Expected translation field to be 'test_translation', got '%s'", translationField)
	}
}