package database

import (
	"context"
	"strings"
)

// 元数据读取的统一入口。各数据库的差异在 dialect_*.go 里，这里只做"按类型找方言再调用"，
// 并统一补上通用的后处理。所有函数都带 ctx：调用方可取消，也可以用 WithTimeout 限定时长。

// GetServerVersion 获取数据库服务器版本号
func GetServerVersion(ctx context.Context, db Executor, dbType DBType) (string, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return "", err
	}
	return d.ServerVersion(ctx, db)
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
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Nullable        bool     `json:"nullable"`
	DefaultValue    *string  `json:"defaultValue"`
	IsPrimary       bool     `json:"isPrimary"`
	IsAutoIncrement bool     `json:"isAutoIncrement"`
	Comment         string   `json:"comment"`
	MaxLength       *int64   `json:"maxLength"`
	CharacterSet    string   `json:"characterSet"`
	Collation       string   `json:"collation"`
	Extra           string   `json:"extra"`
	ForeignKey      string   `json:"foreignKey"`
	EnumOptions     []string `json:"enumOptions,omitempty"`
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
func GetDatabases(ctx context.Context, db Executor, dbType DBType) ([]DatabaseInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.Databases(ctx, db, true)
}

// GetDatabaseNames 快速获取数据库名称列表（不统计表数量，适合切换器）
func GetDatabaseNames(ctx context.Context, db Executor, dbType DBType) ([]DatabaseInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.Databases(ctx, db, false)
}

// GetTables 获取表列表
func GetTables(ctx context.Context, db Executor, dbType DBType, dbName string) ([]TableInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.Tables(ctx, db, dbName)
}

// GetColumns 获取列信息，并从类型字符串里解析 enum/set 的选项。
func GetColumns(ctx context.Context, db Executor, dbType DBType, dbName, tableName string) ([]ColumnInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	cols, err := d.Columns(ctx, db, dbName, tableName)
	if err != nil {
		return nil, err
	}
	for i := range cols {
		cols[i].EnumOptions = parseEnumOptionsFromType(cols[i].Type)
	}
	return cols, nil
}

// GetDDL 获取表 DDL
func GetDDL(ctx context.Context, db Executor, dbType DBType, dbName, tableName string) (string, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return "", err
	}
	return d.DDL(ctx, db, dbName, tableName)
}

// GetTableStats 获取表的详细统计信息
func GetTableStats(ctx context.Context, db Executor, dbType DBType, dbName, tableName string) (*TableStats, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.TableStats(ctx, db, dbName, tableName)
}

// GetIndexes 获取表的索引信息
func GetIndexes(ctx context.Context, db Executor, dbType DBType, dbName, tableName string) ([]IndexInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.Indexes(ctx, db, dbName, tableName)
}

// parseEnumOptionsFromType 从列类型字符串中解析 enum('a','b') 或 set('a','b') 的选项列表
func parseEnumOptionsFromType(colType string) []string {
	t := strings.ToLower(strings.TrimSpace(colType))
	var prefix string
	if strings.HasPrefix(t, "enum(") {
		prefix = "enum("
	} else if strings.HasPrefix(t, "set(") {
		prefix = "set("
	} else {
		return nil
	}
	inner := colType[len(prefix):]
	if idx := strings.LastIndex(inner, ")"); idx >= 0 {
		inner = inner[:idx]
	}
	var result []string
	for len(inner) > 0 {
		inner = strings.TrimSpace(inner)
		if inner == "" {
			break
		}
		if inner[0] == '\'' {
			end := 1
			for end < len(inner) {
				if inner[end] == '\'' {
					if end+1 < len(inner) && inner[end+1] == '\'' {
						end += 2
						continue
					}
					break
				}
				if inner[end] == '\\' && end+1 < len(inner) {
					end += 2
					continue
				}
				end++
			}
			val := inner[1:end]
			val = strings.ReplaceAll(val, "''", "'")
			val = strings.ReplaceAll(val, "\\'", "'")
			result = append(result, val)
			inner = inner[end+1:]
		}
		// 跳过逗号和空白
		if idx := strings.Index(inner, ","); idx >= 0 {
			inner = inner[idx+1:]
		} else {
			break
		}
	}
	return result
}
