package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()
	// 用文件库而不是 :memory:，保证连接池里的每条连接看到的是同一份数据
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开 sqlite 失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// 会话级状态（这里用 SQLite 临时表代表）只在同一条连接上可见，
// WithSession 必须保证 fn 内的所有语句都落在这一条连接上。
func TestWithSessionPinsSingleConnection(t *testing.T) {
	db := openTestSQLite(t)
	db.SetMaxOpenConns(4)

	err := WithSession(context.Background(), db, "sqlite", "", func(s *Session) error {
		if _, err := s.Exec("CREATE TEMP TABLE session_only (v INTEGER)"); err != nil {
			return err
		}
		if _, err := s.Exec("INSERT INTO session_only VALUES (42)"); err != nil {
			return err
		}
		var v int
		if err := s.QueryRow("SELECT v FROM session_only").Scan(&v); err != nil {
			return err
		}
		if v != 42 {
			t.Fatalf("同一会话内应读到 42，实际 %d", v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithSession 失败: %v", err)
	}
}

func TestUpdateAndDeleteRowRejectUnmatchedPrimaryKey(t *testing.T) {
	db := openTestSQLite(t)
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users VALUES (1, 'alice')"); err != nil {
		t.Fatal(err)
	}

	// 主键匹配：成功
	if err := UpdateRow(db, "sqlite", "", "users", map[string]interface{}{"id": 1}, map[string]interface{}{"name": "bob"}); err != nil {
		t.Fatalf("匹配主键的更新应成功: %v", err)
	}
	// 值未变化但行存在：不应被误判为失败
	if err := UpdateRow(db, "sqlite", "", "users", map[string]interface{}{"id": 1}, map[string]interface{}{"name": "bob"}); err != nil {
		t.Fatalf("值未变化的更新应成功: %v", err)
	}
	// 主键不匹配：必须报错，而不是静默成功
	err := UpdateRow(db, "sqlite", "", "users", map[string]interface{}{"id": 999}, map[string]interface{}{"name": "x"})
	if err == nil || !strings.Contains(err.Error(), "未匹配") {
		t.Fatalf("主键不匹配的更新应报错，实际: %v", err)
	}
	err = DeleteRow(db, "sqlite", "", "users", map[string]interface{}{"id": 999})
	if err == nil || !strings.Contains(err.Error(), "未匹配") {
		t.Fatalf("主键不匹配的删除应报错，实际: %v", err)
	}
	if err := DeleteRow(db, "sqlite", "", "users", map[string]interface{}{"id": 1}); err != nil {
		t.Fatalf("匹配主键的删除应成功: %v", err)
	}
}

func TestNormalizeScannedValue(t *testing.T) {
	ts := time.Date(2024, 3, 5, 10, 20, 30, 123456000, time.FixedZone("CST", 8*3600))
	tests := []struct {
		name     string
		val      interface{}
		typeName string
		want     interface{}
	}{
		{"nil", nil, "INT", nil},
		{"文本字节", []byte("hello 你好"), "VARCHAR", "hello 你好"},
		{"二进制字节转十六进制", []byte{0xff, 0xfe, 0x00}, "BLOB", "0xfffe00"},
		{"BIT 转十进制", []byte{0x01}, "BIT", "1"},
		{"安全范围整数保持数值", int64(9007199254740991), "BIGINT", int64(9007199254740991)},
		{"雪花 ID 转字符串", int64(1765432109876543210), "BIGINT", "1765432109876543210"},
		{"负的大整数转字符串", int64(-9007199254740993), "BIGINT", "-9007199254740993"},
		{"大无符号整数转字符串", uint64(18446744073709551615), "UNSIGNED BIGINT", "18446744073709551615"},
		{"DATE", ts, "DATE", "2024-03-05"},
		{"DATETIME 保留微秒", ts, "DATETIME", "2024-03-05 10:20:30.123456"},
		{"TIMESTAMPTZ 带时区", ts, "TIMESTAMPTZ", "2024-03-05 10:20:30.123456+08:00"},
		{"整秒不带小数", ts.Truncate(time.Second), "TIMESTAMP", "2024-03-05 10:20:30"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeScannedValue(tt.val, tt.typeName); got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// 超出 JS 安全整数范围的值端到端应以字符串返回（SQLite 驱动直接返回 int64）。
func TestQueryReturnsBigIntegersAsStrings(t *testing.T) {
	db := openTestSQLite(t)
	if _, err := db.Exec("CREATE TABLE ids (id INTEGER PRIMARY KEY, small INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO ids VALUES (1765432109876543210, 7)"); err != nil {
		t.Fatal(err)
	}
	res, err := ExecuteQueryRaw(db, "SELECT id, small FROM ids")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("期望 1 行，实际 %d", len(res.Rows))
	}
	if got := res.Rows[0]["id"]; got != "1765432109876543210" {
		t.Fatalf("大整数应为字符串，实际 %#v", got)
	}
	if got := res.Rows[0]["small"]; got != int64(7) {
		t.Fatalf("小整数应保持数值，实际 %#v", got)
	}
}

func TestGetDBForDatabaseReturnsMainPoolForNonPostgres(t *testing.T) {
	m := NewManager()
	cfg := &ConnectionConfig{ID: "c1", Type: "sqlite", Database: filepath.Join(t.TempDir(), "a.db")}
	if err := m.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	defer m.CloseAll()
	main, _ := m.GetDB("c1")
	got, err := m.GetDBForDatabase("c1", "whatever")
	if err != nil || got != main {
		t.Fatalf("非 PostgreSQL 应复用主连接池: %v", err)
	}
	if _, err := m.GetDBForDatabase("missing", "x"); err == nil {
		t.Fatal("不存在的连接应报错")
	}
}

// SQLite 的 query_only 要在会话内真的拒绝写入，并且离开后必须复位，不能漏给连接池里的下一个使用者。
func TestSessionWithQueryOnlyBlocksWritesAndResets(t *testing.T) {
	db := openTestSQLite(t)
	db.SetMaxOpenConns(1) // 强制复用同一条物理连接，才能验证复位
	if _, err := db.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	err := WithSession(context.Background(), db, "sqlite", "", func(s *Session) error {
		return s.WithQueryOnly(func() error {
			if _, err := s.Exec("INSERT INTO t VALUES (1)"); err == nil {
				t.Error("query_only 下的 INSERT 应被拒绝")
			}
			if _, err := s.Exec("CREATE TABLE x (id INTEGER)"); err == nil {
				t.Error("query_only 下的 DDL 应被拒绝")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO t VALUES (2)"); err != nil {
		t.Fatalf("离开 query_only 后应能写入: %v", err)
	}
}
