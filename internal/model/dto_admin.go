package model

import (
	"fmt"
	"strings"
)

// UserInfo 是服务器上的一个账号。MySQL 的账号是 用户名@主机；PostgreSQL 的是角色，没有主机。
type UserInfo struct {
	Name string `json:"name"`
	Host string `json:"host"`
	// 以下只有 PostgreSQL 填：
	CanLogin   bool `json:"canLogin"`
	Superuser  bool `json:"superuser"`
	CreateDB   bool `json:"createDb"`
	CreateRole bool `json:"createRole"`
}

// UserLimits 是 MySQL 的账号资源限制，0 表示不限制；nil 表示不改。
type UserLimits struct {
	MaxQueries     *int // 每小时查询数
	MaxUpdates     *int // 每小时更新数
	MaxConnections *int // 每小时连接数
	MaxUserConns   *int // 同时连接数
}

func (l UserLimits) Clauses() []string {
	var out []string
	add := func(name string, v *int) {
		if v != nil {
			out = append(out, fmt.Sprintf("%s %d", name, *v))
		}
	}
	add("MAX_QUERIES_PER_HOUR", l.MaxQueries)
	add("MAX_UPDATES_PER_HOUR", l.MaxUpdates)
	add("MAX_CONNECTIONS_PER_HOUR", l.MaxConnections)
	add("MAX_USER_CONNECTIONS", l.MaxUserConns)
	return out
}

// PrivScope 是授权的范围。MySQL 是 *.*、库.*、库.表；PostgreSQL 是库、模式、表。
type PrivScope struct {
	DB     string // 空为全局（MySQL）
	Schema string // 仅 PostgreSQL
	Table  string // 空为整个库（MySQL）或整个模式里的所有表（PostgreSQL）
}

// PGRoleAttrs 是 PostgreSQL 角色的属性；nil 表示不改。
type PGRoleAttrs struct {
	Login      *bool
	Superuser  *bool
	CreateDB   *bool
	CreateRole *bool
}

func (a PGRoleAttrs) Words() []string {
	var out []string
	add := func(on, off string, v *bool) {
		if v == nil {
			return
		}
		if *v {
			out = append(out, on)
		} else {
			out = append(out, off)
		}
	}
	add("LOGIN", "NOLOGIN", a.Login)
	add("SUPERUSER", "NOSUPERUSER", a.Superuser)
	add("CREATEDB", "NOCREATEDB", a.CreateDB)
	add("CREATEROLE", "NOCREATEROLE", a.CreateRole)
	return out
}

// UserSpec 是创建或修改账号要用的信息。
type UserSpec struct {
	Name     string
	Host     string // 仅 MySQL
	Password string // 空表示不设置（创建时为无密码）
	Limits   UserLimits
	Attrs    PGRoleAttrs
}

func (u UserSpec) Check() error {
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("用户名为空")
	}
	return nil
}

// ProcessInfo 是一条服务端会话/进程信息（MySQL PROCESSLIST 或 PostgreSQL pg_stat_activity）。
type ProcessInfo struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Host     string `json:"host"`
	Database string `json:"database"`
	Command  string `json:"command"`
	TimeSec  int64  `json:"timeSec"`
	State    string `json:"state"`
	Info     string `json:"info"` // 正在执行的 SQL（已截断）
}

// TableOpOptions 是清空 / 删除表时的可选项。
//
// @author ixxxxoooo
type TableOpOptions struct {
	Cascade         bool `json:"cascade"`         // 连带清空 / 删除引用它的表（PostgreSQL）
	RestartIdentity bool `json:"restartIdentity"` // 清空后自增序列从头开始（PostgreSQL）
}

// TableOverview 是「项目概览」页里一张表（或视图）的一行：名称、类型、引擎、行数、大小、排序规则、备注、时间。
// 数字取不到时是 -1，文字取不到时是空串，界面上都显示成「—」。
//
// @author ixxxxoooo
type TableOverview struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // table / view
	Engine    string `json:"engine"`
	Rows      int64  `json:"rows"`      // MySQL、PostgreSQL 是估算值，SQLite 是精确值
	DataSize  int64  `json:"dataSize"`  // 字节
	IndexSize int64  `json:"indexSize"` // 字节
	Collation string `json:"collation"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ImportColumn 一列的映射：源行下标 → 目标列名。
type ImportColumn struct {
	Source int    `json:"source"`
	Target string `json:"target"`
}

// ImportOptions 导入选项
type ImportOptions struct {
	// Atomic 为 true 时整个导入在一个事务里，任何错误或取消都会全部回滚；
	// 为 false 时每批各自提交，失败时已提交的批次保留（适合超大文件，避免超出事务大小限制）。
	Atomic bool `json:"atomic"`
	// EmptyAsNull 把空字符串写成 NULL（CSV 中空单元格通常表示缺失）。
	EmptyAsNull bool `json:"emptyAsNull"`
	// BatchSize 每条多行 INSERT 的行数，<=0 取默认值。
	BatchSize int `json:"batchSize"`
	// Conflict 遇到重复键怎么办：空为报错停止，见 Conflict* 常量。
	Conflict string `json:"conflict"`
	// Keys 是更新模式用来判断重复的列（主键或唯一键）。MySQL 系自己认键，可以不给；
	// PostgreSQL 和 SQLite 必须给。
	Keys []string `json:"keys"`
}
