// Package model 放整个应用共用的纯数据类型：连接配置、库表结构信息、数据库类型等。
// 它不依赖任何内部包，database、ai 等包用类型别名引用这里的定义，
// 界面层则可以只依赖 model 而不必为了一个结构体去依赖 database。
package model

// DBType 是数据库类型。底层仍是字符串，JSON 与已落盘的连接配置保持兼容；
// 用具名类型后，漏写 case 或传错类型会在编译期暴露，而不是运行时悄悄走进 default。
type DBType string

// 支持的数据库类型。除 dialect_*.go 与本文件外，其他地方一律用这些常量，不写字符串字面量。
const (
	MySQL     DBType = "mysql"
	MariaDB   DBType = "mariadb"
	TiDB      DBType = "tidb"
	StarRocks DBType = "starrocks"
	Postgres  DBType = "postgres"
	SQLServer DBType = "sqlserver"
	SQLite    DBType = "sqlite"
)

// AllDBTypes 按界面展示顺序列出全部受支持的数据库类型。
func AllDBTypes() []DBType {
	return []DBType{MySQL, MariaDB, Postgres, SQLServer, SQLite, TiDB, StarRocks}
}

func (t DBType) String() string { return string(t) }

// Valid 报告 t 是否是受支持的数据库类型。
func (t DBType) Valid() bool {
	for _, v := range AllDBTypes() {
		if v == t {
			return true
		}
	}
	return false
}

// MySQLCompatible 报告 t 是否使用 MySQL 协议与驱动（mysql、mariadb、tidb、starrocks）。
func (t DBType) MySQLCompatible() bool {
	switch t {
	case MySQL, MariaDB, TiDB, StarRocks:
		return true
	}
	return false
}

// SafeMode 是连接的安全级别，对应 TablePlus 的 Safe Mode：
//
//	silent  不提示（默认）
//	alert1  发送任何语句前都要确认
//	alert2  除 SELECT / EXPLAIN / SHOW 外都要确认
//	safe1   发送任何语句前都要输入数据库密码
//	safe2   除 SELECT / EXPLAIN / SHOW 外都要输入数据库密码
type SafeMode string

const (
	SafeModeSilent SafeMode = "silent"
	SafeModeAlert1 SafeMode = "alert1"
	SafeModeAlert2 SafeMode = "alert2"
	SafeModeSafe1  SafeMode = "safe1"
	SafeModeSafe2  SafeMode = "safe2"
)

// SafeModes 按严格程度从低到高列出全部级别。
var SafeModes = []SafeMode{SafeModeSilent, SafeModeAlert1, SafeModeAlert2, SafeModeSafe1, SafeModeSafe2}

// SafeAction 是发送语句前需要用户完成的动作。
type SafeAction int

const (
	SafeNone     SafeAction = iota // 直接发送
	SafeConfirm                    // 弹出确认
	SafePassword                   // 输入数据库密码
)

// Valid 判断是不是已知级别。
func (m SafeMode) Valid() bool {
	for _, v := range SafeModes {
		if v == m {
			return true
		}
	}
	return false
}

// EffectiveSafeMode 返回连接实际生效的级别。
// 没有显式设置时：生产环境标签沿用旧版"提交前二次确认"的行为，按 alert2 处理；其余为 silent。
func (c ConnectionConfig) EffectiveSafeMode() SafeMode {
	if m := SafeMode(c.SafeMode); m.Valid() {
		return m
	}
	if c.Tag == "production" {
		return SafeModeAlert2
	}
	return SafeModeSilent
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

// ForeignKeyInfo 是表上的一条外键约束（可以多列）。
type ForeignKeyInfo struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnUpdate   string   `json:"onUpdate"`
	OnDelete   string   `json:"onDelete"`
}

// TriggerInfo 是表上的一个触发器。Definition 是完整的 CREATE TRIGGER 语句。
type TriggerInfo struct {
	Name       string `json:"name"`
	Timing     string `json:"timing"`
	Event      string `json:"event"`
	Definition string `json:"definition"`
}

// RoutineInfo 是一个函数或存储过程。
type RoutineInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // function / procedure
}

// ColumnMeta 列元数据
type ColumnMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// SSHConfig SSH 隧道配置：启用后通过跳板机转发到 Host:Port，数据库驱动连接本机转发端口。
type SSHConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	AuthType string `json:"authType"` // "password" | "key"
	KeyPath  string `json:"keyPath"`
	// Secret 为 SSH 密码或私钥口令（取决于 AuthType），落库时加密。
	Secret string `json:"secret"`
	// HasSecret 仅用于前端展示：列表接口不回传 Secret。不会被持久化。
	HasSecret bool `json:"hasSecret,omitempty"`
	// ClearSecret 仅在保存时有效：显式清除已保存的 Secret（Secret 为空默认表示"保持不变"）。不会被持久化。
	ClearSecret bool `json:"clearSecret,omitempty"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     DBType `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	// HasPassword 仅用于前端展示：列表接口不回传明文密码，用它表示"已保存过密码"。不会被持久化。
	HasPassword bool `json:"hasPassword,omitempty"`
	// ClearPassword 仅在保存时有效：显式清除已保存的密码（Password 为空默认表示"保持不变"）。不会被持久化。
	ClearPassword bool   `json:"clearPassword,omitempty"`
	Database      string `json:"database"`
	SSLMode       string `json:"sslMode"`
	Color         string `json:"color"`
	// Group 是早期预留的字符串分组，从未使用；分组改用 FolderID，这个字段只为兼容旧备份保留。
	Group string `json:"group"`
	// FolderID 所属文件夹的 ID，空表示「未分组」。
	FolderID string `json:"folderId,omitempty"`
	// Order 在同一文件夹（或未分组）里的顺序，小的在前；0 视为尚未排序，排在有序号的后面。
	Order int    `json:"order,omitempty"`
	Tag   string `json:"tag"` // 环境标签：local/test/production
	// ReadOnly 只读模式：后端拒绝一切写操作（SQL 编辑器、表数据编辑、DDL、清空/删表）。
	ReadOnly bool `json:"readOnly"`
	// AIMetadataOnly 只把库结构（表、列、索引、注释）给 AI：不注册会读取行数据的工具，
	// 表里的内容不会发送到第三方模型服务。
	AIMetadataOnly bool `json:"aiMetadataOnly,omitempty"`
	// SafeMode 安全级别（silent/alert1/alert2/safe1/safe2）；空表示未设置，见 EffectiveSafeMode。
	SafeMode string `json:"safeMode,omitempty"`
	// PasswordMode 密码来源：空或 keychain 为已保存（系统钥匙串）；ask 每次连接时询问；none 不使用密码。
	PasswordMode string `json:"passwordMode,omitempty"`
	// SSH SSH 隧道（仅网络型数据库）。
	SSH SSHConfig `json:"ssh"`
}
