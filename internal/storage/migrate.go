package storage

import (
	"database/sql"
	"fmt"
)

const metaSchemaVersion = "schema_version"

// migrations 是按顺序执行的存储迁移，下标 i 把版本 i 升到 i+1。
// 只许在末尾追加，已发布的迁移不能改、不能删、不能调序。
// 每个迁移在同一个事务里执行，失败整体回滚，版本号不前进。
var migrations = []func(tx *sql.Tx) error{
	// v0 → v1：基线。此前的版本没有版本号，kv 表的结构就是现在的结构，无需改动。
	func(tx *sql.Tx) error { return nil },
}

// currentSchemaVersion 是当前程序认识的最新存储版本。
func currentSchemaVersion() int { return len(migrations) }

// migrate 把存储升级到最新版本。库的版本比程序新（用户降级了应用）时拒绝打开，
// 否则旧程序可能按旧结构写坏新数据。
func migrate(db *sql.DB) error {
	var raw []byte
	err := db.QueryRow(`SELECT v FROM kv WHERE bucket=? AND k=?`, metaBucket, metaSchemaVersion).Scan(&raw)
	version := 0
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return fmt.Errorf("读取存储版本失败: %w", err)
	default:
		if _, scanErr := fmt.Sscanf(string(raw), "%d", &version); scanErr != nil {
			return fmt.Errorf("存储版本号损坏: %q", raw)
		}
	}
	if version > currentSchemaVersion() {
		return fmt.Errorf("本地数据由更新版本的 MiniDB 创建（存储版本 %d，当前程序支持到 %d），请升级应用", version, currentSchemaVersion())
	}
	for ; version < currentSchemaVersion(); version++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := migrations[version](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("存储迁移 v%d→v%d 失败: %w", version, version+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO kv(bucket,k,v) VALUES(?,?,?)
			ON CONFLICT(bucket,k) DO UPDATE SET v=excluded.v`, metaBucket, metaSchemaVersion, []byte(fmt.Sprint(version+1))); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
