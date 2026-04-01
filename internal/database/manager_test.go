package database

import (
	"testing"
)

// TestNewManager 测试管理器创建
func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.conns == nil {
		t.Error("conns map 未初始化")
	}
	if m.cfgs == nil {
		t.Error("cfgs map 未初始化")
	}
}

// TestBuildDSN 测试 DSN 构建
func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ConnectionConfig
		wantDriver string
		wantErr    bool
	}{
		{
			name: "MySQL",
			cfg: ConnectionConfig{
				Type: "mysql", Host: "localhost", Port: 3306,
				User: "root", Password: "pass", Database: "testdb",
			},
			wantDriver: "mysql",
		},
		{
			name: "TiDB（复用 MySQL 驱动）",
			cfg: ConnectionConfig{
				Type: "tidb", Host: "10.116.48.70", Port: 8400,
				User: "zztest_app", Password: "zztest_app@123", Database: "testdb",
			},
			wantDriver: "mysql",
		},
		{
			name: "StarRocks（复用 MySQL 驱动）",
			cfg: ConnectionConfig{
				Type: "starrocks", Host: "10.116.32.105", Port: 8031,
				User: "root", Password: "pass", Database: "testdb",
			},
			wantDriver: "mysql",
		},
		{
			name: "PostgreSQL",
			cfg: ConnectionConfig{
				Type: "postgres", Host: "localhost", Port: 5432,
				User: "postgres", Password: "pass", Database: "testdb",
			},
			wantDriver: "postgres",
		},
		{
			name: "SQLite",
			cfg: ConnectionConfig{
				Type: "sqlite", Database: "/tmp/test.db",
			},
			wantDriver: "sqlite3",
		},
		{
			name: "不支持的类型",
			cfg: ConnectionConfig{
				Type: "oracle",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, driver, err := buildDSN(&tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("期望报错但没有")
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望的错误: %v", err)
			}
			if driver != tt.wantDriver {
				t.Errorf("驱动名不匹配: got=%s want=%s", driver, tt.wantDriver)
			}
			if dsn == "" {
				t.Error("DSN 为空")
			}
		})
	}
}

// TestDisconnectNotConnected 测试断开未连接的连接不报错
func TestDisconnectNotConnected(t *testing.T) {
	m := NewManager()
	err := m.Disconnect("nonexistent")
	if err != nil {
		t.Errorf("断开不存在的连接应该返回 nil，但得到: %v", err)
	}
}

// TestGetDBNotExists 测试获取不存在的连接
func TestGetDBNotExists(t *testing.T) {
	m := NewManager()
	_, err := m.GetDB("nonexistent")
	if err == nil {
		t.Error("获取不存在的连接应该返回错误")
	}
}

// TestGetConfigNotExists 测试获取不存在的配置
func TestGetConfigNotExists(t *testing.T) {
	m := NewManager()
	_, ok := m.GetConfig("nonexistent")
	if ok {
		t.Error("获取不存在的配置应该返回 false")
	}
}

// TestIsMySQLCompatible 测试 MySQL 兼容性判断
func TestIsMySQLCompatible(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{"mysql", true},
		{"tidb", true},
		{"starrocks", true},
		{"postgres", false},
		{"sqlite", false},
		{"oracle", false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			got := IsMySQLCompatible(tt.dbType)
			if got != tt.want {
				t.Errorf("IsMySQLCompatible(%q) = %v, want %v", tt.dbType, got, tt.want)
			}
		})
	}
}
