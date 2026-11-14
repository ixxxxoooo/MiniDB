package app

import (
	"path/filepath"
	"strings"
	"testing"

	"minidb/internal/storage"
)

func newTestSettings(t *testing.T) (*SettingsService, *storage.Store) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // 密钥文件随 HOME 隔离
	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewSettingsService(store), store
}

func TestGetAIConfigDefaultsWhenNothingSaved(t *testing.T) {
	svc, _ := newTestSettings(t)
	cfg, err := svc.GetAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.SystemPrompt == "" || cfg.APIKey != "" {
		t.Fatalf("没存过时应给出默认值且没有密钥: %+v", cfg)
	}
}

func TestSaveAIConfigEncryptsSecretsAtRest(t *testing.T) {
	svc, store := newTestSettings(t)
	err := svc.SaveAIConfig(AIConfig{APIKey: "sk-secret", Headers: map[string]string{"X-Token": "tok-123"}})
	if err != nil {
		t.Fatal(err)
	}
	var raw AIConfig
	if err := store.Get("settings", "ai_config", &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw.APIKey, "sk-secret") || !storage.IsEncryptedString(raw.APIKey) {
		t.Fatalf("API Key 落库时应加密: %q", raw.APIKey)
	}
	if h := raw.Headers["X-Token"]; strings.Contains(h, "tok-123") || !storage.IsEncryptedString(h) {
		t.Fatalf("请求头的值落库时应加密: %q", h)
	}
	got, err := svc.GetAIConfig()
	if err != nil || got.APIKey != "sk-secret" || got.Headers["X-Token"] != "tok-123" {
		t.Fatalf("读回应是明文: %+v, %v", got, err)
	}
	// 空的基础地址、模型、系统提示词会被补成默认值
	if got.BaseURL == "" || got.Model == "" || got.SystemPrompt == "" {
		t.Fatalf("应补默认值: %+v", got)
	}
}

func TestGetAIConfigMigratesLegacyPlaintext(t *testing.T) {
	svc, store := newTestSettings(t)
	// 旧版直接把明文存进去
	if err := store.Put("settings", "ai_config", AIConfig{BaseURL: "u", Model: "m", APIKey: "plain-key", Headers: map[string]string{"H": "plain-h"}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetAIConfig()
	if err != nil || got.APIKey != "plain-key" || got.Headers["H"] != "plain-h" {
		t.Fatalf("读到明文应原样返回: %+v, %v", got, err)
	}
	var raw AIConfig
	_ = store.Get("settings", "ai_config", &raw)
	if !storage.IsEncryptedString(raw.APIKey) || !storage.IsEncryptedString(raw.Headers["H"]) {
		t.Fatalf("读取之后应顺手迁移成密文: %+v", raw)
	}
}

func TestAIConfigNeedsEncryption(t *testing.T) {
	_, store := newTestSettings(t)
	enc, err := store.EncryptString("k")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		key  string
		hdr  map[string]string
		want bool
	}{
		{"", nil, false},
		{"plain", nil, true},
		{enc, nil, false},
		{enc, map[string]string{"A": "plain"}, true},
		{enc, map[string]string{"A": enc, "B": ""}, false},
	}
	for i, c := range cases {
		if got := aiConfigNeedsEncryption(c.key, c.hdr); got != c.want {
			t.Errorf("case %d: got %v, want %v", i, got, c.want)
		}
	}
}

func TestGetAppInfoHasName(t *testing.T) {
	svc, _ := newTestSettings(t)
	if info := svc.GetAppInfo(); info.Name == "" {
		t.Fatalf("应用名不应为空: %+v", info)
	}
}
