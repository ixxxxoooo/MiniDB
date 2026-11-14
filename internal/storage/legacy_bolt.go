package storage

import (
	"errors"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	metaBucket       = "meta"
	metaBoltImported = "bolt_imported"
)

// Deprecated: 仅为升级 Wails 时代用户的旧数据保留（把 BoltDB 一次性导入 SQLite）。
// 等旧版本不再有人使用后整体删除本文件，不要在这里添加新功能，也不要让别处依赖它。
//
// importLegacyBolt copies every bucket of the Wails-era BoltDB file into the
// SQLite store, once. The legacy file is opened read-only and never modified.
// A missing file is not an error.
func (s *Store) importLegacyBolt(path string) error {
	var done bool
	if err := s.Get(metaBucket, metaBoltImported, &done); err == nil && done {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	src, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: 2 * time.Second})
	if err != nil {
		return err
	}
	defer src.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = src.View(func(btx *bolt.Tx) error {
		return btx.ForEach(func(name []byte, b *bolt.Bucket) error {
			return b.ForEach(func(k, v []byte) error {
				if v == nil { // nested bucket; the old app had none
					return nil
				}
				_, err := tx.Exec(`INSERT INTO kv(bucket,k,v) VALUES(?,?,?)
					ON CONFLICT(bucket,k) DO NOTHING`, string(name), string(k), append([]byte(nil), v...))
				return err
			})
		})
	})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO kv(bucket,k,v) VALUES(?,?,?)
		ON CONFLICT(bucket,k) DO UPDATE SET v=excluded.v`, metaBucket, metaBoltImported, []byte("true")); err != nil {
		return err
	}
	return tx.Commit()
}
