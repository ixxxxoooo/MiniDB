package database

import (
	"context"
	"fmt"
	"strings"
)

// SQL Server 的表统计、概览和 DDL 合成。

func (d sqlserverDialect) TableStats(ctx context.Context, db Executor, _, table string) (*TableStats, error) {
	stats := &TableStats{Engine: "SQL Server"}
	// 取不到的项保持 0：统计只是展示用，不因权限等原因让整个页面失败
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(p.rows), 0) FROM sys.partitions p WHERE p.object_id = `+objectID(1)+` AND p.index_id IN (0, 1)`, table).Scan(&stats.RowCount)
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN p.index_id < 2 THEN a.total_pages ELSE 0 END), 0) * 8192,
			COALESCE(SUM(CASE WHEN p.index_id >= 2 THEN a.total_pages ELSE 0 END), 0) * 8192
		FROM sys.partitions p JOIN sys.allocation_units a ON `+allocJoin+` WHERE p.object_id = `+objectID(1), table).Scan(&stats.DataSize, &stats.IndexSize)
	stats.TotalSize = stats.DataSize + stats.IndexSize
	_ = db.QueryRowContext(ctx, `SELECT CONVERT(varchar(19), o.create_date, 120), CONVERT(varchar(19), o.modify_date, 120)
		FROM sys.objects o WHERE o.object_id = `+objectID(1), table).Scan(&stats.CreateTime, &stats.UpdateTime)
	_ = db.QueryRowContext(ctx, `SELECT CAST(DATABASEPROPERTYEX(DB_NAME(), 'Collation') AS nvarchar(128))`).Scan(&stats.Collation)
	return stats, nil
}

// allocJoin 是 sys.partitions 与 sys.allocation_units 的连接条件：行内数据和溢出数据挂在 hobt_id 上，LOB 挂在 partition_id 上。
const allocJoin = `((a.type IN (1, 3) AND a.container_id = p.hobt_id) OR (a.type = 2 AND a.container_id = p.partition_id))`

func (sqlserverDialect) TableOverview(ctx context.Context, db Executor, _ string) ([]TableOverview, error) {
	var out []TableOverview
	err := queryEach(ctx, db, `SELECT o.name, CASE o.type WHEN 'V' THEN 'view' ELSE 'table' END,
		COALESCE(r.cnt, -1), COALESCE(s.data_size, -1), COALESCE(s.index_size, -1),
		COALESCE(CAST(ep.value AS nvarchar(4000)), ''),
		CONVERT(varchar(19), o.create_date, 120), CONVERT(varchar(19), o.modify_date, 120)
		FROM sys.objects o
		LEFT JOIN (SELECT object_id, SUM(CASE WHEN index_id IN (0, 1) THEN rows ELSE 0 END) AS cnt
		           FROM sys.partitions GROUP BY object_id) r ON r.object_id = o.object_id
		LEFT JOIN (SELECT p.object_id,
		                  SUM(CASE WHEN p.index_id < 2 THEN a.total_pages ELSE 0 END) * 8192 AS data_size,
		                  SUM(CASE WHEN p.index_id >= 2 THEN a.total_pages ELSE 0 END) * 8192 AS index_size
		           FROM sys.partitions p JOIN sys.allocation_units a ON `+allocJoin+`
		           GROUP BY p.object_id) s ON s.object_id = o.object_id
		LEFT JOIN sys.extended_properties ep ON ep.major_id = o.object_id AND ep.minor_id = 0 AND ep.class = 1 AND ep.name = 'MS_Description'
		WHERE o.schema_id = SCHEMA_ID() AND o.type IN ('U', 'V') AND o.is_ms_shipped = 0
		ORDER BY o.name`, nil, func(rows rowScanner) error {
		t := TableOverview{Engine: "SQL Server"}
		if err := rows.Scan(&t.Name, &t.Type, &t.Rows, &t.DataSize, &t.IndexSize, &t.Comment, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return err
		}
		if t.Type == "view" {
			t.Engine = ""
		}
		out = append(out, t)
		return nil
	})
	return out, err
}

// DDL 由元数据合成：视图直接取定义；表由列、主键约束、其余索引和外键拼出 CREATE TABLE 与后续语句。
func (d sqlserverDialect) DDL(ctx context.Context, db Executor, dbName, table string) (string, error) {
	var def *string
	if err := db.QueryRowContext(ctx, `SELECT OBJECT_DEFINITION(o.object_id) FROM sys.objects o
		WHERE o.object_id = `+objectID(1)+` AND o.type = 'V'`, table).Scan(&def); err == nil && def != nil {
		return strings.TrimSpace(*def), nil
	}
	cols, err := d.Columns(ctx, db, dbName, table)
	if err != nil {
		return "", err
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("表 %s 不存在或没有列", table)
	}
	seed, step := int64(1), int64(1)
	_ = db.QueryRowContext(ctx, `SELECT CAST(seed_value AS bigint), CAST(increment_value AS bigint) FROM sys.identity_columns WHERE object_id = `+objectID(1), table).Scan(&seed, &step)
	idx, err := d.Indexes(ctx, db, dbName, table)
	if err != nil {
		return "", err
	}
	fks, err := d.ForeignKeys(ctx, db, dbName, table)
	if err != nil {
		return "", err
	}
	return buildSQLServerDDL(table, cols, idx, fks, seed, step), nil
}

// buildSQLServerDDL 是纯函数：由元数据拼出建表语句，主键写在表里，其余索引和外键用后续语句。
func buildSQLServerDDL(table string, cols []ColumnInfo, idx []IndexInfo, fks []ForeignKeyInfo, seed, step int64) string {
	q := sqlserverDialect{}.QuoteIdent
	list := func(names []string) string {
		quoted := make([]string, len(names))
		for i, n := range names {
			quoted[i] = q(n)
		}
		return strings.Join(quoted, ", ")
	}
	var lines []string
	for _, c := range cols {
		line := "  " + q(c.Name) + " " + c.Type
		if c.IsAutoIncrement {
			line += fmt.Sprintf(" IDENTITY(%d,%d)", seed, step)
		}
		if !c.Nullable {
			line += " NOT NULL"
		}
		if c.DefaultValue != nil {
			line += " DEFAULT " + *c.DefaultValue
		}
		lines = append(lines, line)
	}
	var after []string
	for _, ix := range idx {
		switch {
		case ix.IsPrimary:
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s PRIMARY KEY %s (%s)", q(ix.Name), clusteredWord(ix.Type), list(ix.Columns)))
		default:
			u := ""
			if ix.IsUnique {
				u = "UNIQUE "
			}
			after = append(after, fmt.Sprintf("CREATE %s%s INDEX %s ON %s (%s);", u, clusteredWord(ix.Type), q(ix.Name), q(table), list(ix.Columns)))
		}
	}
	for _, fk := range fks {
		s := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", q(table), q(fk.Name), list(fk.Columns), q(fk.RefTable), list(fk.RefColumns))
		if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
			s += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
			s += " ON UPDATE " + fk.OnUpdate
		}
		after = append(after, s+";")
	}
	out := fmt.Sprintf("CREATE TABLE %s (\n%s\n);", q(table), strings.Join(lines, ",\n"))
	if len(after) > 0 {
		out += "\n\n" + strings.Join(after, "\n")
	}
	return out
}

// clusteredWord 取索引类型描述里的 CLUSTERED / NONCLUSTERED；认不出的返回空。
func clusteredWord(typeDesc string) string {
	t := strings.ToUpper(typeDesc)
	switch {
	case strings.HasPrefix(t, "NONCLUSTERED"):
		return "NONCLUSTERED"
	case strings.HasPrefix(t, "CLUSTERED"):
		return "CLUSTERED"
	}
	return ""
}
