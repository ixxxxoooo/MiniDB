package storage

import (
	"bytes"
	"strings"
	"testing"
)

type fixedKeyProvider []byte

func (k fixedKeyProvider) Key() ([]byte, error) { return k, nil }

// newKey 返回用固定密钥的加解密器；每个测试各用各的，互不影响。
func newKey(b byte) *Secrets {
	return NewSecrets(fixedKeyProvider(bytes.Repeat([]byte{b}, 32)))
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	k := newKey(1)
	enc, err := k.EncryptString("p@ss/w0rd 密码")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncryptedString(enc) || strings.Contains(enc, "p@ss") {
		t.Fatalf("密文应带前缀且不含明文: %q", enc)
	}
	dec, err := k.DecryptString(enc)
	if err != nil || dec != "p@ss/w0rd 密码" {
		t.Fatalf("解密 = %q, %v", dec, err)
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	k := newKey(1)
	a, _ := k.EncryptString("same")
	b, _ := k.EncryptString("same")
	if a == b {
		t.Fatal("同一明文两次加密应得到不同密文（随机 nonce）")
	}
}

func TestEncryptLeavesEmptyAndEncryptedAlone(t *testing.T) {
	k := newKey(1)
	if got, err := k.EncryptString(""); got != "" || err != nil {
		t.Fatalf("空串应原样返回: %q %v", got, err)
	}
	enc, _ := k.EncryptString("x")
	if again, _ := k.EncryptString(enc); again != enc {
		t.Fatal("已加密的值不应被再加密一次")
	}
}

func TestDecryptPlaintextPassesThrough(t *testing.T) {
	k := newKey(1)
	if got, err := k.DecryptString("legacy-plain"); got != "legacy-plain" || err != nil {
		t.Fatalf("旧版明文应原样返回: %q %v", got, err)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	k := newKey(1)
	enc, _ := k.EncryptString("secret")
	other := newKey(2)
	if _, err := other.DecryptString(enc); err == nil {
		t.Fatal("换了密钥应解不开")
	}
}

func TestDecryptRejectsTamperedAndMalformed(t *testing.T) {
	k := newKey(1)
	enc, _ := k.EncryptString("secret")
	// 改动密文最后一个字符（保持 base64 合法）：GCM 校验应失败
	last := enc[len(enc)-5]
	repl := byte('A')
	if last == 'A' {
		repl = 'B'
	}
	tampered := enc[:len(enc)-5] + string(repl) + enc[len(enc)-4:]
	if _, err := k.DecryptString(tampered); err == nil {
		t.Fatal("被篡改的密文应解密失败")
	}
	if _, err := k.DecryptString(encryptedValuePrefix + "!!!不是base64"); err == nil {
		t.Fatal("非法 base64 应报错")
	}
	if _, err := k.DecryptString(encryptedValuePrefix + "QQ=="); err == nil {
		t.Fatal("过短的密文应报错")
	}
}

func TestKeyOfWrongLengthIsRejected(t *testing.T) {
	k := NewSecrets(fixedKeyProvider([]byte("short")))
	if _, err := k.EncryptString("x"); err == nil {
		t.Fatal("长度不对的密钥应被拒绝")
	}
}
