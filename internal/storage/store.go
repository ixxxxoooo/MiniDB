package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketConnections = []byte("connections")
	bucketDocs        = []byte("docs")
	bucketHistory     = []byte("history")
	bucketSettings    = []byte("settings")
)

// Store BoltDB 本地存储引擎
type Store struct {
	db *bolt.DB
}

// NewStore 创建存储实例，数据保存在 ~/.tableplus-ai/data.db
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".tableplus-ai")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "data.db")
	return newStoreWithPath(dbPath)
}

// newStoreWithPath 使用指定路径创建存储实例（可用于测试）
func newStoreWithPath(dbPath string) (*Store, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketConnections, bucketDocs, bucketHistory, bucketSettings} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化存储失败: %w", err)
	}

	return &Store{db: db}, nil
}

// Close 关闭存储
func (s *Store) Close() error {
	return s.db.Close()
}

// Put 存储键值对
func (s *Store) Put(bucket, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Put([]byte(key), data)
	})
}

// Get 获取值
func (s *Store) Get(bucket, key string, dest interface{}) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key not found: %s", key)
		}
		return json.Unmarshal(data, dest)
	})
}

// Delete 删除键
func (s *Store) Delete(bucket, key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Delete([]byte(key))
	})
}

// List 列出 bucket 中所有值
func (s *Store) List(bucket string, factory func() interface{}) ([]interface{}, error) {
	var results []interface{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			item := factory()
			if err := json.Unmarshal(v, item); err != nil {
				return err
			}
			results = append(results, item)
			return nil
		})
	})
	return results, err
}
