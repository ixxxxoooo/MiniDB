package database

import (
	"database/sql"
	"fmt"
)

// TableOverview 是「项目概览」页里一张表（或视图）的一行：名称、类型、引擎、行数、大小、排序规则、备注、时间。
// 数字取不到时是 -1，文字取不到时是空串，界面上都显示成「—」。
//
// @author ixxxxoooo
type TableOverview struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // table / view
	Engine    string `json:"engine"`
	Rows      int64  `json:"rows"`      // MySQL、PostgreSQL 是估算值，SQLite 是精确值
	DataSize  int64  `json:"dataSize"`  // 字节
	IndexSize int64  `json:"indexSize"` // 字节
	Collation string `json:"collation"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// GetTableOverview 读出当前库里所有表和视图的概览信息；各数据库能给的字段不同，给不出的留空。
//
// 参数：db 连接；dbType 库类型；dbName 库名。
// 返回值：按名称排好的概览；出错时返回错误。
func GetTableOverview(db Executor, dbType, dbName string) ([]TableOverview, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLOverview(db, dbName)
	case "postgres":
		return getPostgresOverview(db)
	case "sqlite":
		return getSQLiteOverview(db)
	case "starrocks":
		// StarRocks 不支持预编译语句，且元信息少：用已有的表列表凑
		tables, err := GetTables(db, dbType, dbName)
		if err != nil {
			return nil, err
		}
		out := make([]TableOverview, 0, len(tables))
		for _, t := range tables {
			out = append(out, TableOverview{Name: t.Name, Type: t.Type, Rows: t.RowCount, DataSize: t.Size, IndexSize: -1, Comment: t.Comment})
		}
		return out, nil
	}
	return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
}

func getMySQLOverview(db Executor, dbName string) ([]TableOverview, error) {
	rows, err := db.Query(`SELECT table_name, table_type, engine, table_rows, data_length, index_length,
		table_collation, table_comment, create_time, update_time
		FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name`, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableOverview
	for rows.Next() {
		var t TableOverview
		var typ string
		var engine, coll, comment, created, updated sql.NullString
		var nrows, data, index sql.NullInt64
		if err := rows.Scan(&t.Name, &typ, &engine, &nrows, &data, &index, &coll, &comment, &created, &updated); err != nil {
			return nil, err
		}
		t.Type = "table"
		if typ == "VIEW" {
			t.Type = "view"
		}
		t.Engine, t.Collation, t.Comment, t.CreatedAt, t.UpdatedAt = engine.String, coll.String, comment.String, created.String, updated.String
		t.Rows, t.DataSize, t.IndexSize = nullOr(nrows, -1), nullOr(data, -1), nullOr(index, -1)
		if t.Type == "view" {
			t.Rows, t.DataSize, t.IndexSize = -1, -1, -1
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func getPostgresOverview(db Executor) ([]TableOverview, error) {
	rows, err := db.Query(`SELECT c.relname,
		CASE WHEN c.relkind IN ('v','m') THEN 'view' ELSE 'table' END,
		CASE WHEN c.relkind IN ('r','p','m') THEN GREATEST(c.reltuples, 0)::bigint ELSE -1 END,
		CASE WHEN c.relkind IN ('r','p','m') THEN pg_table_size(c.oid) ELSE -1 END,
		CASE WHEN c.relkind IN ('r','p','m') THEN pg_indexes_size(c.oid) ELSE -1 END,
		COALESCE(obj_description(c.oid, 'pg_class'), '')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m') ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableOverview
	for rows.Next() {
		var t TableOverview
		if err := rows.Scan(&t.Name, &t.Type, &t.Rows, &t.DataSize, &t.IndexSize, &t.Comment); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func getSQLiteOverview(db Executor) ([]TableOverview, error) {
	rows, err := db.Query(`SELECT name, type FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var out []TableOverview
	for rows.Next() {
		t := TableOverview{Rows: -1, DataSize: -1, IndexSize: -1}
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	rows.Close()
	for i := range out {
		if out[i].Type != "table" {
			continue
		}
		var n int64
		if db.QueryRow("SELECT COUNT(*) FROM "+QuoteIdent("sqlite", out[i].Name)).Scan(&n) == nil {
			out[i].Rows = n
		}
		// dbstat 不是每个构建都有；取不到就不显示大小
		var size sql.NullInt64
		if db.QueryRow("SELECT SUM(pgsize) FROM dbstat WHERE name = ?", out[i].Name).Scan(&size) == nil && size.Valid {
			out[i].DataSize = size.Int64
		}
	}
	return out, nil
}

func nullOr(n sql.NullInt64, def int64) int64 {
	if n.Valid {
		return n.Int64
	}
	return def
}
