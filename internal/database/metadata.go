package database

import (
	"database/sql"
	"fmt"
	"strings"
)

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
	case "mysql":
		return getMySQLDatabases(db)
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
	case "mysql":
		return getMySQLDatabaseNames(db)
	case "postgres":
		return getPostgresDatabases(db)
	case "sqlite":
		return []DatabaseInfo{{Name: "main"}}, nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
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
		if name == "information_schema" || name == "performance_schema" || name == "sys" {
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
		// 跳过系统数据库
		if name == "information_schema" || name == "performance_schema" || name == "sys" {
			continue
		}

		// 获取表数量
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
	case "mysql":
		return getMySQLTables(db, dbName)
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
	case "mysql":
		var name, ddl string
		err := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", dbName, tableName)).Scan(&name, &ddl)
		return ddl, err
	case "postgres":
		// PostgreSQL 没有简单的 SHOW CREATE TABLE，需要拼装
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
	case "mysql":
		return getMySQLTableStats(db, dbName, tableName)
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
	case "mysql":
		return getMySQLIndexes(db, dbName, tableName)
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
