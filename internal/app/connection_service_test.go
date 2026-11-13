package app

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"minidb/internal/database"
	"minidb/internal/storage"
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

func newTestConnectionService(t *testing.T) *ConnectionService {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // 密钥文件随 HOME 隔离
	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewConnectionService(store, database.NewManager(), nil)
}

func TestGetConnectionsNeverReturnsPassword(t *testing.T) {
	svc := newTestConnectionService(t)
	if err := svc.SaveConnection(database.ConnectionConfig{ID: "c1", Name: "n", Type: "mysql", Host: "h", Port: 3306, Password: "s3cret"}); err != nil {
		t.Fatal(err)
	}
	conns, err := svc.GetConnections()
	if err != nil || len(conns) != 1 {
		t.Fatalf("conns=%v err=%v", conns, err)
	}
	if conns[0].Password != "" {
		t.Fatal("前端可见的连接列表不应包含明文密码")
	}
	if !conns[0].HasPassword {
		t.Fatal("应通过 HasPassword 表示已保存密码")
	}
	// 后端内部仍能拿到真实密码（用于连接与备份）
	if got := svc.storedPassword("c1"); got != "s3cret" {
		t.Fatalf("storedPassword=%q", got)
	}
}

func TestSaveConnectionKeepsOrClearsPassword(t *testing.T) {
	svc := newTestConnectionService(t)
	base := database.ConnectionConfig{ID: "c1", Name: "n", Type: "mysql", Host: "h", Port: 3306, Password: "s3cret"}
	if err := svc.SaveConnection(base); err != nil {
		t.Fatal(err)
	}

	// 编辑时不改密码：前端发空密码，应沿用已保存的
	edited := base
	edited.Password, edited.Name = "", "renamed"
	edited.HasPassword = true
	if err := svc.SaveConnection(edited); err != nil {
		t.Fatal(err)
	}
	if got := svc.storedPassword("c1"); got != "s3cret" {
		t.Fatalf("空密码保存应保持原密码，实际 %q", got)
	}

	// 输入新密码：覆盖
	changed := edited
	changed.Password = "n3w"
	if err := svc.SaveConnection(changed); err != nil {
		t.Fatal(err)
	}
	if got := svc.storedPassword("c1"); got != "n3w" {
		t.Fatalf("应更新为新密码，实际 %q", got)
	}

	// 显式清除
	cleared := edited
	cleared.ClearPassword = true
	if err := svc.SaveConnection(cleared); err != nil {
		t.Fatal(err)
	}
	if got := svc.storedPassword("c1"); got != "" {
		t.Fatalf("显式清除后密码应为空，实际 %q", got)
	}

	// 展示用标记不应被持久化
	var raw database.ConnectionConfig
	if err := svc.store.Get("connections", "c1", &raw); err != nil {
		t.Fatal(err)
	}
	if raw.HasPassword || raw.ClearPassword {
		t.Fatal("HasPassword/ClearPassword 不应写入存储")
	}
}

func TestBackupEncryptionRoundTrip(t *testing.T) {
	plain := []byte(`{"version":1,"connections":[{"id":"c1","password":"s3cret"}]}`)
	enc, err := encryptBackup(plain, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), "s3cret") {
		t.Fatal("加密后的文件不应包含明文密码")
	}
	if !isEncryptedBackup(enc) || isEncryptedBackup(plain) {
		t.Fatal("加密标记识别错误")
	}
	got, err := decryptBackup(enc, "correct horse")
	if err != nil || string(got) != string(plain) {
		t.Fatalf("解密失败: %v %s", err, got)
	}
	if _, err := decryptBackup(enc, "wrong"); !errors.Is(err, ErrBackupPassphraseWrong) {
		t.Fatalf("错误口令应返回 ErrBackupPassphraseWrong，实际 %v", err)
	}
	if _, err := decryptBackup(enc, ""); !errors.Is(err, ErrBackupPassphraseRequired) {
		t.Fatalf("缺少口令应返回 ErrBackupPassphraseRequired，实际 %v", err)
	}
}

func TestSSHSecretIsMaskedEncryptedAndKept(t *testing.T) {
	svc := newTestConnectionService(t)
	base := database.ConnectionConfig{
		ID: "c1", Name: "n", Type: "mysql", Host: "db.internal", Port: 3306,
		SSH: database.SSHConfig{Enabled: true, Host: "jump", Port: 22, User: "ops", AuthType: "password", Secret: "ssh-s3cret"},
	}
	if err := svc.SaveConnection(base); err != nil {
		t.Fatal(err)
	}

	// 落库的是密文
	var raw database.ConnectionConfig
	if err := svc.store.Get("connections", "c1", &raw); err != nil {
		t.Fatal(err)
	}
	if raw.SSH.Secret == "" || strings.Contains(raw.SSH.Secret, "ssh-s3cret") {
		t.Fatalf("SSH 密码应加密存储: %q", raw.SSH.Secret)
	}
	if raw.SSH.HasSecret || raw.SSH.ClearSecret {
		t.Fatal("展示/清除标记不应持久化")
	}

	// 前端列表不含明文，仅 HasSecret
	conns, _ := svc.GetConnections()
	if conns[0].SSH.Secret != "" || !conns[0].SSH.HasSecret {
		t.Fatalf("列表不应返回 SSH 密码: %+v", conns[0].SSH)
	}

	// 编辑时 Secret 留空 → 沿用
	edited := base
	edited.SSH.Secret, edited.Name = "", "renamed"
	if err := svc.SaveConnection(edited); err != nil {
		t.Fatal(err)
	}
	if got := svc.storedSSHSecret("c1"); got != "ssh-s3cret" {
		t.Fatalf("留空应沿用，实际 %q", got)
	}

	// 备份内部读取能拿到明文（用于导出）
	all, err := svc.loadConnectionsWithSecrets()
	if err != nil || all[0].SSH.Secret != "ssh-s3cret" {
		t.Fatalf("内部读取应得到明文: %v %+v", err, all)
	}

	// 显式清除
	cleared := edited
	cleared.SSH.ClearSecret = true
	if err := svc.SaveConnection(cleared); err != nil {
		t.Fatal(err)
	}
	if got := svc.storedSSHSecret("c1"); got != "" {
		t.Fatalf("清除后应为空，实际 %q", got)
	}
}

func TestValidateSSHConfig(t *testing.T) {
	ok := database.ConnectionConfig{ID: "a", Type: "mysql", Host: "h", Port: 3306,
		SSH: database.SSHConfig{Enabled: true, Host: "j", Port: 22, User: "u", AuthType: "password"}}
	if err := validateConnectionConfig(ok); err != nil {
		t.Fatalf("合法配置被拒绝: %v", err)
	}
	cases := map[string]func(c *database.ConnectionConfig){
		"缺主机":       func(c *database.ConnectionConfig) { c.SSH.Host = " " },
		"缺用户":       func(c *database.ConnectionConfig) { c.SSH.User = "" },
		"端口非法":      func(c *database.ConnectionConfig) { c.SSH.Port = 0 },
		"密钥缺路径":     func(c *database.ConnectionConfig) { c.SSH.AuthType = "key"; c.SSH.KeyPath = "" },
		"sqlite 不可": func(c *database.ConnectionConfig) { c.Type = "sqlite" },
	}
	for name, mutate := range cases {
		c := ok
		mutate(&c)
		if err := validateConnectionConfig(c); err == nil {
			t.Errorf("%s: 应被拒绝", name)
		}
	}
}

func TestSaveConnectionAssignsID(t *testing.T) {
	svc := newTestConnectionService(t)
	cfg := database.ConnectionConfig{Name: "n", Type: "sqlite", Database: "/tmp/x.db"}
	if err := svc.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	conns, _ := svc.GetConnections()
	if len(conns) != 2 || conns[0].ID == "" || conns[0].ID == conns[1].ID || len(conns[0].ID) != 32 {
		t.Fatalf("two new connections need two distinct IDs: %+v", conns)
	}
}

func TestListingNeedsNoDecryption(t *testing.T) {
	svc := newTestConnectionService(t)
	if err := svc.SaveConnection(database.ConnectionConfig{ID: "a", Name: "a", Type: "mysql", Host: "h", Port: 1, User: "u", Password: "pw"}); err != nil {
		t.Fatal(err)
	}
	// Lose the key: a different one now encrypts and decrypts.
	storage.SetKeyProvider(fixedKey(bytes.Repeat([]byte{9}, 32)))
	defer storage.SetKeyProvider(storage.FileKeyProvider{})
	conns, err := svc.GetConnections()
	if err != nil || len(conns) != 1 || !conns[0].HasPassword || conns[0].Password != "" {
		t.Fatalf("list must survive a key mismatch: %+v %v", conns, err)
	}
}

type fixedKey []byte

func (k fixedKey) Key() ([]byte, error) { return k, nil }
