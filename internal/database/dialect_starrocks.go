package database

import (
	"context"
	"fmt"
	"strings"
)

// starrocksDialect 在 MySQL 协议之上：没有外键、触发器、存储过程，没有用户管理与可靠的受影响行数，
// 元信息较少，部分读取要容忍失败。StarRocks 不支持 COM_STMT_PREPARE，
// 连接 DSN 开启 interpolateParams，由驱动在客户端插值，因此这里同样用 ? 占位符。
type starrocksDialect struct{ mysqlDialect }

func (starrocksDialect) Label() string    { return "StarRocks" }
func (starrocksDialect) DefaultPort() int { return 9030 }

func (starrocksDialect) Caps() Caps {
	return Caps{
		RowEdit:      true,
		SessionUseDB: true, Network: true,
		ProcessList: true, CreateDatabase: true, MultiDatabase: true, Explain: true,
		BackslashEscape: true,
	}
}

func (starrocksDialect) ColumnTypes() []string {
	return []string{
		"BOOLEAN", "TINYINT", "SMALLINT", "INT", "BIGINT", "LARGEINT",
		"FLOAT", "DOUBLE", "DECIMAL",
		"CHAR", "VARCHAR", "STRING", "BINARY", "VARBINARY",
		"DATE", "DATETIME",
		"JSON", "ARRAY", "MAP", "STRUCT",
		"BITMAP", "HLL", "PERCENTILE",
	}
}

// starrocksSystemDBs StarRocks 需要过滤的系统数据库
var starrocksSystemDBs = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"sys":                true,
	"_statistics_":       true,
	"starrocks_monitor":  true,
}

func isStarRocksSystemDB(name string) bool { return starrocksSystemDBs[strings.ToLower(name)] }

func (d starrocksDialect) Databases(ctx context.Context, db Executor, withCounts bool) ([]DatabaseInfo, error) {
	return d.databases(ctx, db, withCounts, isStarRocksSystemDB, true)
}

func (d starrocksDialect) Columns(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error) {
	return d.columnsNoFK(ctx, db, dbName, table)
}

// TableStats StarRocks 可能不返回某些字段，取不到时只给基本信息而不是报错。
func (d starrocksDialect) TableStats(ctx context.Context, db Executor, dbName, table string) (*TableStats, error) {
	stats, err := d.mysqlDialect.TableStats(ctx, db, dbName, table)
	if err != nil {
		return &TableStats{Engine: "StarRocks"}, nil
	}
	return stats, nil
}

// TableOverview StarRocks 的元信息少：用表列表凑。
func (d starrocksDialect) TableOverview(ctx context.Context, db Executor, dbName string) ([]TableOverview, error) {
	tables, err := d.Tables(ctx, db, dbName)
	if err != nil {
		return nil, err
	}
	out := make([]TableOverview, 0, len(tables))
	for _, t := range tables {
		out = append(out, TableOverview{Name: t.Name, Type: t.Type, Rows: t.RowCount, DataSize: t.Size, IndexSize: -1, Comment: t.Comment})
	}
	return out, nil
}

// Indexes StarRocks 的 information_schema.STATISTICS 不一定兼容，用 SHOW INDEX；
// 某些表类型不支持 SHOW INDEX，此时返回空列表。
func (d starrocksDialect) Indexes(ctx context.Context, db Executor, dbName, table string) ([]IndexInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SHOW INDEX FROM %s FROM %s", d.QuoteIdent(table), d.QuoteIdent(dbName)))
	if err != nil {
		return []IndexInfo{}, nil
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return []IndexInfo{}, nil
	}
	colCount := len(cols)

	indexMap := make(map[string]*IndexInfo)
	var order []string
	for rows.Next() {
		scanVals := make([]any, colCount)
		scanPtrs := make([]any, colCount)
		for i := range scanVals {
			scanPtrs[i] = &scanVals[i]
		}
		if err := rows.Scan(scanPtrs...); err != nil || colCount < 5 {
			continue
		}
		nonUnique := fmt.Sprintf("%v", scanVals[1])
		keyName := fmt.Sprintf("%v", scanVals[2])
		colName := fmt.Sprintf("%v", scanVals[4])
		idxType := ""
		if colCount > 10 {
			idxType = fmt.Sprintf("%v", scanVals[10])
		}
		if _, exists := indexMap[keyName]; !exists {
			indexMap[keyName] = &IndexInfo{
				Name: keyName, Columns: []string{}, IsUnique: nonUnique == "0",
				IsPrimary: keyName == "PRIMARY", Type: idxType,
			}
			order = append(order, keyName)
		}
		indexMap[keyName].Columns = append(indexMap[keyName].Columns, colName)
	}
	indexes := []IndexInfo{}
	for _, name := range order {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}




