package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MySQL 系的索引、统计、概览、外键、触发器与例程读取。

func (mysqlDialect) Indexes(ctx context.Context, db Executor, dbName, table string) ([]IndexInfo, error) {
	// 按索引名分组，因为一个索引可能包含多列
	indexMap := make(map[string]*IndexInfo)
	var order []string
	err := queryEach(ctx, db, `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE, SEQ_IN_INDEX, NULLABLE, INDEX_COMMENT
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`, []any{dbName, table}, func(rows rowScanner) error {
		var idxName, colName, idxType, nullable, comment string
		var nonUnique, seqInIndex int
		if err := rows.Scan(&idxName, &colName, &nonUnique, &idxType, &seqInIndex, &nullable, &comment); err != nil {
			return err
		}
		if _, exists := indexMap[idxName]; !exists {
			indexMap[idxName] = &IndexInfo{
				Name: idxName, Columns: []string{}, IsUnique: nonUnique == 0,
				IsPrimary: idxName == "PRIMARY", Type: idxType, Comment: comment,
			}
			order = append(order, idxName)
		}
		indexMap[idxName].Columns = append(indexMap[idxName].Columns, colName)
		return nil
	})
	if err != nil {
		return nil, err
	}
	var indexes []IndexInfo
	for _, name := range order {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}

func (mysqlDialect) TableStats(ctx context.Context, db Executor, dbName, table string) (*TableStats, error) {
	var stats TableStats
	err := db.QueryRowContext(ctx, `SELECT IFNULL(table_rows,0), IFNULL(data_length,0), IFNULL(index_length,0),
		IFNULL(data_length+index_length,0),
		IFNULL(create_time,''), IFNULL(update_time,''),
		IFNULL(engine,''), IFNULL(table_collation,'')
		FROM information_schema.tables
		WHERE table_schema = ? AND table_name = ?`, dbName, table).Scan(
		&stats.RowCount, &stats.DataSize, &stats.IndexSize, &stats.TotalSize,
		&stats.CreateTime, &stats.UpdateTime, &stats.Engine, &stats.Collation,
	)
	return &stats, err
}

func (mysqlDialect) TableOverview(ctx context.Context, db Executor, dbName string) ([]TableOverview, error) {
	var out []TableOverview
	err := queryEach(ctx, db, `SELECT table_name, table_type, engine, table_rows, data_length, index_length,
		table_collation, table_comment, create_time, update_time
		FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name`, []any{dbName}, func(rows rowScanner) error {
		var t TableOverview
		var typ string
		var engine, coll, comment, created, updated sql.NullString
		var nrows, data, index sql.NullInt64
		if err := rows.Scan(&t.Name, &typ, &engine, &nrows, &data, &index, &coll, &comment, &created, &updated); err != nil {
			return err
		}
		t.Type = tableTypeOf(typ)
		t.Engine, t.Collation, t.Comment, t.CreatedAt, t.UpdatedAt = engine.String, coll.String, comment.String, created.String, updated.String
		t.Rows, t.DataSize, t.IndexSize = nullOr(nrows, -1), nullOr(data, -1), nullOr(index, -1)
		if t.Type == "view" {
			t.Rows, t.DataSize, t.IndexSize = -1, -1, -1
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

func (mysqlDialect) ForeignKeys(ctx context.Context, db Executor, dbName, table string) ([]ForeignKeyInfo, error) {
	byName := map[string]*ForeignKeyInfo{}
	var order []string
	err := queryEach(ctx, db, `SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
			r.UPDATE_RULE, r.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME
		WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, []any{dbName, table}, func(rows rowScanner) error {
		var name, col, refTable, refCol, upd, del string
		if err := rows.Scan(&name, &col, &refTable, &refCol, &upd, &del); err != nil {
			return err
		}
		fk, ok := byName[name]
		if !ok {
			fk = &ForeignKeyInfo{Name: name, RefTable: refTable, OnUpdate: upd, OnDelete: del}
			byName[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, col)
		fk.RefColumns = append(fk.RefColumns, refCol)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]ForeignKeyInfo, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out, nil
}

func (d mysqlDialect) Triggers(ctx context.Context, db Executor, dbName, table string) ([]TriggerInfo, error) {
	var out []TriggerInfo
	err := queryEach(ctx, db, `SELECT TRIGGER_NAME, ACTION_TIMING, EVENT_MANIPULATION, ACTION_STATEMENT
		FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA = ? AND EVENT_OBJECT_TABLE = ? ORDER BY TRIGGER_NAME`,
		[]any{dbName, table}, func(rows rowScanner) error {
			var t TriggerInfo
			var body string
			if err := rows.Scan(&t.Name, &t.Timing, &t.Event, &body); err != nil {
				return err
			}
			t.Definition = fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW %s",
				d.QuoteIdent(t.Name), t.Timing, t.Event, d.QuoteIdent(table), body)
			out = append(out, t)
			return nil
		})
	return out, err
}

func (mysqlDialect) Routines(ctx context.Context, db Executor, dbName string) ([]RoutineInfo, error) {
	var out []RoutineInfo
	err := queryEach(ctx, db, `SELECT ROUTINE_NAME, ROUTINE_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_NAME`,
		[]any{dbName}, func(rows rowScanner) error {
			var r RoutineInfo
			if err := rows.Scan(&r.Name, &r.Kind); err != nil {
				return err
			}
			r.Kind = strings.ToLower(r.Kind)
			out = append(out, r)
			return nil
		})
	return out, err
}

func (d mysqlDialect) RoutineDDL(ctx context.Context, db Executor, dbName, kind, name string) (string, error) {
	if kind != "function" && kind != "procedure" {
		return "", fmt.Errorf("未知的例程类型: %s", kind)
	}
	return showCreate(ctx, db, fmt.Sprintf("SHOW CREATE %s %s.%s", strings.ToUpper(kind), d.QuoteIdent(dbName), d.QuoteIdent(name)), 2)
}
