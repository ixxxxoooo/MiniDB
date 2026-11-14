package storage

import (
	"bytes"
	"strings"
	"testing"
)

type fixedKeyProvider []byte

func (k fixedKeyProvider) Key() ([]byte, error) { return k, nil }

// useKey 换上固定密钥，测试结束后换回文件密钥。
func useKey(t *testing.T, b byte) {
	t.Helper()
	SetKeyProvider(fixedKeyProvider(bytes.Repeat([]byte{b}, 32)))
	t.Cleanup(func() { SetKeyProvider(FileKeyProvider{}) })
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	useKey(t, 1)
	enc, err := EncryptString("p@ss/w0rd 密码")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncryptedString(enc) || strings.Contains(enc, "p@ss") {
		t.Fatalf("密文应带前缀且不含明文: %q", enc)
	}
	dec, err := DecryptString(enc)
	if err != nil || dec != "p@ss/w0rd 密码" {
		t.Fatalf("解密 = %q, %v", dec, err)
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	useKey(t, 1)
	a, _ := EncryptString("same")
	b, _ := EncryptString("same")
	if a == b {
		t.Fatal("同一明文两次加密应得到不同密文（随机 nonce）")
	}
}

func TestEncryptLeavesEmptyAndEncryptedAlone(t *testing.T) {
	useKey(t, 1)
	if got, err := EncryptString(""); got != "" || err != nil {
		t.Fatalf("空串应原样返回: %q %v", got, err)
	}
	enc, _ := EncryptString("x")
	if again, _ := EncryptString(enc); again != enc {
		t.Fatal("已加密的值不应被再加密一次")
	}
}

func TestDecryptPlaintextPassesThrough(t *testing.T) {
	useKey(t, 1)
	if got, err := DecryptString("legacy-plain"); got != "legacy-plain" || err != nil {
		t.Fatalf("旧版明文应原样返回: %q %v", got, err)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	useKey(t, 1)
	enc, _ := EncryptString("secret")
	SetKeyProvider(fixedKeyProvider(bytes.Repeat([]byte{2}, 32)))
	if _, err := DecryptString(enc); err == nil {
		t.Fatal("换了密钥应解不开")
	}
}

func TestDecryptRejectsTamperedAndMalformed(t *testing.T) {
	useKey(t, 1)
	enc, _ := EncryptString("secret")
	// 改动密文最后一个字符（保持 base64 合法）：GCM 校验应失败
	last := enc[len(enc)-5]
	repl := byte('A')
	if last == 'A' {
		repl = 'B'
	}
	tampered := enc[:len(enc)-5] + string(repl) + enc[len(enc)-4:]
	if _, err := DecryptString(tampered); err == nil {
		t.Fatal("被篡改的密文应解密失败")
	}
	if _, err := DecryptString(encryptedValuePrefix + "!!!不是base64"); err == nil {
		t.Fatal("非法 base64 应报错")
	}
	if _, err := DecryptString(encryptedValuePrefix + "QQ=="); err == nil {
		t.Fatal("过短的密文应报错")
	}
}

func TestKeyOfWrongLengthIsRejected(t *testing.T) {
	SetKeyProvider(fixedKeyProvider([]byte("short")))
	t.Cleanup(func() { SetKeyProvider(FileKeyProvider{}) })
	if _, err := EncryptString("x"); err == nil {
		t.Fatal("长度不对的密钥应被拒绝")
	}
}
