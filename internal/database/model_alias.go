package database

import "minidb/internal/model"

// 纯数据类型的定义在 model 包。这里用类型别名保留 database.XXX 的写法，
// 调用方可以渐进地改成直接引用 model。

type (
	DBType           = model.DBType
	SafeMode         = model.SafeMode
	SafeAction       = model.SafeAction
	SSHConfig        = model.SSHConfig
	ConnectionConfig = model.ConnectionConfig
	DatabaseInfo     = model.DatabaseInfo
	TableInfo        = model.TableInfo
	ColumnInfo       = model.ColumnInfo
	IndexInfo        = model.IndexInfo
	TableStats       = model.TableStats
	ForeignKeyInfo   = model.ForeignKeyInfo
	TriggerInfo      = model.TriggerInfo
	RoutineInfo      = model.RoutineInfo
	ColumnMeta       = model.ColumnMeta
)

// 数据库类型常量。
const (
	MySQL     = model.MySQL
	TiDB      = model.TiDB
	StarRocks = model.StarRocks
	Postgres  = model.Postgres
	SQLite    = model.SQLite
)

// 安全级别与发送前动作的常量。
const (
	SafeModeSilent = model.SafeModeSilent
	SafeModeAlert1 = model.SafeModeAlert1
	SafeModeAlert2 = model.SafeModeAlert2
	SafeModeSafe1  = model.SafeModeSafe1
	SafeModeSafe2  = model.SafeModeSafe2

	SafeNone     = model.SafeNone
	SafeConfirm  = model.SafeConfirm
	SafePassword = model.SafePassword
)

// SafeModes 按严格程度从低到高列出全部级别。
var SafeModes = model.SafeModes

// AllDBTypes 按界面展示顺序列出全部受支持的数据库类型。
func AllDBTypes() []DBType { return model.AllDBTypes() }
