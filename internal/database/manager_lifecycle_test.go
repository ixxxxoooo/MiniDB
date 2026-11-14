// @author ixxxxoooo

package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func sqliteCfg(t *testing.T, id string) *ConnectionConfig {
	t.Helper()
	return &ConnectionConfig{ID: id, Name: id, Type: SQLite, Database: filepath.Join(t.TempDir(), id+".db")}
}

// TestManagerConnectReconnectLifecycle 覆盖 Connect / Reconnect / Disconnect 共用建连逻辑后的行为。
func TestManagerConnectReconnectLifecycle(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	cfg := sqliteCfg(t, "a")

	// 没连过的连接不能被 Reconnect「复活」
	if err := m.Reconnect(cfg); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("未连接时 Reconnect 应返回 ErrDisconnected，得到 %v", err)
	}
	if _, err := m.GetDB("a"); err == nil {
		t.Fatal("Reconnect 失败后不应留下连接")
	}

	if err := m.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	first, _ := m.GetDB("a")
	if err := m.Ping(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}

	// Connect 同一个 ID 会替换旧连接池，旧池被关闭
	if err := m.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	second, _ := m.GetDB("a")
	if first == second {
		t.Error("重复 Connect 应换成新的连接池")
	}
	if err := first.Ping(); err == nil {
		t.Error("被替换的旧连接池应已关闭")
	}

	// Reconnect 在已连接时替换连接池与配置
	cfg2 := *cfg
	cfg2.Name = "renamed"
	if err := m.Reconnect(&cfg2); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.GetConfig("a"); got.Name != "renamed" {
		t.Errorf("Reconnect 后配置应更新，得到 %q", got.Name)
	}

	if err := m.Disconnect("a"); err != nil {
		t.Fatal(err)
	}
	if err := m.Reconnect(&cfg2); !errors.Is(err, ErrDisconnected) {
		t.Errorf("断开后 Reconnect 应返回 ErrDisconnected，得到 %v", err)
	}
}

// TestManagerFailedReconnectKeepsOldConnection 重连失败不能打断现有可用连接。
func TestManagerFailedReconnectKeepsOldConnection(t *testing.T) {
	m := NewManager()
	defer m.CloseAll()
	cfg := sqliteCfg(t, "b")
	if err := m.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	bad := *cfg
	bad.Database = filepath.Join(t.TempDir(), "no", "such", "dir", "x.db")
	if err := m.Reconnect(&bad); err == nil {
		t.Fatal("指向不存在目录的重连应失败")
	}
	if err := m.Ping(context.Background(), "b"); err != nil {
		t.Errorf("重连失败后旧连接应仍可用: %v", err)
	}
	if got, _ := m.GetConfig("b"); got.Database != cfg.Database {
		t.Error("重连失败后配置不应被改动")
	}
}

func TestManagerTestConnection(t *testing.T) {
	m := NewManager()
	if err := m.TestConnection(sqliteCfg(t, "c")); err != nil {
		t.Fatal(err)
	}
	if len(m.ConnectionIDs()) != 0 {
		t.Error("TestConnection 不应保留连接")
	}
	bad := &ConnectionConfig{ID: "d", Type: "oracle"}
	if err := m.TestConnection(bad); err == nil {
		t.Error("未知类型应返回错误")
	}
}
