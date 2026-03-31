package database

import (
	"database/sql"
	"fmt"
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

// GetDatabases 获取数据库列表
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
	query := `SELECT column_name, column_type, is_nullable, column_default,
		column_key, extra, column_comment, character_maximum_length
		FROM information_schema.columns 
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`

	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &key, &extra, &c.Comment, &c.MaxLength); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsAutoIncrement = extra == "auto_increment"
		columns = append(columns, c)
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
