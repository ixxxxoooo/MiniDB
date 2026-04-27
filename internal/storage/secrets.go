package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const encryptedValuePrefix = "enc:v1:"

func IsEncryptedString(value string) bool {
	return strings.HasPrefix(value, encryptedValuePrefix)
}

// EncryptString encrypts a local secret before it is persisted in BoltDB.
// Values without this prefix are treated as legacy plaintext on read.
func EncryptString(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}
	gcm, err := secretCipher()
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
func DecryptString(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedValuePrefix))
	if err != nil {
		return "", fmt.Errorf("解码加密数据失败: %w", err)
	}
	gcm, err := secretCipher()
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

func secretCipher() (cipher.AEAD, error) {
	key, err := loadOrCreateSecretKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %w", err)
	}
	return cipher.NewGCM(block)
}

func loadOrCreateSecretKey() ([]byte, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}
	dataDir := filepath.Join(homeDir, ".tableplus-ai")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	keyPath := filepath.Join(dataDir, "secret.key")
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("本地加密密钥长度无效")
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取本地加密密钥失败: %w", err)
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成本地加密密钥失败: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("写入本地加密密钥失败: %w", err)
	}
	return key, nil
}
