package database

import (
	"context"
	"fmt"
	"strings"
)

// SQL Server 的元数据读取。表都限定在登录用户的默认架构（SCHEMA_ID() / SCHEMA_NAME()），
// 与不带架构前缀的表名解析到的是同一批表。多行聚合（索引列、外键列）放在 Go 里做，不依赖 STRING_AGG（2017+）。

// objectID 是把表名解析成 object_id 的表达式；参数是第 n 个占位符。
func objectID(n int) string {
	return fmt.Sprintf("OBJECT_ID(QUOTENAME(SCHEMA_NAME()) + '.' + QUOTENAME(@p%d))", n)
}

func (sqlserverDialect) Databases(ctx context.Context, db Executor, _ bool) ([]DatabaseInfo, error) {
	var out []DatabaseInfo
	// 1 至 4 是 master、tempdb、model、msdb
	err := queryEach(ctx, db, `SELECT name FROM sys.databases WHERE database_id > 4 AND state = 0 ORDER BY name`, nil,
		func(rows rowScanner) error {
			var d DatabaseInfo
			if err := rows.Scan(&d.Name); err != nil {
				return err
			}
			out = append(out, d)
			return nil
		})
	return out, err
}

func (sqlserverDialect) Tables(ctx context.Context, db Executor, _ string) ([]TableInfo, error) {
	var out []TableInfo
	err := queryEach(ctx, db, `SELECT o.name, CASE o.type WHEN 'V' THEN 'view' ELSE 'table' END
		FROM sys.objects o WHERE o.schema_id = SCHEMA_ID() AND o.type IN ('U', 'V') AND o.is_ms_shipped = 0
		ORDER BY o.name`, nil, func(rows rowScanner) error {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return err
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

// unwrapDefault 去掉 sys.default_constraints.definition 外面包的括号：((0)) → 0，('a') → 'a'。
func unwrapDefault(def string) string {
	def = strings.TrimSpace(def)
	for len(def) >= 2 && def[0] == '(' && def[len(def)-1] == ')' && balanced(def[1:len(def)-1]) {
		def = strings.TrimSpace(def[1 : len(def)-1])
	}
	return def
}

// balanced 判断括号是否成对（忽略引号里的括号）。
func balanced(s string) bool {
	depth, inStr := 0, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'':
			inStr = !inStr
		case inStr:
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func (d sqlserverDialect) Columns(ctx context.Context, db Executor, _, table string) ([]ColumnInfo, error) {
	var out []ColumnInfo
	err := queryEach(ctx, db, `SELECT c.name,
		CASE WHEN t.name IN ('varchar', 'nvarchar', 'char', 'nchar', 'varbinary', 'binary')
		       THEN t.name + '(' + CASE WHEN c.max_length = -1 THEN 'max'
		                                WHEN t.name IN ('nvarchar', 'nchar') THEN CAST(c.max_length / 2 AS varchar(10))
		                                ELSE CAST(c.max_length AS varchar(10)) END + ')'
		     WHEN t.name IN ('decimal', 'numeric') THEN t.name + '(' + CAST(c.precision AS varchar(10)) + ',' + CAST(c.scale AS varchar(10)) + ')'
		     WHEN t.name IN ('datetime2', 'datetimeoffset', 'time') THEN t.name + '(' + CAST(c.scale AS varchar(10)) + ')'
		     ELSE t.name END,
		c.is_nullable, dc.definition,
		CASE WHEN pk.column_id IS NULL THEN 0 ELSE 1 END,
		c.is_identity,
		COALESCE(CAST(ep.value AS nvarchar(4000)), ''),
		CASE WHEN t.name IN ('varchar', 'char') THEN c.max_length
		     WHEN t.name IN ('nvarchar', 'nchar') AND c.max_length > 0 THEN c.max_length / 2
		     WHEN t.name IN ('varchar', 'nvarchar', 'varbinary') AND c.max_length = -1 THEN -1 END,
		COALESCE(c.collation_name, ''),
		c.is_computed
		FROM sys.columns c
		JOIN sys.types t ON t.user_type_id = c.user_type_id
		LEFT JOIN sys.default_constraints dc ON dc.object_id = c.default_object_id
		LEFT JOIN (SELECT ic.object_id, ic.column_id FROM sys.indexes i
		           JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		           WHERE i.is_primary_key = 1) pk ON pk.object_id = c.object_id AND pk.column_id = c.column_id
		LEFT JOIN sys.extended_properties ep ON ep.major_id = c.object_id AND ep.minor_id = c.column_id
		           AND ep.class = 1 AND ep.name = 'MS_Description'
		WHERE c.object_id = `+objectID(1)+`
		ORDER BY c.column_id`, []any{table}, func(rows rowScanner) error {
		var c ColumnInfo
		var nullable, primary, identity, computed bool
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &primary, &identity, &c.Comment, &c.MaxLength, &c.Collation, &computed); err != nil {
			return err
		}
		c.Nullable, c.IsPrimary, c.IsAutoIncrement = nullable, primary, identity
		if c.DefaultValue != nil {
			v := unwrapDefault(*c.DefaultValue)
			c.DefaultValue = &v
		}
		switch {
		case identity:
			c.Extra = "identity"
		case computed:
			c.Extra = "computed"
		}
		out = append(out, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	d.fillForeignKeys(ctx, db, table, out)
	return out, nil
}

// fillForeignKeys 把外键引用（"表.列"）填到列上；读取失败时静默跳过，不影响列清单。
func (d sqlserverDialect) fillForeignKeys(ctx context.Context, db Executor, table string, cols []ColumnInfo) {
	fks, err := d.ForeignKeys(ctx, db, "", table)
	if err != nil {
		return
	}
	refs := map[string]string{}
	for _, fk := range fks {
		for i, c := range fk.Columns {
			if i < len(fk.RefColumns) {
				refs[c] = fk.RefTable + "." + fk.RefColumns[i]
			}
		}
	}
	for i := range cols {
		if r, ok := refs[cols[i].Name]; ok {
			cols[i].ForeignKey = r
		}
	}
}

func (sqlserverDialect) Indexes(ctx context.Context, db Executor, _, table string) ([]IndexInfo, error) {
	var out []IndexInfo
	pos := map[string]int{}
	err := queryEach(ctx, db, `SELECT i.name, col.name, i.is_unique, i.is_primary_key, i.type_desc
		FROM sys.indexes i
		JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id AND ic.is_included_column = 0
		JOIN sys.columns col ON col.object_id = ic.object_id AND col.column_id = ic.column_id
		WHERE i.object_id = `+objectID(1)+` AND i.type > 0 AND i.name IS NOT NULL
		ORDER BY i.name, ic.key_ordinal`, []any{table}, func(rows rowScanner) error {
		var name, col, typ string
		var unique, primary bool
		if err := rows.Scan(&name, &col, &unique, &primary, &typ); err != nil {
			return err
		}
		i, ok := pos[name]
		if !ok {
			i = len(out)
			pos[name] = i
			out = append(out, IndexInfo{Name: name, IsUnique: unique, IsPrimary: primary, Type: typ})
		}
		out[i].Columns = append(out[i].Columns, col)
		return nil
	})
	return out, err
}

// sqlserverFKRule 把 sys.foreign_keys 的动作描述（NO_ACTION、SET_NULL）写成 SQL 里的样子。
func sqlserverFKRule(desc string) string { return strings.ReplaceAll(desc, "_", " ") }

const sqlserverFKSelect = `SELECT pt.name, fk.name, pc.name, rt.name, rc.name,
		fk.update_referential_action_desc, fk.delete_referential_action_desc
	FROM sys.foreign_keys fk
	JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
	JOIN sys.tables pt ON pt.object_id = fkc.parent_object_id
	JOIN sys.columns pc ON pc.object_id = fkc.parent_object_id AND pc.column_id = fkc.parent_column_id
	JOIN sys.tables rt ON rt.object_id = fkc.referenced_object_id
	JOIN sys.columns rc ON rc.object_id = fkc.referenced_object_id AND rc.column_id = fkc.referenced_column_id
	WHERE pt.schema_id = SCHEMA_ID() AND rt.schema_id = SCHEMA_ID()`

func (sqlserverDialect) scanForeignKeys(ctx context.Context, db Executor, where, order string, args []any) ([]TableForeignKey, error) {
	var g fkGrouper
	err := queryEach(ctx, db, sqlserverFKSelect+where+order, args, func(rows rowScanner) error {
		var table, name, col, refTable, refCol, upd, del string
		if err := rows.Scan(&table, &name, &col, &refTable, &refCol, &upd, &del); err != nil {
			return err
		}
		g.add(table, name, col, refTable, refCol, sqlserverFKRule(upd), sqlserverFKRule(del))
		return nil
	})
	return g.out, err
}

func (d sqlserverDialect) ForeignKeys(ctx context.Context, db Executor, _, table string) ([]ForeignKeyInfo, error) {
	list, err := d.scanForeignKeys(ctx, db, ` AND fk.parent_object_id = `+objectID(1), ` ORDER BY fk.name, fkc.constraint_column_id`, []any{table})
	out := make([]ForeignKeyInfo, 0, len(list))
	for _, fk := range list {
		out = append(out, fk.ForeignKeyInfo)
	}
	return out, err
}

// AllForeignKeys 用一次查询取回默认架构里所有表的外键（ER 图用）。
func (d sqlserverDialect) AllForeignKeys(ctx context.Context, db Executor, _ string) ([]TableForeignKey, error) {
	return d.scanForeignKeys(ctx, db, "", ` ORDER BY pt.name, fk.name, fkc.constraint_column_id`, nil)
}



