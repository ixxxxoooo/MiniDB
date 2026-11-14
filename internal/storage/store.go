package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	_ "modernc.org/sqlite"

	"minidb/internal/appdata"
)

// ErrKeyNotFound is returned by Get when the key does not exist.
var ErrKeyNotFound = errors.New("key not found")

// Store is the local key-value store backing connections, docs, settings,
// schema indexes, AI sessions and SQL history. Values are JSON documents in
// one SQLite table, addressed by (bucket, key); keys sort lexicographically,
// which prefix scans rely on.
type Store struct {
	db        *sql.DB
	importErr error
	secrets   *Secrets
}

const schema = `
CREATE TABLE IF NOT EXISTS kv (
	bucket TEXT NOT NULL,
	k      TEXT NOT NULL,
	v      BLOB NOT NULL,
	PRIMARY KEY (bucket, k)
) WITHOUT ROWID;`

// NewStore opens the application's store, creating it and importing data of
// the Wails-era BoltDB file (~/.minidb/data.db) on first run.
func NewStore(opts ...Option) (*Store, error) {
	if err := appdata.EnsureRootDir(); err != nil {
		return nil, err
	}
	s, err := OpenStore(appdata.DataFilePath(), opts...)
	if err != nil {
		return nil, err
	}
	if err := s.importLegacyBolt(appdata.LegacyDataFilePath()); err != nil {
		// A failed import must not block startup; the old data stays untouched
		// and the import is retried on the next launch.
		s.importErr = err
	}
	return s, nil
}

// Option 调整 Store 的可选行为。
type Option func(*Store)

// WithKeyProvider 指定加密敏感字段的密钥来源（比如系统钥匙串）；默认用数据目录里的密钥文件。
func WithKeyProvider(p KeyProvider) Option {
	return func(s *Store) { s.secrets = NewSecrets(p) }
}

// EncryptString 用这个存储的密钥加密一个敏感字段。
func (s *Store) EncryptString(value string) (string, error) { return s.secrets.EncryptString(value) }

// DecryptString 解密 EncryptString 的结果；没有加密前缀的旧明文原样返回。
func (s *Store) DecryptString(value string) (string, error) { return s.secrets.DecryptString(value) }

// OpenStore opens (creating if needed) the store at path.
func OpenStore(path string, opts ...Option) (*Store, error) {
	// 先以 0600 建好文件：SQLite 的 -wal / -shm 沿用主文件的权限，库里有历史 SQL、AI 会话等明文内容
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600); err == nil {
		f.Close()
	}
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化存储失败: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = appdata.TightenFile(path + suffix) // 旧版本留下的宽松权限也收紧
	}
	st := &Store{db: db, secrets: NewSecrets(nil)}
	for _, opt := range opts {
		opt(st)
	}
	return st, nil
}

// ImportError reports why importing legacy data failed on startup, if it did.
func (s *Store) ImportError() error { return s.importErr }

// Close closes the store.
func (s *Store) Close() error { return s.db.Close() }

// Put stores value as JSON.
func (s *Store) Put(bucket, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.putRaw(bucket, key, data)
}

func (s *Store) putRaw(bucket, key string, data []byte) error {
	_, err := s.db.Exec(`INSERT INTO kv(bucket,k,v) VALUES(?,?,?)
		ON CONFLICT(bucket,k) DO UPDATE SET v=excluded.v`, bucket, key, data)
	return err
}

// Get loads the value of key into dest.
func (s *Store) Get(bucket, key string, dest interface{}) error {
	var data []byte
	err := s.db.QueryRow(`SELECT v FROM kv WHERE bucket=? AND k=?`, bucket, key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrKeyNotFound, key)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes key; deleting a missing key is not an error.
func (s *Store) Delete(bucket, key string) error {
	_, err := s.db.Exec(`DELETE FROM kv WHERE bucket=? AND k=?`, bucket, key)
	return err
}

// List returns every value of bucket in key order.
func (s *Store) List(bucket string, factory func() interface{}) ([]interface{}, error) {
	rows, err := s.db.Query(`SELECT v FROM kv WHERE bucket=? ORDER BY k`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []interface{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		item := factory()
		if err := json.Unmarshal(data, item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// prefixEnd is the exclusive upper bound of keys starting with prefix.
func prefixEnd(prefix string) string { return prefix + "\U0010FFFF" }

// scanPrefix calls fn with the raw value of each key starting with prefix, in
// key order (descending when desc). fn returns false to stop.
func (s *Store) scanPrefix(bucket, prefix string, desc bool, fn func(key string, v []byte) bool) error {
	order := "ASC"
	if desc {
		order = "DESC"
	}
	rows, err := s.db.Query(`SELECT k, v FROM kv WHERE bucket=? AND k>=? AND k<? ORDER BY k `+order,
		bucket, prefix, prefixEnd(prefix))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		if !fn(k, v) {
			break
		}
	}
	return rows.Err()
}

// deletePrefix removes every key of bucket starting with prefix.
func (s *Store) deletePrefix(bucket, prefix string) error {
	_, err := s.db.Exec(`DELETE FROM kv WHERE bucket=? AND k>=? AND k<?`, bucket, prefix, prefixEnd(prefix))
	return err
}

// ListPrefix returns the values of bucket whose key starts with prefix, in key
// order (newest first when desc).
func (s *Store) ListPrefix(bucket, prefix string, desc bool, factory func() interface{}) ([]interface{}, error) {
	var out []interface{}
	var decodeErr error
	err := s.scanPrefix(bucket, prefix, desc, func(_ string, v []byte) bool {
		item := factory()
		if err := json.Unmarshal(v, item); err != nil {
			decodeErr = err
			return false
		}
		out = append(out, item)
		return true
	})
	if err != nil {
		return nil, err
	}
	return out, decodeErr
}
