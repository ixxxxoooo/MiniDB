package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func writeLegacy(t *testing.T, path string) {
	t.Helper()
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		for name, kv := range map[string]map[string]string{
			"connections": {"c1": `{"id":"c1","name":"prod","password":"enc:v1:abc"}`},
			"ai_sessions": {"meta:s1": `{"id":"s1","title":"hi"}`, "msg:s1:m1": `{"id":"m1","role":"user","content":"yo"}`},
			"settings":    {"ai": `{"model":"gpt"}`},
		} {
			b, err := tx.CreateBucketIfNotExists([]byte(name))
			if err != nil {
				return err
			}
			for k, v := range kv {
				if err := b.Put([]byte(k), []byte(v)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestImportLegacyBolt(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "data.db")
	writeLegacy(t, legacy)
	before, _ := os.ReadFile(legacy)

	s, err := OpenStore(filepath.Join(dir, "new.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.importLegacyBolt(legacy); err != nil {
		t.Fatal(err)
	}

	var conn struct{ Name, Password string }
	if err := s.Get("connections", "c1", &conn); err != nil || conn.Name != "prod" || conn.Password != "enc:v1:abc" {
		t.Fatalf("connection = %+v, %v", conn, err)
	}
	if sess, err := s.GetAISession("s1"); err != nil || sess.Title != "hi" {
		t.Fatalf("session = %+v, %v", sess, err)
	}
	if msgs, _ := s.GetAISessionMessages("s1", 0); len(msgs) != 1 || msgs[0].Content != "yo" {
		t.Fatalf("messages = %+v", msgs)
	}

	// Not repeated: later edits survive, and a deleted row does not come back.
	s.Delete("connections", "c1")
	if err := s.importLegacyBolt(legacy); err != nil {
		t.Fatal(err)
	}
	if err := s.Get("connections", "c1", &conn); err == nil {
		t.Fatal("import must run once")
	}
	after, _ := os.ReadFile(legacy)
	if !bytes.Equal(before, after) {
		t.Fatal("legacy file must not be modified")
	}
}

func TestImportLegacyBoltMissingFile(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "new.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.importLegacyBolt(filepath.Join(t.TempDir(), "nope.db")); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyCiphertextDecryptsWithAdoptedKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(legacyKey()), 0o755)
	os.WriteFile(legacyKey(), bytes.Repeat([]byte{5}, 32), 0o600)
	enc, err := NewSecrets(FileKeyProvider{}).EncryptString("hunter2")
	if err != nil || !IsEncryptedString(enc) {
		t.Fatal(enc, err)
	}
	// new process: key re-read, not generated anew
	if got, err := NewSecrets(FileKeyProvider{}).DecryptString(enc); err != nil || got != "hunter2" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
}

func legacyKey() string { return filepath.Join(os.Getenv("HOME"), ".minidb", "secret.key") }
