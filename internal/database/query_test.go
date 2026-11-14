package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

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
			want:    "WHERE `name` = ?",
		},
		{
			name:    "IS NULL",
			filters: []Filter{{Column: "email", Operator: "IS NULL"}},
			want:    "WHERE `email` IS NULL",
		},
		{
			name: "多条件",
			filters: []Filter{
				{Column: "age", Operator: ">", Value: "18"},
				{Column: "status", Operator: "=", Value: "active"},
			},
			want: "WHERE `age` > ? AND `status` = ?",
		},
		{
			name: "OR 连接",
			filters: []Filter{
				{Column: "age", Operator: ">", Value: "18"},
				{Column: "status", Operator: "=", Value: "active", Or: true},
				{Column: "vip", Operator: "=", Value: "1", Or: true},
			},
			want: "WHERE `age` > ? OR `status` = ? OR `vip` = ?",
		},
		{
			name: "任意列搜索",
			filters: []Filter{
				{Column: "__any", Operator: "LIKE", Value: "alice"},
			},
			want: "WHERE (CAST(`name` AS CHAR) LIKE ? OR CAST(`email` AS CHAR) LIKE ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var columns []string
			if tt.name == "任意列搜索" {
				columns = []string{"name", "email"}
			}
			result, args, err := buildWhereClause("mysql", tt.filters, columns)
			if err != nil {
				t.Fatal(err)
			}
			if result != tt.want {
				t.Errorf("WHERE 子句不匹配\n  got:  %s\n  want: %s", result, tt.want)
			}
			if tt.name == "任意列搜索" && len(args) != 2 {
				t.Errorf("任意列搜索应有 2 个参数，got %d", len(args))
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
			want:  "ORDER BY `id` ASC",
		},
		{
			name:  "多列排序",
			sorts: []Sort{{Column: "name", Direction: "ASC"}, {Column: "id", Direction: "DESC"}},
			want:  "ORDER BY `name` ASC, `id` DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildOrderByClause("mysql", tt.sorts)
			if err != nil {
				t.Fatal(err)
			}
			if result != tt.want {
				t.Errorf("ORDER BY 子句不匹配\n  got:  %s\n  want: %s", result, tt.want)
			}
		})
	}
}

// TestQuoteTable 测试表名引用
func TestQuoteTable(t *testing.T) {
	tests := []struct {
		dbType DBType
		dbName string
		table  string
		want   string
	}{
		{"mysql", "testdb", "users", "`testdb`.`users`"},
		{"mysql", "", "users", "`users`"},
		{"tidb", "testdb", "users", "`testdb`.`users`"},
		{"tidb", "", "users", "`users`"},
		{"starrocks", "testdb", "users", "`testdb`.`users`"},
		{"starrocks", "", "users", "`users`"},
		{"postgres", "", "users", `"users"`},
		{"sqlite", "", "users", `"users"`},
	}

	for _, tt := range tests {
		t.Run(string(tt.dbType)+"/"+tt.table, func(t *testing.T) {
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
	if got := QuoteTableName("tidb", "users"); got != "`users`" {
		t.Errorf("TiDB: got %s, want `users`", got)
	}
	if got := QuoteTableName("starrocks", "users"); got != "`users`" {
		t.Errorf("StarRocks: got %s, want `users`", got)
	}
	if got := QuoteTableName("postgres", "users"); got != `"users"` {
		t.Errorf(`got %s, want "users"`, got)
	}
}

func TestExecuteQueryPagedContextReturnsCancelled(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := ExecuteQueryPagedContext(ctx, db, "SELECT 1", 1, 10, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got result=%#v err=%v", result, err)
	}
	if result != nil {
		t.Fatalf("cancelled query should not return a result: %#v", result)
	}
}

func TestIsSelectQuery(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM t", true},
		{"  select 1", true},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"with x as (select 1) select * from x", true},
		{"UPDATE t SET a=1", false},
		{"SELECTX FROM t", false}, // 无词边界，不应识别为查询
		{"WITHIN GROUP", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsSelectQuery(c.sql); got != c.want {
			t.Errorf("IsSelectQuery(%q) = %v, want %v", c.sql, got, c.want)
		}
	}
}

func TestHasLimitClause(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT * FROM t LIMIT 10", true},
		{"SELECT * FROM t limit 5", true},
		{"SELECT * FROM t", false},
		// 列名/别名含 limit 子串，不应误判
		{"SELECT limited_user FROM t", false},
		{"SELECT * FROM t WHERE name = 'LIMITED'", false},
		{"SELECT t.limit_amount FROM t", false},
		// 尾部分号与空白
		{"SELECT * FROM t LIMIT 10;  ", true},
	}
	for _, c := range cases {
		if got := hasLimitClause(c.sql); got != c.want {
			t.Errorf("hasLimitClause(%q) = %v, want %v", c.sql, got, c.want)
		}
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

func TestHasLimitClauseOnlyChecksOutermostLevel(t *testing.T) {
	withLimit := []string{
		"SELECT * FROM t LIMIT 10",
		"select * from t limit 10 offset 5;",
		"SELECT * FROM t ORDER BY id\nLIMIT 1",
		"(SELECT 1) UNION (SELECT 2) LIMIT 3",
	}
	without := []string{
		"SELECT * FROM t",
		"SELECT * FROM t WHERE id IN (SELECT id FROM u LIMIT 1)",
		"SELECT * FROM (SELECT * FROM t LIMIT 5) x",
		"SELECT `limit` FROM t",
		"SELECT limited_user FROM t",
		"SELECT * FROM t WHERE note = 'limit 5'",
		"SELECT * FROM t -- limit 5",
		"SELECT * FROM t /* limit 5 */",
	}
	for _, q := range withLimit {
		if !hasLimitClause(q) {
			t.Errorf("应识别为已带 LIMIT: %q", q)
		}
	}
	for _, q := range without {
		if hasLimitClause(q) {
			t.Errorf("不应识别为带 LIMIT: %q", q)
		}
	}
}
