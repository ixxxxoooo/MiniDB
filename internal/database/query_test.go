package database

import (
	"testing"
)

// TestGenerateInsertSQL 测试 INSERT 语句生成
func TestGenerateInsertSQL(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		row      map[string]interface{}
		wantPart string // 只验证包含关键部分
	}{
		{
			name:     "基础类型",
			table:    "users",
			row:      map[string]interface{}{"id": float64(1), "name": "alice"},
			wantPart: "INSERT INTO users",
		},
		{
			name:     "NULL 值",
			table:    "orders",
			row:      map[string]interface{}{"id": float64(1), "note": nil},
			wantPart: "NULL",
		},
		{
			name:     "布尔值",
			table:    "flags",
			row:      map[string]interface{}{"active": true, "deleted": false},
			wantPart: "INSERT INTO flags",
		},
		{
			name:     "单引号转义",
			table:    "comments",
			row:      map[string]interface{}{"text": "it's a test"},
			wantPart: "it''s a test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateInsertSQL(tt.table, tt.row)
			if result == "" {
				t.Error("生成的 SQL 为空")
			}
			if !contains(result, tt.wantPart) {
				t.Errorf("生成的 SQL 不包含期望部分\n  生成: %s\n  期望包含: %s", result, tt.wantPart)
			}
		})
	}
}

// TestBuildWhereClause 测试 WHERE 子句构建
func TestBuildWhereClause(t *testing.T) {
	tests := []struct {
		name    string
		filters []Filter
		want    string
	}{
		{
			name:    "空筛选",
			filters: []Filter{},
			want:    "",
		},
		{
			name:    "单个等值筛选",
			filters: []Filter{{Column: "name", Operator: "=", Value: "alice"}},
			want:    "WHERE name = 'alice'",
		},
		{
			name:    "IS NULL",
			filters: []Filter{{Column: "email", Operator: "IS NULL"}},
			want:    "WHERE email IS NULL",
		},
		{
			name: "多条件",
			filters: []Filter{
				{Column: "age", Operator: ">", Value: "18"},
				{Column: "status", Operator: "=", Value: "active"},
			},
			want: "WHERE age > '18' AND status = 'active'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildWhereClause(tt.filters)
			if result != tt.want {
				t.Errorf("WHERE 子句不匹配\n  got:  %s\n  want: %s", result, tt.want)
			}
		})
	}
}

// TestBuildOrderByClause 测试 ORDER BY 子句构建
func TestBuildOrderByClause(t *testing.T) {
	tests := []struct {
		name  string
		sorts []Sort
		want  string
	}{
		{
			name:  "空排序",
			sorts: []Sort{},
			want:  "",
		},
		{
			name:  "单列升序",
			sorts: []Sort{{Column: "id", Direction: "ASC"}},
			want:  "ORDER BY id ASC",
		},
		{
			name:  "多列排序",
			sorts: []Sort{{Column: "name", Direction: "ASC"}, {Column: "id", Direction: "DESC"}},
			want:  "ORDER BY name ASC, id DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildOrderByClause(tt.sorts)
			if result != tt.want {
				t.Errorf("ORDER BY 子句不匹配\n  got:  %s\n  want: %s", result, tt.want)
			}
		})
	}
}

// TestQuoteTable 测试表名引用
func TestQuoteTable(t *testing.T) {
	tests := []struct {
		dbType string
		dbName string
		table  string
		want   string
	}{
		{"mysql", "testdb", "users", "`testdb`.`users`"},
		{"mysql", "", "users", "`users`"},
		{"postgres", "", "users", `"users"`},
		{"sqlite", "", "users", "users"},
	}

	for _, tt := range tests {
		t.Run(tt.dbType+"/"+tt.table, func(t *testing.T) {
			result := quoteTable(tt.dbType, tt.dbName, tt.table)
			if result != tt.want {
				t.Errorf("表名引用不匹配\n  got:  %s\n  want: %s", result, tt.want)
			}
		})
	}
}

// TestQuoteTableName 测试导出的表名引用函数
func TestQuoteTableName(t *testing.T) {
	if got := QuoteTableName("mysql", "users"); got != "`users`" {
		t.Errorf("got %s, want `users`", got)
	}
	if got := QuoteTableName("postgres", "users"); got != `"users"` {
		t.Errorf(`got %s, want "users"`, got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
