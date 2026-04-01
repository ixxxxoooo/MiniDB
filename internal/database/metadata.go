package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetServerVersion 获取数据库服务器版本号
func GetServerVersion(db *sql.DB, dbType string) (string, error) {
	switch dbType {
	case "mysql", "tidb", "starrocks":
		var version string
		err := db.QueryRow("SELECT VERSION()").Scan(&version)
		return version, err
	case "postgres":
		var version string
		err := db.QueryRow("SELECT version()").Scan(&version)
		return version, err
	case "sqlite":
		var version string
		err := db.QueryRow("SELECT sqlite_version()").Scan(&version)
		if err == nil {
			version = "SQLite " + version
		}
		return version, err
	default:
		return "", fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Name       string `json:"name"`
	TableCount int    `json:"tableCount"`
	Size       int64  `json:"size"`
}

// TableInfo 表信息
type TableInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	RowCount int64  `json:"rowCount"`
	Size     int64  `json:"size"`
	Comment  string `json:"comment"`
}

// ColumnInfo 列信息
type ColumnInfo struct {
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Nullable        bool    `json:"nullable"`
	DefaultValue    *string `json:"defaultValue"`
	IsPrimary       bool    `json:"isPrimary"`
	IsAutoIncrement bool    `json:"isAutoIncrement"`
	Comment         string  `json:"comment"`
	MaxLength       *int64  `json:"maxLength"`
	CharacterSet    string  `json:"characterSet"`
	Collation       string  `json:"collation"`
	Extra           string  `json:"extra"`
	ForeignKey      string  `json:"foreignKey"`
}

// IndexInfo 索引信息
type IndexInfo struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"isUnique"`
	IsPrimary bool     `json:"isPrimary"`
	Type      string   `json:"type"`
	Comment   string   `json:"comment"`
}

// TableStats 表统计信息
type TableStats struct {
	RowCount   int64  `json:"rowCount"`
	DataSize   int64  `json:"dataSize"`
	IndexSize  int64  `json:"indexSize"`
	TotalSize  int64  `json:"totalSize"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
	Engine     string `json:"engine"`
	Collation  string `json:"collation"`
}

// GetDatabases 获取数据库列表（包含表数量，较慢）
func GetDatabases(db *sql.DB, dbType string) ([]DatabaseInfo, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLDatabases(db)
	case "starrocks":
		return getStarRocksDatabases(db)
	case "postgres":
		return getPostgresDatabases(db)
	case "sqlite":
		return []DatabaseInfo{{Name: "main", TableCount: 0}}, nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// GetDatabaseNames 快速获取数据库名称列表（不统计表数量，适合切换器）
func GetDatabaseNames(db *sql.DB, dbType string) ([]DatabaseInfo, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLDatabaseNames(db)
	case "starrocks":
		return getStarRocksDatabaseNames(db)
	case "postgres":
		return getPostgresDatabases(db)
	case "sqlite":
		return []DatabaseInfo{{Name: "main"}}, nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// mysqlSystemDBs MySQL 系统数据库（大小写不敏感匹配）
var mysqlSystemDBs = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"sys":               true,
	"mysql":             true,
}

func isMySQLSystemDB(name string) bool {
	return mysqlSystemDBs[strings.ToLower(name)]
}

func getMySQLDatabaseNames(db *sql.DB) ([]DatabaseInfo, error) {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if isMySQLSystemDB(name) {
			continue
		}
		databases = append(databases, DatabaseInfo{Name: name})
	}
	return databases, nil
}

func getMySQLDatabases(db *sql.DB) ([]DatabaseInfo, error) {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if isMySQLSystemDB(name) {
			continue
		}

		var count int
		countRow := db.QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?", name)
		countRow.Scan(&count)

		databases = append(databases, DatabaseInfo{
			Name:       name,
			TableCount: count,
		})
	}
	return databases, nil
}

func getPostgresDatabases(db *sql.DB) ([]DatabaseInfo, error) {
	rows, err := db.Query(
		"SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, DatabaseInfo{Name: name})
	}
	return databases, nil
}

// GetTables 获取表列表
func GetTables(db *sql.DB, dbType, dbName string) ([]TableInfo, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLTables(db, dbName)
	case "starrocks":
		// StarRocks 不支持 COM_STMT_PREPARE（预编译语句），需要用字符串拼接
		return getStarRocksTables(db, dbName)
	case "postgres":
		return getPostgresTables(db)
	case "sqlite":
		return getSQLiteTables(db)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

func getMySQLTables(db *sql.DB, dbName string) ([]TableInfo, error) {
	query := `SELECT table_name, table_type, 
		IFNULL(table_rows, 0), IFNULL(data_length, 0), 
		IFNULL(table_comment, '')
		FROM information_schema.tables 
		WHERE table_schema = ? 
		ORDER BY table_name`

	rows, err := db.Query(query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		var tableType string
		if err := rows.Scan(&t.Name, &tableType, &t.RowCount, &t.Size, &t.Comment); err != nil {
			return nil, err
		}
		if tableType == "VIEW" {
			t.Type = "view"
		} else {
			t.Type = "table"
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func getPostgresTables(db *sql.DB) ([]TableInfo, error) {
	query := `SELECT tablename, 'table' as type FROM pg_tables 
		WHERE schemaname = 'public' 
		UNION ALL 
		SELECT viewname, 'view' as type FROM pg_views 
		WHERE schemaname = 'public' 
		ORDER BY 1`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func getSQLiteTables(db *sql.DB) ([]TableInfo, error) {
	query := `SELECT name, type FROM sqlite_master 
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' 
		ORDER BY name`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// GetColumns 获取列信息
func GetColumns(db *sql.DB, dbType, dbName, tableName string) ([]ColumnInfo, error) {
	switch dbType {
	case "mysql":
		return getMySQLColumns(db, dbName, tableName)
	case "tidb":
		// TiDB 兼容 MySQL，但不支持外键，使用专用函数跳过外键查询
		return getMySQLColumnsNoFK(db, dbName, tableName)
	case "starrocks":
		// StarRocks 不支持外键和 COM_STMT_PREPARE，使用纯字符串拼接查询
		return getStarRocksColumns(db, dbName, tableName)
	case "postgres":
		return getPostgresColumns(db, tableName)
	case "sqlite":
		return getSQLiteColumns(db, tableName)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

func getMySQLColumns(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := `SELECT c.column_name, c.column_type, c.is_nullable, c.column_default,
		c.column_key, c.extra, c.column_comment, c.character_maximum_length,
		IFNULL(c.character_set_name, ''), IFNULL(c.collation_name, '')
		FROM information_schema.columns c
		WHERE c.table_schema = ? AND c.table_name = ?
		ORDER BY c.ordinal_position`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &key, &extra, &c.Comment, &c.MaxLength,
			&c.CharacterSet, &c.Collation); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsAutoIncrement = extra == "auto_increment"
		c.Extra = extra
		columns = append(columns, c)
	}

	// 查询外键信息，填充到对应列
	fkQuery := `SELECT kcu.column_name,
		CONCAT(kcu.referenced_table_name, '.', kcu.referenced_column_name) AS fk_ref
		FROM information_schema.key_column_usage kcu
		WHERE kcu.table_schema = ? AND kcu.table_name = ?
		AND kcu.referenced_table_name IS NOT NULL`
	fkRows, err := db.Query(fkQuery, dbName, tableName)
	if err == nil {
		defer fkRows.Close()
		fkMap := make(map[string]string)
		for fkRows.Next() {
			var colName, fkRef string
			if fkRows.Scan(&colName, &fkRef) == nil {
				fkMap[colName] = fkRef
			}
		}
		for i := range columns {
			if ref, ok := fkMap[columns[i].Name]; ok {
				columns[i].ForeignKey = ref
			}
		}
	}

	return columns, nil
}

func getPostgresColumns(db *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := `SELECT c.column_name, c.data_type, c.is_nullable, c.column_default,
		CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary,
		c.character_maximum_length
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &c.IsPrimary, &c.MaxLength); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		columns = append(columns, c)
	}
	return columns, nil
}

func getSQLiteColumns(db *sql.DB, tableName string) ([]ColumnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var cid int
		var c ColumnInfo
		var notNull int
		var pk int
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &c.DefaultValue, &pk); err != nil {
			return nil, err
		}
		c.Nullable = notNull == 0
		c.IsPrimary = pk == 1
		columns = append(columns, c)
	}
	return columns, nil
}

// GetDDL 获取表 DDL
func GetDDL(db *sql.DB, dbType, dbName, tableName string) (string, error) {
	switch dbType {
	case "mysql", "tidb", "starrocks":
		// TiDB 和 StarRocks 均支持 SHOW CREATE TABLE
		var name, ddl string
		err := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", dbName, tableName)).Scan(&name, &ddl)
		return ddl, err
	case "postgres":
		return getPostgresDDL(db, tableName)
	case "sqlite":
		var ddl string
		err := db.QueryRow("SELECT sql FROM sqlite_master WHERE name = ?", tableName).Scan(&ddl)
		return ddl, err
	default:
		return "", fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// GetTableStats 获取表的详细统计信息
func GetTableStats(db *sql.DB, dbType, dbName, tableName string) (*TableStats, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLTableStats(db, dbName, tableName)
	case "starrocks":
		return getStarRocksTableStats(db, dbName, tableName)
	case "postgres":
		return getPostgresTableStats(db, tableName)
	case "sqlite":
		return getSQLiteTableStats(db, tableName)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

func getMySQLTableStats(db *sql.DB, dbName, tableName string) (*TableStats, error) {
	query := `SELECT IFNULL(table_rows,0), IFNULL(data_length,0), IFNULL(index_length,0),
		IFNULL(data_length+index_length,0),
		IFNULL(create_time,''), IFNULL(update_time,''),
		IFNULL(engine,''), IFNULL(table_collation,'')
		FROM information_schema.tables
		WHERE table_schema = ? AND table_name = ?`
	var stats TableStats
	err := db.QueryRow(query, dbName, tableName).Scan(
		&stats.RowCount, &stats.DataSize, &stats.IndexSize, &stats.TotalSize,
		&stats.CreateTime, &stats.UpdateTime, &stats.Engine, &stats.Collation,
	)
	return &stats, err
}

func getPostgresTableStats(db *sql.DB, tableName string) (*TableStats, error) {
	stats := &TableStats{Engine: "PostgreSQL"}
	// 通过 pg_stat_user_tables 获取行数估算
	db.QueryRow(`SELECT COALESCE(n_live_tup, 0) FROM pg_stat_user_tables WHERE relname = $1`, tableName).Scan(&stats.RowCount)
	// 通过 pg_total_relation_size 获取大小
	db.QueryRow(`SELECT pg_total_relation_size($1::regclass)`, tableName).Scan(&stats.TotalSize)
	db.QueryRow(`SELECT pg_relation_size($1::regclass)`, tableName).Scan(&stats.DataSize)
	stats.IndexSize = stats.TotalSize - stats.DataSize
	return stats, nil
}

func getSQLiteTableStats(db *sql.DB, tableName string) (*TableStats, error) {
	stats := &TableStats{Engine: "SQLite"}
	db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&stats.RowCount)
	return stats, nil
}

func getPostgresDDL(db *sql.DB, tableName string) (string, error) {
	// 简化版: 获取表的列定义
	columns, err := getPostgresColumns(db, tableName)
	if err != nil {
		return "", err
	}

	ddl := fmt.Sprintf("CREATE TABLE %s (\n", tableName)
	for i, c := range columns {
		nullable := ""
		if !c.Nullable {
			nullable = " NOT NULL"
		}
		def := ""
		if c.DefaultValue != nil {
			def = fmt.Sprintf(" DEFAULT %s", *c.DefaultValue)
		}
		ddl += fmt.Sprintf("  %s %s%s%s", c.Name, c.Type, nullable, def)
		if i < len(columns)-1 {
			ddl += ","
		}
		ddl += "\n"
	}
	ddl += ");"
	return ddl, nil
}

// GetIndexes 获取表的索引信息
func GetIndexes(db *sql.DB, dbType, dbName, tableName string) ([]IndexInfo, error) {
	switch dbType {
	case "mysql", "tidb":
		return getMySQLIndexes(db, dbName, tableName)
	case "starrocks":
		// StarRocks 的 information_schema.STATISTICS 可能不完全兼容，使用 SHOW INDEX 方式
		return getStarRocksIndexes(db, dbName, tableName)
	case "postgres":
		return getPostgresIndexes(db, tableName)
	case "sqlite":
		return getSQLiteIndexes(db, tableName)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

func getMySQLIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	query := `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE, SEQ_IN_INDEX, NULLABLE, INDEX_COMMENT
		FROM information_schema.STATISTICS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 按索引名分组，因为一个索引可能包含多列
	indexMap := make(map[string]*IndexInfo)
	var indexOrder []string

	for rows.Next() {
		var idxName, colName, idxType, nullable, comment string
		var nonUnique int
		var seqInIndex int
		if err := rows.Scan(&idxName, &colName, &nonUnique, &idxType, &seqInIndex, &nullable, &comment); err != nil {
			return nil, err
		}

		if _, exists := indexMap[idxName]; !exists {
			indexMap[idxName] = &IndexInfo{
				Name:      idxName,
				Columns:   []string{},
				IsUnique:  nonUnique == 0,
				IsPrimary: idxName == "PRIMARY",
				Type:      idxType,
				Comment:   comment,
			}
			indexOrder = append(indexOrder, idxName)
		}
		indexMap[idxName].Columns = append(indexMap[idxName].Columns, colName)
	}

	var indexes []IndexInfo
	for _, name := range indexOrder {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}

func getPostgresIndexes(db *sql.DB, tableName string) ([]IndexInfo, error) {
	query := `SELECT i.relname AS index_name,
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
		ORDER BY i.relname`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var idx IndexInfo
		var colStr string
		if err := rows.Scan(&idx.Name, &colStr, &idx.IsUnique, &idx.IsPrimary, &idx.Type); err != nil {
			return nil, err
		}
		idx.Columns = strings.Split(colStr, ",")
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

func getSQLiteIndexes(db *sql.DB, tableName string) ([]IndexInfo, error) {
	query := fmt.Sprintf("PRAGMA index_list(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}

		idx := IndexInfo{
			Name:      name,
			IsUnique:  unique == 1,
			IsPrimary: origin == "pk",
			Type:      "BTREE",
		}

		// 获取索引包含的列
		colRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", name))
		if err == nil {
			for colRows.Next() {
				var seqno, cid int
				var colName string
				if err := colRows.Scan(&seqno, &cid, &colName); err == nil {
					idx.Columns = append(idx.Columns, colName)
				}
			}
			colRows.Close()
		}

		indexes = append(indexes, idx)
	}
	return indexes, nil
}

// ========== StarRocks 专用函数 ==========
// StarRocks 不支持 COM_STMT_PREPARE（预编译语句），所有查询使用字符串拼接避免 ? 占位符

// starrocksSystemDBs StarRocks 需要过滤的系统数据库
var starrocksSystemDBs = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"sys":               true,
	"_statistics_":      true,
	"starrocks_monitor":  true,
}

func isStarRocksSystemDB(name string) bool {
	return starrocksSystemDBs[strings.ToLower(name)]
}

// getStarRocksDatabaseNames 获取 StarRocks 数据库名称列表
func getStarRocksDatabaseNames(db *sql.DB) ([]DatabaseInfo, error) {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if isStarRocksSystemDB(name) {
			continue
		}
		databases = append(databases, DatabaseInfo{Name: name})
	}
	return databases, nil
}

// getStarRocksDatabases 获取 StarRocks 数据库列表（含表数量）
func getStarRocksDatabases(db *sql.DB) ([]DatabaseInfo, error) {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if isStarRocksSystemDB(name) {
			continue
		}

		// StarRocks 不支持预编译语句，使用字符串拼接
		var count int
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '%s'",
			strings.ReplaceAll(name, "'", "''"))
		db.QueryRow(countSQL).Scan(&count)

		databases = append(databases, DatabaseInfo{
			Name:       name,
			TableCount: count,
		})
	}
	return databases, nil
}

// getStarRocksTables 获取 StarRocks 表列表（避免预编译语句）
func getStarRocksTables(db *sql.DB, dbName string) ([]TableInfo, error) {
	// StarRocks 不支持 COM_STMT_PREPARE，使用字符串拼接
	escapedDB := strings.ReplaceAll(dbName, "'", "''")
	query := fmt.Sprintf(`SELECT table_name, table_type, 
		IFNULL(table_rows, 0), IFNULL(data_length, 0), 
		IFNULL(table_comment, '')
		FROM information_schema.tables 
		WHERE table_schema = '%s' 
		ORDER BY table_name`, escapedDB)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		var tableType string
		if err := rows.Scan(&t.Name, &tableType, &t.RowCount, &t.Size, &t.Comment); err != nil {
			return nil, err
		}
		if tableType == "VIEW" {
			t.Type = "view"
		} else {
			t.Type = "table"
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// getMySQLColumnsNoFK 获取 MySQL 兼容数据库列信息（跳过外键查询，适用于 TiDB）
func getMySQLColumnsNoFK(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := `SELECT c.column_name, c.column_type, c.is_nullable, c.column_default,
		c.column_key, c.extra, c.column_comment, c.character_maximum_length,
		IFNULL(c.character_set_name, ''), IFNULL(c.collation_name, '')
		FROM information_schema.columns c
		WHERE c.table_schema = ? AND c.table_name = ?
		ORDER BY c.ordinal_position`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &key, &extra, &c.Comment, &c.MaxLength,
			&c.CharacterSet, &c.Collation); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsAutoIncrement = extra == "auto_increment"
		c.Extra = extra
		columns = append(columns, c)
	}

	return columns, nil
}

// getStarRocksColumns 获取 StarRocks 列信息（避免预编译语句，跳过外键）
func getStarRocksColumns(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	escapedDB := strings.ReplaceAll(dbName, "'", "''")
	escapedTable := strings.ReplaceAll(tableName, "'", "''")

	query := fmt.Sprintf(`SELECT c.column_name, c.column_type, c.is_nullable, c.column_default,
		c.column_key, c.extra, c.column_comment, c.character_maximum_length,
		IFNULL(c.character_set_name, ''), IFNULL(c.collation_name, '')
		FROM information_schema.columns c
		WHERE c.table_schema = '%s' AND c.table_name = '%s'
		ORDER BY c.ordinal_position`, escapedDB, escapedTable)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &key, &extra, &c.Comment, &c.MaxLength,
			&c.CharacterSet, &c.Collation); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsAutoIncrement = extra == "auto_increment"
		c.Extra = extra
		columns = append(columns, c)
	}

	return columns, nil
}

// getStarRocksTableStats 获取 StarRocks 表统计（避免预编译语句）
func getStarRocksTableStats(db *sql.DB, dbName, tableName string) (*TableStats, error) {
	escapedDB := strings.ReplaceAll(dbName, "'", "''")
	escapedTable := strings.ReplaceAll(tableName, "'", "''")

	query := fmt.Sprintf(`SELECT IFNULL(table_rows,0), IFNULL(data_length,0), IFNULL(index_length,0),
		IFNULL(data_length+index_length,0),
		IFNULL(create_time,''), IFNULL(update_time,''),
		IFNULL(engine,''), IFNULL(table_collation,'')
		FROM information_schema.tables
		WHERE table_schema = '%s' AND table_name = '%s'`, escapedDB, escapedTable)

	var stats TableStats
	err := db.QueryRow(query).Scan(
		&stats.RowCount, &stats.DataSize, &stats.IndexSize, &stats.TotalSize,
		&stats.CreateTime, &stats.UpdateTime, &stats.Engine, &stats.Collation,
	)
	if err != nil {
		// StarRocks 可能不返回某些字段，返回基本信息
		stats.Engine = "StarRocks"
		return &stats, nil
	}
	return &stats, nil
}

// getStarRocksIndexes 获取 StarRocks 索引信息（通过 SHOW INDEX 方式）
func getStarRocksIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	query := fmt.Sprintf("SHOW INDEX FROM `%s` FROM `%s`", tableName, dbName)
	rows, err := db.Query(query)
	if err != nil {
		// StarRocks 某些表类型可能不支持 SHOW INDEX，返回空列表
		return []IndexInfo{}, nil
	}
	defer rows.Close()

	colTypes, err := rows.Columns()
	if err != nil {
		return []IndexInfo{}, nil
	}
	colCount := len(colTypes)

	indexMap := make(map[string]*IndexInfo)
	var indexOrder []string

	for rows.Next() {
		scanVals := make([]interface{}, colCount)
		scanPtrs := make([]interface{}, colCount)
		for i := range scanVals {
			scanPtrs[i] = &scanVals[i]
		}
		if err := rows.Scan(scanPtrs...); err != nil {
			continue
		}

		if colCount < 5 {
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
				Name:      keyName,
				Columns:   []string{},
				IsUnique:  nonUnique == "0",
				IsPrimary: keyName == "PRIMARY",
				Type:      idxType,
			}
			indexOrder = append(indexOrder, keyName)
		}
		indexMap[keyName].Columns = append(indexMap[keyName].Columns, colName)
	}

	var indexes []IndexInfo
	for _, name := range indexOrder {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}
