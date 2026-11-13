// Package keychain supplies the local encryption key from the system
// credential store (macOS Keychain), falling back to a key file where none
// exists. It is a storage.KeyProvider.
package keychain

import (
	"errors"
	"os"

	"github.com/egoist/mygo"

	"minidb/internal/appdata"
	"minidb/internal/storage"
)

const account = "local-secret-key"

// Backend is the part of a credential store the provider needs.
type Backend interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}

// Errors a Backend maps its failures to.
var (
	ErrNotFound    = errors.New("secret not found")
	ErrUnsupported = errors.New("no credential store")
)

// System is the credential store MyGo exposes.
type System struct{}

func (System) Get(key string) ([]byte, error) {
	v, err := mygo.Secrets.Get(key)
	switch {
	case errors.Is(err, mygo.ErrSecretNotFound):
		return nil, ErrNotFound
	case errors.Is(err, errors.ErrUnsupported):
		return nil, ErrUnsupported
	}
	return v, err
}

func (System) Set(key string, value []byte) error {
	err := mygo.Secrets.Set(key, value)
	if errors.Is(err, errors.ErrUnsupported) {
		return ErrUnsupported
	}
	return err
}

// Provider reads the key from Backend. When the store has none it adopts an
// existing key file (the new one, or the legacy app's: its ciphertext needs
// that same key) or makes a new key, and stores it. After the key is safely
// in the store, the plaintext key file of the new data directory is removed;
// the legacy app's file is never touched. Where the store is unsupported or
// fails, the file provider is used.
type Provider struct {
	Backend Backend
	File    storage.FileKeyProvider
	// PreferFile 让开发版优先使用磁盘上已有的密钥文件（新旧数据目录里的 secret.key），
	// 找不到才去钥匙串。开发版每次重编译的 ad-hoc 签名都不同，钥匙串会在每次启动时弹出
	// 授权框，并且 mygo.Secrets 是在主线程上调用的，授权框不点掉整个窗口都出不来，看起来就像程序崩了。
	PreferFile bool
}

func (p Provider) Key() ([]byte, error) {
	if p.PreferFile {
		if key, err := p.File.Existing(); err == nil && len(key) == 32 {
			return key, nil
		}
	}
	key, err := p.Backend.Get(account)
	if err == nil && len(key) == 32 {
		return key, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return p.File.Key() // unsupported or failing store
	}
	// Absent (or malformed): adopt or create, then store.
	existing, ferr := p.File.Existing()
	if ferr != nil {
		return nil, ferr
	}
	if existing == nil {
		if existing, err = storage.NewRandomKey(); err != nil {
			return nil, err
		}
	}
	if err := p.Backend.Set(account, existing); err != nil {
		return p.File.Key() // could not store it: keep working from the file
	}
	if err := os.Remove(appdata.SecretKeyPath()); err != nil && !os.IsNotExist(err) {
		return existing, nil // key is safe in the store; a leftover file is not fatal
	}
	return existing, nil
}
