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
}

func (p Provider) Key() ([]byte, error) {
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
