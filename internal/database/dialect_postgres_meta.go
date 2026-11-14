package database

import (
	"context"
	"strings"
)

// PostgreSQL 的索引、外键、触发器与例程读取。

func (postgresDialect) Indexes(ctx context.Context, db Executor, _, table string) ([]IndexInfo, error) {
	var indexes []IndexInfo
	err := queryEach(ctx, db, `SELECT i.relname AS index_name,
		array_to_string(array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)), ',') AS columns,
		ix.indisunique, ix.indisprimary,
		am.amname AS index_type
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON am.oid = i.relam
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1 AND t.relkind = 'r'
		GROUP BY i.relname, ix.indisunique, ix.indisprimary, am.amname
		ORDER BY i.relname`, []any{table}, func(rows rowScanner) error {
		var idx IndexInfo
		var colStr string
		if err := rows.Scan(&idx.Name, &colStr, &idx.IsUnique, &idx.IsPrimary, &idx.Type); err != nil {
			return err
		}
		idx.Columns = strings.Split(colStr, ",")
		indexes = append(indexes, idx)
		return nil
	})
	return indexes, err
}

func (postgresDialect) ForeignKeys(ctx context.Context, db Executor, _, table string) ([]ForeignKeyInfo, error) {
	var out []ForeignKeyInfo
	err := queryEach(ctx, db, `SELECT con.conname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
			cl.relname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum),
			con.confupdtype::text, con.confdeltype::text
		FROM pg_constraint con JOIN pg_class cl ON cl.oid = con.confrelid
		WHERE con.contype = 'f' AND con.conrelid = quote_ident($1)::regclass
		ORDER BY con.conname`, []any{table}, func(rows rowScanner) error {
		var name, cols, refTable, refCols, upd, del string
		if err := rows.Scan(&name, &cols, &refTable, &refCols, &upd, &del); err != nil {
			return err
		}
		out = append(out, ForeignKeyInfo{Name: name, Columns: strings.Split(cols, ","), RefTable: refTable,
			RefColumns: strings.Split(refCols, ","), OnUpdate: pgFKAction(upd), OnDelete: pgFKAction(del)})
		return nil
	})
	return out, err
}

func pgFKAction(code string) string {
	switch code {
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	}
	return "NO ACTION"
}

func (postgresDialect) Triggers(ctx context.Context, db Executor, _, table string) ([]TriggerInfo, error) {
	var out []TriggerInfo
	err := queryEach(ctx, db, `SELECT t.tgname, pg_get_triggerdef(t.oid)
		FROM pg_trigger t WHERE t.tgrelid = quote_ident($1)::regclass AND NOT t.tgisinternal ORDER BY t.tgname`,
		[]any{table}, func(rows rowScanner) error {
			var t TriggerInfo
			if err := rows.Scan(&t.Name, &t.Definition); err != nil {
				return err
			}
			t.Timing, t.Event = parseTriggerHead(t.Definition)
			out = append(out, t)
			return nil
		})
	return out, err
}

func (postgresDialect) Routines(ctx context.Context, db Executor, _ string) ([]RoutineInfo, error) {
	var out []RoutineInfo
	err := queryEach(ctx, db, `SELECT p.proname, CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = current_schema() AND p.prokind IN ('f', 'p') ORDER BY p.proname`, nil, func(rows rowScanner) error {
		var r RoutineInfo
		if err := rows.Scan(&r.Name, &r.Kind); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	return out, err
}

func (postgresDialect) RoutineDDL(ctx context.Context, db Executor, _, _, name string) (string, error) {
	var ddl string
	err := db.QueryRowContext(ctx, `SELECT pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = current_schema() AND p.proname = $1 LIMIT 1`, name).Scan(&ddl)
	return ddl, err
}
