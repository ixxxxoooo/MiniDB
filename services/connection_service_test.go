package services

import (
	"testing"

	"minidb/internal/database"
)

// TestValidateConnectionConfig 测试连接配置校验逻辑
func TestValidateConnectionConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     database.ConnectionConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "空 ID",
			cfg:     database.ConnectionConfig{ID: "", Type: "mysql", Host: "localhost", Port: 3306},
			wantErr: true,
			errMsg:  "连接配置 ID 不能为空",
		},
		{
			name:    "不支持的类型",
			cfg:     database.ConnectionConfig{ID: "test1", Type: "oracle", Host: "localhost", Port: 1521},
			wantErr: true,
			errMsg:  "不支持的数据库类型",
		},
		{
			name:    "空主机",
			cfg:     database.ConnectionConfig{ID: "test2", Type: "mysql", Host: "", Port: 3306},
			wantErr: true,
			errMsg:  "主机地址不能为空",
		},
		{
			name:    "端口为零",
			cfg:     database.ConnectionConfig{ID: "test3", Type: "mysql", Host: "localhost", Port: 0},
			wantErr: true,
			errMsg:  "端口号不合法",
		},
		{
			name:    "端口超限",
			cfg:     database.ConnectionConfig{ID: "test4", Type: "mysql", Host: "localhost", Port: 70000},
			wantErr: true,
			errMsg:  "端口号不合法",
		},
		{
			name:    "有效 MySQL 配置",
			cfg:     database.ConnectionConfig{ID: "test5", Type: "mysql", Host: "localhost", Port: 3306},
			wantErr: false,
		},
		{
			name:    "有效 PostgreSQL 配置",
			cfg:     database.ConnectionConfig{ID: "test6", Type: "postgres", Host: "127.0.0.1", Port: 5432},
			wantErr: false,
		},
		{
			name:    "SQLite 不要求 host 和 port",
			cfg:     database.ConnectionConfig{ID: "test7", Type: "sqlite", Host: "", Port: 0},
			wantErr: false,
		},
		{
			name:    "有效 TiDB 配置",
			cfg:     database.ConnectionConfig{ID: "test8", Type: "tidb", Host: "tidb.local", Port: 4000},
			wantErr: false,
		},
		{
			name:    "有效 StarRocks 配置",
			cfg:     database.ConnectionConfig{ID: "test9", Type: "starrocks", Host: "sr.local", Port: 9030},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConnectionConfig(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("期望返回错误，但得到 nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("错误信息不匹配\n  期望包含: %s\n  实际: %s", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("不期望返回错误，但得到: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
