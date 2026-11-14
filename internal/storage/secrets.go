package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"minidb/internal/appdata"
)

const encryptedValuePrefix = "enc:v1:"

func IsEncryptedString(value string) bool {
	return strings.HasPrefix(value, encryptedValuePrefix)
}

// Secrets 用一个 KeyProvider 提供的密钥加解密本地敏感字段。
// 每个 Store 持有自己的 Secrets，不再有进程级的全局密钥。
type Secrets struct {
	mu       sync.Mutex
	provider KeyProvider
	cached   []byte
}

// NewSecrets 创建加解密器；provider 为 nil 时用 FileKeyProvider。
func NewSecrets(provider KeyProvider) *Secrets {
	if provider == nil {
		provider = FileKeyProvider{}
	}
	return &Secrets{provider: provider}
}

// EncryptString encrypts a local secret before it is persisted.
// Values without this prefix are treated as legacy plaintext on read.
func (k *Secrets) EncryptString(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}
	gcm, err := k.cipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成加密随机数失败: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return encryptedValuePrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptString decrypts an encrypted local secret, leaving legacy plaintext unchanged.
func (k *Secrets) DecryptString(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedValuePrefix))
	if err != nil {
		return "", fmt.Errorf("解码加密数据失败: %w", err)
	}
	gcm, err := k.cipher()
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("加密数据格式错误")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密本地密钥失败: %w", err)
	}
	return string(plain), nil
}

func (k *Secrets) cipher() (cipher.AEAD, error) {
	key, err := k.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %w", err)
	}
	return cipher.NewGCM(block)
}

// KeyProvider supplies the 32-byte key that encrypts persisted secrets.
type KeyProvider interface {
	Key() ([]byte, error)
}

func (k *Secrets) key() ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.cached != nil {
		return k.cached, nil
	}
	key, err := k.provider.Key()
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("本地加密密钥长度无效")
	}
	k.cached = key
	return key, nil
}

// FileKeyProvider keeps the key in a file of the data directory. It is the
// fallback where no credential store exists. If the file is missing it adopts
// the legacy app's key, which the legacy ciphertext needs, or creates one.
type FileKeyProvider struct{}

// Existing returns the key from the new or the legacy key file, or nil.
func (FileKeyProvider) Existing() ([]byte, error) {
	for _, path := range []string{appdata.SecretKeyPath(), appdata.LegacySecretKeyPath()} {
		key, err := os.ReadFile(path)
		if err == nil {
			if len(key) != 32 {
				return nil, fmt.Errorf("本地加密密钥长度无效: %s", path)
			}
			return key, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取本地加密密钥失败: %w", err)
		}
	}
	return nil, nil
}

func (p FileKeyProvider) Key() ([]byte, error) {
	if err := appdata.EnsureRootDir(); err != nil {
		return nil, err
	}
	key, err := p.Existing()
	if err != nil || key != nil {
		return key, err
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成本地加密密钥失败: %w", err)
	}
	if err := os.WriteFile(appdata.SecretKeyPath(), key, 0o600); err != nil {
		return nil, fmt.Errorf("写入本地加密密钥失败: %w", err)
	}
	return key, nil
}

// NewRandomKey returns a fresh 32-byte key.
func NewRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}
