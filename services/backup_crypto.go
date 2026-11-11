package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	backupKDFIterations = 600_000
	backupEncryptedVer  = 2
)

// ErrBackupPassphraseRequired 备份文件已加密但没有提供口令。
var ErrBackupPassphraseRequired = errors.New("备份文件已加密，请输入备份口令后重试")

// ErrBackupPassphraseWrong 口令错误或文件已损坏（AES-GCM 无法区分两者）。
var ErrBackupPassphraseWrong = errors.New("备份口令错误，或备份文件已损坏")

// encryptedBackupEnvelope 是加密备份的外层格式。内容是 ConnectionsBackup 的 JSON，
// 用口令经 PBKDF2-SHA256 派生出的密钥做 AES-256-GCM 加密。
type encryptedBackupEnvelope struct {
	Version    int    `json:"version"`
	Encrypted  bool   `json:"encrypted"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Data       string `json:"data"`
}

func deriveBackupKey(passphrase string, salt []byte, iterations int) ([]byte, error) {
	return pbkdf2.Key(sha256.New, passphrase, salt, iterations, 32)
}

func encryptBackup(plain []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := deriveBackupKey(passphrase, salt, backupKDFIterations)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	env := encryptedBackupEnvelope{
		Version:    backupEncryptedVer,
		Encrypted:  true,
		KDF:        "pbkdf2-sha256",
		Iterations: backupKDFIterations,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Data:       base64.StdEncoding.EncodeToString(gcm.Seal(nil, nonce, plain, nil)),
	}
	return json.MarshalIndent(env, "", "  ")
}

// isEncryptedBackup 判断文件内容是否为加密备份。
func isEncryptedBackup(data []byte) bool {
	var probe struct {
		Encrypted bool `json:"encrypted"`
	}
	return json.Unmarshal(data, &probe) == nil && probe.Encrypted
}

func decryptBackup(data []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, ErrBackupPassphraseRequired
	}
	var env encryptedBackupEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("解析备份文件失败: %w", err)
	}
	if env.KDF != "pbkdf2-sha256" || env.Iterations < 100_000 {
		return nil, fmt.Errorf("不支持的备份加密参数")
	}
	salt, err1 := base64.StdEncoding.DecodeString(env.Salt)
	nonce, err2 := base64.StdEncoding.DecodeString(env.Nonce)
	sealed, err3 := base64.StdEncoding.DecodeString(env.Data)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, fmt.Errorf("备份文件格式损坏")
	}
	key, err := deriveBackupKey(passphrase, salt, env.Iterations)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("备份文件格式损坏")
	}
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrBackupPassphraseWrong
	}
	return plain, nil
}
