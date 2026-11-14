package database

import (
	"context"
	"strings"
)

// TableForeignKey 是某张表上的一个外键。
type TableForeignKey struct {
	Table string
	ForeignKeyInfo
}

// BulkForeignKeys 由能一次取回整库外键的方言实现（MySQL 系用 information_schema，PostgreSQL 用 pg_constraint），
// 避免对每张表各发一次查询。没有实现的方言由 AllForeignKeys 逐表退回。
type BulkForeignKeys interface {
	AllForeignKeys(ctx context.Context, db Executor, dbName string) ([]TableForeignKey, error)
}

// AllForeignKeys 返回库里所有表的外键：方言支持批量就一次取回，否则对 tables 逐表取。
func AllForeignKeys(ctx context.Context, db Executor, dbType DBType, dbName string, tables []string) ([]TableForeignKey, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	if b, ok := d.(BulkForeignKeys); ok {
		return b.AllForeignKeys(ctx, db, dbName)
	}
	r, ok := d.(ForeignKeyReader)
	if !ok || !d.Caps().ForeignKeys {
		return nil, nil
	}
	var out []TableForeignKey
	for _, t := range tables {
		fks, err := r.ForeignKeys(ctx, db, dbName, t)
		if err != nil {
			return nil, err
		}
		for _, fk := range fks {
			out = append(out, TableForeignKey{Table: t, ForeignKeyInfo: fk})
		}
	}
	return out, nil
}

// fkGrouper 把按「表、约束名、列顺序」排好的行合并成一个个外键（联合外键有多行）。
type fkGrouper struct {
	index map[string]int
	out   []TableForeignKey
}

func (g *fkGrouper) add(table, name, col, refTable, refCol, upd, del string) {
	if g.index == nil {
		g.index = map[string]int{}
	}
	key := table + "\x00" + name
	i, ok := g.index[key]
	if !ok {
		i = len(g.out)
		g.index[key] = i
		g.out = append(g.out, TableForeignKey{Table: table, ForeignKeyInfo: ForeignKeyInfo{Name: name, RefTable: refTable, OnUpdate: upd, OnDelete: del}})
	}
	g.out[i].Columns = append(g.out[i].Columns, col)
	g.out[i].RefColumns = append(g.out[i].RefColumns, refCol)
}

// AllForeignKeys 用一次 information_schema 查询取回整库外键。
func (mysqlDialect) AllForeignKeys(ctx context.Context, db Executor, dbName string) ([]TableForeignKey, error) {
	var g fkGrouper
	err := queryEach(ctx, db, `SELECT k.TABLE_NAME, k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
			r.UPDATE_RULE, r.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME
		WHERE k.TABLE_SCHEMA = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, []any{dbName}, func(rows rowScanner) error {
		var table, name, col, refTable, refCol, upd, del string
		if err := rows.Scan(&table, &name, &col, &refTable, &refCol, &upd, &del); err != nil {
			return err
		}
		g.add(table, name, col, refTable, refCol, upd, del)
		return nil
	})
	return g.out, err
}

// AllForeignKeys 用一次 pg_constraint 查询取回 public 模式里所有表的外键；引用别的模式的外键不返回。
func (postgresDialect) AllForeignKeys(ctx context.Context, db Executor, _ string) ([]TableForeignKey, error) {
	var out []TableForeignKey
	err := queryEach(ctx, db, `SELECT cl.relname, con.conname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
			rcl.relname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum),
			con.confupdtype::text, con.confdeltype::text
		FROM pg_constraint con
		JOIN pg_class cl ON cl.oid = con.conrelid
		JOIN pg_namespace ns ON ns.oid = cl.relnamespace
		JOIN pg_class rcl ON rcl.oid = con.confrelid
		JOIN pg_namespace rns ON rns.oid = rcl.relnamespace
		WHERE con.contype = 'f' AND ns.nspname = 'public' AND rns.nspname = 'public'
		ORDER BY cl.relname, con.conname`, nil, func(rows rowScanner) error {
		var table, name, cols, refTable, refCols, upd, del string
		if err := rows.Scan(&table, &name, &cols, &refTable, &refCols, &upd, &del); err != nil {
			return err
		}
		out = append(out, TableForeignKey{Table: table, ForeignKeyInfo: ForeignKeyInfo{Name: name, Columns: strings.Split(cols, ","),
			RefTable: refTable, RefColumns: strings.Split(refCols, ","), OnUpdate: pgFKAction(upd), OnDelete: pgFKAction(del)}})
		return nil
	})
	return out, err
}
