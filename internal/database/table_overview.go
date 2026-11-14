package database

import "context"

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

// GetTableOverview 读出当前库里所有表和视图的概览信息；各数据库能给的字段不同，给不出的留空。
//
// 参数：ctx 用于取消与超时；db 连接；dbType 库类型；dbName 库名。
// 返回值：按名称排好的概览；出错时返回错误。
func GetTableOverview(ctx context.Context, db Executor, dbType DBType, dbName string) ([]TableOverview, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return nil, err
	}
	return d.TableOverview(ctx, db, dbName)
}
