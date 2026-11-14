package database

import "context"

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
