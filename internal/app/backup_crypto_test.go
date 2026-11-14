package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestBackupEncryptDecryptRoundTrip(t *testing.T) {
	plain := []byte(`{"connections":[{"name":"生产库"}]}`)
	enc, err := encryptBackup(plain, "口令-123")
	if err != nil {
		t.Fatal(err)
	}
	if !isEncryptedBackup(enc) || bytes.Contains(enc, []byte("生产库")) {
		t.Fatal("加密备份应标记为已加密且不含明文")
	}
	got, err := decryptBackup(enc, "口令-123")
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("解密 = %q, %v", got, err)
	}
}

func TestBackupWrongOrMissingPassphrase(t *testing.T) {
	enc, err := encryptBackup([]byte("data"), "right")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptBackup(enc, "wrong"); !errors.Is(err, ErrBackupPassphraseWrong) {
		t.Fatalf("错误口令应得到 ErrBackupPassphraseWrong，实际 %v", err)
	}
	if _, err := decryptBackup(enc, ""); !errors.Is(err, ErrBackupPassphraseRequired) {
		t.Fatalf("缺口令应得到 ErrBackupPassphraseRequired，实际 %v", err)
	}
}

func TestBackupRejectsTamperedData(t *testing.T) {
	enc, _ := encryptBackup([]byte("data"), "pw")
	var env map[string]any
	if err := json.Unmarshal(enc, &env); err != nil {
		t.Fatal(err)
	}
	env["data"] = "AAAA" + env["data"].(string)[4:]
	bad, _ := json.Marshal(env)
	if _, err := decryptBackup(bad, "pw"); err == nil {
		t.Fatal("密文被改动应解密失败")
	}
}

func TestBackupRejectsWeakKDF(t *testing.T) {
	enc, _ := encryptBackup([]byte("data"), "pw")
	var env map[string]any
	_ = json.Unmarshal(enc, &env)
	env["iterations"] = 10 // 迭代次数被降到很低：拒绝，避免被降级攻击
	bad, _ := json.Marshal(env)
	if _, err := decryptBackup(bad, "pw"); err == nil {
		t.Fatal("过低的迭代次数应被拒绝")
	}
}

func TestIsEncryptedBackupOnPlainJSON(t *testing.T) {
	if isEncryptedBackup([]byte(`{"connections":[]}`)) || isEncryptedBackup([]byte("不是 JSON")) {
		t.Fatal("明文备份不应被识别为加密")
	}
}
