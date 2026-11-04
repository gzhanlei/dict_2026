package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建测试配置文件
	testConfig := `
database:
  host: "localhost"
  port: 3306
  username: "test"
  password: "testtesttest"
  dbname: "dict"

tables:
  places: "places_new"
  names: "names_241220"

fields:
  query_field: "name_for_query_251202"
  query_field_clean: "name_for_query_clean"
  name_field: "name"
  nation_field: "nation"
  translation_field: "translation"

server:
  port: 8080

cache:
  directory: "./cache"
  expiration: 3600

log:
  directory: "./logs"
`

	// 写入临时配置文件
	tmpFile := "/tmp/test_config.yaml"
	err := os.WriteFile(tmpFile, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}
	defer os.Remove(tmpFile)

	// 测试加载配置
	config, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证数据库配置
	if config.Database.Host != "localhost" {
		t.Errorf("Expected database host to be 'localhost', got '%s'", config.Database.Host)
	}
	if config.Database.Username != "test" {
		t.Errorf("Expected database username to be 'test', got '%s'", config.Database.Username)
	}
	if config.Database.Password != "testtesttest" {
		t.Errorf("Expected database password to be 'testtesttest', got '%s'", config.Database.Password)
	}
	if config.Database.DBName != "dict" {
		t.Errorf("Expected database name to be 'dict', got '%s'", config.Database.DBName)
	}

	// 验证表配置
	if config.Tables.Places != "places_new" {
		t.Errorf("Expected places table to be 'places_new', got '%s'", config.Tables.Places)
	}
	if config.Tables.Names != "names_241220" {
		t.Errorf("Expected names table to be 'names_241220', got '%s'", config.Tables.Names)
	}

	// 验证字段配置
	if config.Fields.QueryField != "name_for_query_251202" {
		t.Errorf("Expected query field to be 'name_for_query_251202', got '%s'", config.Fields.QueryField)
	}
	if config.Fields.QueryFieldClean != "name_for_query_clean" {
		t.Errorf("Expected query field clean to be 'name_for_query_clean', got '%s'", config.Fields.QueryFieldClean)
	}
	if config.Fields.NameField != "name" {
		t.Errorf("Expected name field to be 'name', got '%s'", config.Fields.NameField)
	}

	// 验证服务器配置
	if config.Server.Port != 8080 {
		t.Errorf("Expected server port to be 8080, got %d", config.Server.Port)
	}

	// 验证缓存配置
	if config.Cache.Directory != "./cache" {
		t.Errorf("Expected cache directory to be './cache', got '%s'", config.Cache.Directory)
	}
	if config.Cache.Expiration != 3600 {
		t.Errorf("Expected cache expiration to be 3600, got %d", config.Cache.Expiration)
	}
}

func TestLoadConfigWithMissingFile(t *testing.T) {
	// 测试加载不存在的配置文件
	_, err := LoadConfig("/non/existent/file.yaml")
	if err == nil {
		t.Error("Expected error when loading non-existent config file, got nil")
	}
}

func TestGetDSN(t *testing.T) {
	config := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "test",
			Password: "testpass",
			DBName:   "testdb",
		},
	}

	expectedDSN := "test:testpass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := config.GetDSN()
	if dsn != expectedDSN {
		t.Errorf("Expected DSN to be '%s', got '%s'", expectedDSN, dsn)
	}
}