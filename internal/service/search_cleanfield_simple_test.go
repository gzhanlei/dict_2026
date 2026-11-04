package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// 钉住三种查询变体的最终 SQL（zh / 预清洗列 / REPLACE 回退）。
// 重构前后这些字符串必须完全一致：它们是与数据库的实际契约。

func TestBuildSearchQuerySQL(t *testing.T) {
	tests := []struct {
		name      string
		whereExpr string
		want      string
	}{
		{
			name:      "zh translation field",
			whereExpr: "translation",
			want: "SELECT DISTINCT name, nation, translation FROM names_260202 " +
				"WHERE translation LIKE ? ORDER BY name LIMIT 100",
		},
		{
			name:      "clean field",
			whereExpr: "name_for_query_clean",
			want: "SELECT DISTINCT name, nation, translation FROM names_260202 " +
				"WHERE name_for_query_clean LIKE ? ORDER BY name LIMIT 100",
		},
		{
			name:      "replace fallback",
			whereExpr: "REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')",
			want: "SELECT DISTINCT name, nation, translation FROM names_260202 " +
				"WHERE REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ') LIKE ? " +
				"ORDER BY name LIMIT 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchQuerySQL("names_260202", tt.whereExpr, "name", "nation", "translation")
			if got != tt.want {
				t.Errorf("buildSearchQuerySQL() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestParenCleanedExpr 验证回退用的括号清洗表达式构造
func TestParenCleanedExpr(t *testing.T) {
	got := parenCleanedExpr("name_for_query_251202")
	want := "REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')"
	if got != want {
		t.Errorf("parenCleanedExpr() = %q, want %q", got, want)
	}
}

// TestIsUnknownColumnError 验证 MySQL 1054（列不存在）错误识别，
// 用于预清洗列缺失时的回退判断
func TestIsUnknownColumnError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("some error"), false},
		{"1054 unknown column", &mysql.MySQLError{Number: 1054, Message: "Unknown column 'name_for_query_clean' in 'where clause'"}, true},
		{"1146 table not exist", &mysql.MySQLError{Number: 1146, Message: "Table 'dict.names' doesn't exist"}, false},
		{"wrapped 1054", fmt.Errorf("query failed: %w", &mysql.MySQLError{Number: 1054, Message: "Unknown column"}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnknownColumnError(tt.err); got != tt.expected {
				t.Errorf("isUnknownColumnError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}
