package keychain

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"minidb/internal/appdata"
)

type fake struct {
	data   map[string][]byte
	getErr error
	setErr error
}

func (f *fake) Get(k string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	v, ok := f.data[k]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}
func (f *fake) Set(k string, v []byte) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.data[k] = v
	return nil
}

func key32(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestAdoptsLegacyKeyIntoStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(appdata.LegacyRootDir(), 0o755)
	os.WriteFile(appdata.LegacySecretKeyPath(), key32(7), 0o600)
	f := &fake{data: map[string][]byte{}}
	got, err := Provider{Backend: f}.Key()
	if err != nil || !bytes.Equal(got, key32(7)) {
		t.Fatalf("key = %v, %v", got, err)
	}
	if !bytes.Equal(f.data[account], key32(7)) {
		t.Fatal("legacy key not stored in the credential store")
	}
	if _, err := os.Stat(appdata.LegacySecretKeyPath()); err != nil {
		t.Fatal("legacy key file must stay for the old app")
	}
}

func TestStoreWinsAndNewKeyFileIsRemoved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	appdata.EnsureRootDir()
	os.WriteFile(appdata.SecretKeyPath(), key32(1), 0o600)
	f := &fake{data: map[string][]byte{account: key32(9)}}
	got, _ := Provider{Backend: f}.Key()
	if !bytes.Equal(got, key32(9)) {
		t.Fatal("credential store key must win")
	}
	// Moving a leftover new-directory key file into an empty store removes it.
	f2 := &fake{data: map[string][]byte{}}
	if got, _ := (Provider{Backend: f2}).Key(); !bytes.Equal(got, key32(1)) {
		t.Fatal("existing file key must be adopted")
	}
	if _, err := os.Stat(appdata.SecretKeyPath()); !os.IsNotExist(err) {
		t.Fatal("plaintext key file should be removed once stored")
	}
}

func TestCreatesKeyWhenNone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{data: map[string][]byte{}}
	got, err := Provider{Backend: f}.Key()
	if err != nil || len(got) != 32 || !bytes.Equal(f.data[account], got) {
		t.Fatalf("key=%v err=%v stored=%v", got, err, f.data)
	}
}

func TestFallsBackToFileWhenUnsupportedOrFailing(t *testing.T) {
	for _, f := range []*fake{
		{data: map[string][]byte{}, getErr: ErrUnsupported},
		{data: map[string][]byte{}, setErr: errors.New("denied")},
	} {
		t.Setenv("HOME", t.TempDir())
		a, err := Provider{Backend: f}.Key()
		if err != nil || len(a) != 32 {
			t.Fatalf("fallback key = %v, %v", a, err)
		}
		if _, err := os.Stat(appdata.SecretKeyPath()); err != nil {
			t.Fatal("fallback must persist the key file")
		}
		b, _ := Provider{Backend: f}.Key()
		if !bytes.Equal(a, b) {
			t.Fatal("fallback key must be stable across calls")
		}
	}
}
