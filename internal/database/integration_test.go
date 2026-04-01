package database

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// 集成测试需要设置环境变量 INTEGRATION_TEST=1 才会执行
// 运行方式: INTEGRATION_TEST=1 go test ./internal/database/ -v -run TestIntegration -count=1

func skipIfNoIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("跳过集成测试（设置 INTEGRATION_TEST=1 开启）")
	}
}

// ========== TiDB 集成测试 ==========

const (
	tidbHost     = "10.116.48.70"
	tidbPort     = 8400
	tidbUser     = "zztest_app"
	tidbPassword = "zztest_app@123xxx&"
)

func connectTiDB(t *testing.T) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		tidbUser, tidbPassword, tidbHost, tidbPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("TiDB 连接失败: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("TiDB Ping 失败: %v", err)
	}
	t.Log("TiDB 连接成功")
	return db
}

// TestIntegrationTiDBConnect 测试 TiDB 连接
func TestIntegrationTiDBConnect(t *testing.T) {
	skipIfNoIntegration(t)

	m := NewManager()
	cfg := &ConnectionConfig{
		ID:       "test-tidb",
		Name:     "测试 TiDB",
		Type:     "tidb",
		Host:     tidbHost,
		Port:     tidbPort,
		User:     tidbUser,
		Password: tidbPassword,
	}

	// 测试连接
	err := m.TestConnection(cfg)
	if err != nil {
		t.Fatalf("TiDB TestConnection 失败: %v", err)
	}
	t.Log("TiDB TestConnection 成功")

	// 建立连接
	err = m.Connect(cfg)
	if err != nil {
		t.Fatalf("TiDB Connect 失败: %v", err)
	}
	t.Log("TiDB Connect 成功")

	// 验证连接存在
	db, err := m.GetDB("test-tidb")
	if err != nil {
		t.Fatalf("TiDB GetDB 失败: %v", err)
	}
	if db == nil {
		t.Fatal("TiDB GetDB 返回 nil")
	}

	// 断开连接
	err = m.Disconnect("test-tidb")
	if err != nil {
		t.Fatalf("TiDB Disconnect 失败: %v", err)
	}
	t.Log("TiDB Disconnect 成功")
}

// TestIntegrationTiDBDatabases 测试 TiDB 列出数据库
func TestIntegrationTiDBDatabases(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	// 测试 GetDatabases
	dbs, err := GetDatabases(db, "tidb")
	if err != nil {
		t.Fatalf("TiDB GetDatabases 失败: %v", err)
	}
	t.Logf("TiDB 数据库列表 (%d 个):", len(dbs))
	for _, d := range dbs {
		t.Logf("  - %s (表数: %d)", d.Name, d.TableCount)
	}
	if len(dbs) == 0 {
		t.Error("TiDB 数据库列表为空")
	}

	// 测试 GetDatabaseNames
	dbNames, err := GetDatabaseNames(db, "tidb")
	if err != nil {
		t.Fatalf("TiDB GetDatabaseNames 失败: %v", err)
	}
	t.Logf("TiDB 数据库名称列表 (%d 个)", len(dbNames))
	if len(dbNames) == 0 {
		t.Error("TiDB 数据库名称列表为空")
	}
}

// TestIntegrationTiDBTables 测试 TiDB 列出表
func TestIntegrationTiDBTables(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	// 获取第一个可用数据库
	dbs, err := GetDatabaseNames(db, "tidb")
	if err != nil || len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库")
	}

	dbName := dbs[0].Name
	t.Logf("使用数据库: %s", dbName)
	db.Exec("USE " + dbName)

	tables, err := GetTables(db, "tidb", dbName)
	if err != nil {
		t.Fatalf("TiDB GetTables 失败: %v", err)
	}
	t.Logf("TiDB 表列表 (%d 张):", len(tables))
	for i, tbl := range tables {
		if i < 10 {
			t.Logf("  - %s (type=%s, rows=%d, comment=%s)", tbl.Name, tbl.Type, tbl.RowCount, tbl.Comment)
		}
	}
}

// TestIntegrationTiDBColumns 测试 TiDB 列信息
func TestIntegrationTiDBColumns(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	dbs, _ := GetDatabaseNames(db, "tidb")
	if len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库")
	}
	dbName := dbs[0].Name
	db.Exec("USE " + dbName)

	tables, _ := GetTables(db, "tidb", dbName)
	if len(tables) == 0 {
		t.Skip("TiDB 无可用表")
	}

	tableName := tables[0].Name
	t.Logf("查询表列信息: %s.%s", dbName, tableName)

	cols, err := GetColumns(db, "tidb", dbName, tableName)
	if err != nil {
		t.Fatalf("TiDB GetColumns 失败: %v", err)
	}
	t.Logf("列数: %d", len(cols))
	for _, c := range cols {
		t.Logf("  - %s %s nullable=%v primary=%v comment=%s", c.Name, c.Type, c.Nullable, c.IsPrimary, c.Comment)
	}
	if len(cols) == 0 {
		t.Error("TiDB 列列表为空")
	}
}

// TestIntegrationTiDBDDL 测试 TiDB DDL
func TestIntegrationTiDBDDL(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	dbs, _ := GetDatabaseNames(db, "tidb")
	if len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库")
	}
	dbName := dbs[0].Name
	db.Exec("USE " + dbName)

	tables, _ := GetTables(db, "tidb", dbName)
	if len(tables) == 0 {
		t.Skip("TiDB 无可用表")
	}

	tableName := tables[0].Name
	ddl, err := GetDDL(db, "tidb", dbName, tableName)
	if err != nil {
		t.Fatalf("TiDB GetDDL 失败: %v", err)
	}
	t.Logf("TiDB DDL (%s):\n%s", tableName, ddl)
	if ddl == "" {
		t.Error("TiDB DDL 为空")
	}
}

// TestIntegrationTiDBIndexes 测试 TiDB 索引
func TestIntegrationTiDBIndexes(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	dbs, _ := GetDatabaseNames(db, "tidb")
	if len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库")
	}
	dbName := dbs[0].Name
	db.Exec("USE " + dbName)

	tables, _ := GetTables(db, "tidb", dbName)
	if len(tables) == 0 {
		t.Skip("TiDB 无可用表")
	}

	tableName := tables[0].Name
	indexes, err := GetIndexes(db, "tidb", dbName, tableName)
	if err != nil {
		t.Fatalf("TiDB GetIndexes 失败: %v", err)
	}
	t.Logf("TiDB 索引 (%s): %d 个", tableName, len(indexes))
	for _, idx := range indexes {
		t.Logf("  - %s columns=%v unique=%v primary=%v type=%s", idx.Name, idx.Columns, idx.IsUnique, idx.IsPrimary, idx.Type)
	}
}

// TestIntegrationTiDBTableStats 测试 TiDB 表统计
func TestIntegrationTiDBTableStats(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	dbs, _ := GetDatabaseNames(db, "tidb")
	if len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库")
	}
	dbName := dbs[0].Name
	db.Exec("USE " + dbName)

	tables, _ := GetTables(db, "tidb", dbName)
	if len(tables) == 0 {
		t.Skip("TiDB 无可用表")
	}

	tableName := tables[0].Name
	stats, err := GetTableStats(db, "tidb", dbName, tableName)
	if err != nil {
		t.Fatalf("TiDB GetTableStats 失败: %v", err)
	}
	t.Logf("TiDB 表统计 (%s): rows=%d dataSize=%d indexSize=%d engine=%s",
		tableName, stats.RowCount, stats.DataSize, stats.IndexSize, stats.Engine)
}

// TestIntegrationTiDBQuery 测试 TiDB 查询执行
func TestIntegrationTiDBQuery(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectTiDB(t)
	defer db.Close()

	// 测试简单查询
	result, err := ExecuteQuery(db, "SELECT 1 AS val, 'hello' AS msg")
	if err != nil {
		t.Fatalf("TiDB ExecuteQuery 失败: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("TiDB 查询错误: %s", result.Error)
	}
	t.Logf("TiDB 查询结果: 列数=%d 行数=%d 耗时=%dms", len(result.Columns), len(result.Rows), result.Duration)
	if len(result.Rows) != 1 {
		t.Errorf("期望 1 行结果，得到 %d 行", len(result.Rows))
	}

	// 测试分页查询
	dbs, _ := GetDatabaseNames(db, "tidb")
	if len(dbs) == 0 {
		t.Skip("TiDB 无可用数据库用于分页测试")
	}
	dbName := dbs[0].Name
	db.Exec("USE " + dbName)

	tables, _ := GetTables(db, "tidb", dbName)
	if len(tables) == 0 {
		t.Skip("TiDB 无可用表用于分页测试")
	}

	tableName := tables[0].Name
	pageResult, err := QueryTableData(db, "tidb", dbName, tableName, 1, 5, nil, nil)
	if err != nil {
		t.Fatalf("TiDB QueryTableData 失败: %v", err)
	}
	if pageResult.Error != "" {
		t.Fatalf("TiDB 分页查询错误: %s", pageResult.Error)
	}
	t.Logf("TiDB 分页查询 (%s): total=%d 返回行数=%d 列数=%d",
		tableName, pageResult.Total, len(pageResult.Rows), len(pageResult.Columns))
}

// ========== StarRocks 集成测试 ==========

const (
	srHost     = "10.116.32.105"
	srPort     = 8031
	srUser     = "root"
	srPassword = "zyb_DATAWARE_2024"
)

func connectStarRocks(t *testing.T) *sql.DB {
	// StarRocks 不支持 COM_STMT_PREPARE，必须启用 interpolateParams
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local&interpolateParams=true",
		srUser, srPassword, srHost, srPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("StarRocks 连接失败: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("StarRocks Ping 失败: %v", err)
	}
	t.Log("StarRocks 连接成功")
	return db
}

// TestIntegrationStarRocksConnect 测试 StarRocks 连接
func TestIntegrationStarRocksConnect(t *testing.T) {
	skipIfNoIntegration(t)

	m := NewManager()
	cfg := &ConnectionConfig{
		ID:       "test-sr",
		Name:     "测试 StarRocks",
		Type:     "starrocks",
		Host:     srHost,
		Port:     srPort,
		User:     srUser,
		Password: srPassword,
	}

	// 测试连接
	err := m.TestConnection(cfg)
	if err != nil {
		t.Fatalf("StarRocks TestConnection 失败: %v", err)
	}
	t.Log("StarRocks TestConnection 成功")

	// 建立连接
	err = m.Connect(cfg)
	if err != nil {
		t.Fatalf("StarRocks Connect 失败: %v", err)
	}
	t.Log("StarRocks Connect 成功")

	db, err := m.GetDB("test-sr")
	if err != nil {
		t.Fatalf("StarRocks GetDB 失败: %v", err)
	}
	if db == nil {
		t.Fatal("StarRocks GetDB 返回 nil")
	}

	err = m.Disconnect("test-sr")
	if err != nil {
		t.Fatalf("StarRocks Disconnect 失败: %v", err)
	}
	t.Log("StarRocks Disconnect 成功")
}

// TestIntegrationStarRocksDatabases 测试 StarRocks 列出数据库
func TestIntegrationStarRocksDatabases(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbs, err := GetDatabases(db, "starrocks")
	if err != nil {
		t.Fatalf("StarRocks GetDatabases 失败: %v", err)
	}
	t.Logf("StarRocks 数据库列表 (%d 个):", len(dbs))
	for _, d := range dbs {
		t.Logf("  - %s (表数: %d)", d.Name, d.TableCount)
	}
	if len(dbs) == 0 {
		t.Error("StarRocks 数据库列表为空")
	}

	dbNames, err := GetDatabaseNames(db, "starrocks")
	if err != nil {
		t.Fatalf("StarRocks GetDatabaseNames 失败: %v", err)
	}
	t.Logf("StarRocks 数据库名称列表 (%d 个)", len(dbNames))
	if len(dbNames) == 0 {
		t.Error("StarRocks 数据库名称列表为空")
	}

	// 验证系统库已被过滤
	for _, d := range dbNames {
		if d.Name == "_statistics_" || d.Name == "starrocks_monitor" {
			t.Errorf("StarRocks 系统库未被过滤: %s", d.Name)
		}
	}
}

// findStarRocksDBWithTables 找到一个有表的 StarRocks 数据库
func findStarRocksDBWithTables(t *testing.T, db *sql.DB) (string, []TableInfo) {
	dbs, err := GetDatabases(db, "starrocks")
	if err != nil || len(dbs) == 0 {
		return "", nil
	}
	for _, d := range dbs {
		if d.TableCount > 0 {
			db.Exec("USE " + d.Name)
			tables, err := GetTables(db, "starrocks", d.Name)
			if err == nil && len(tables) > 0 {
				return d.Name, tables
			}
		}
	}
	return "", nil
}

// TestIntegrationStarRocksTables 测试 StarRocks 列出表
func TestIntegrationStarRocksTables(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" {
		t.Skip("StarRocks 无含表的数据库")
	}

	t.Logf("使用数据库: %s", dbName)
	t.Logf("StarRocks 表列表 (%d 张):", len(tables))
	for i, tbl := range tables {
		if i < 10 {
			t.Logf("  - %s (type=%s, rows=%d)", tbl.Name, tbl.Type, tbl.RowCount)
		}
	}
}

// TestIntegrationStarRocksColumns 测试 StarRocks 列信息
func TestIntegrationStarRocksColumns(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" || len(tables) == 0 {
		t.Skip("StarRocks 无含表的数据库")
	}

	tableName := tables[0].Name
	t.Logf("查询表列信息: %s.%s", dbName, tableName)

	cols, err := GetColumns(db, "starrocks", dbName, tableName)
	if err != nil {
		t.Fatalf("StarRocks GetColumns 失败: %v", err)
	}
	t.Logf("列数: %d", len(cols))
	for _, c := range cols {
		t.Logf("  - %s %s nullable=%v primary=%v", c.Name, c.Type, c.Nullable, c.IsPrimary)
	}
	if len(cols) == 0 {
		t.Error("StarRocks 列列表为空")
	}
}

// TestIntegrationStarRocksDDL 测试 StarRocks DDL
func TestIntegrationStarRocksDDL(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" || len(tables) == 0 {
		t.Skip("StarRocks 无含表的数据库")
	}

	tableName := tables[0].Name
	ddl, err := GetDDL(db, "starrocks", dbName, tableName)
	if err != nil {
		t.Fatalf("StarRocks GetDDL 失败: %v", err)
	}
	t.Logf("StarRocks DDL (%s):\n%s", tableName, ddl)
	if ddl == "" {
		t.Error("StarRocks DDL 为空")
	}
}

// TestIntegrationStarRocksIndexes 测试 StarRocks 索引
func TestIntegrationStarRocksIndexes(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" || len(tables) == 0 {
		t.Skip("StarRocks 无含表的数据库")
	}

	tableName := tables[0].Name
	indexes, err := GetIndexes(db, "starrocks", dbName, tableName)
	if err != nil {
		t.Fatalf("StarRocks GetIndexes 失败: %v", err)
	}
	t.Logf("StarRocks 索引 (%s): %d 个", tableName, len(indexes))
	for _, idx := range indexes {
		t.Logf("  - %s columns=%v unique=%v primary=%v type=%s", idx.Name, idx.Columns, idx.IsUnique, idx.IsPrimary, idx.Type)
	}
}

// TestIntegrationStarRocksTableStats 测试 StarRocks 表统计
func TestIntegrationStarRocksTableStats(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" || len(tables) == 0 {
		t.Skip("StarRocks 无含表的数据库")
	}

	tableName := tables[0].Name
	stats, err := GetTableStats(db, "starrocks", dbName, tableName)
	if err != nil {
		t.Fatalf("StarRocks GetTableStats 失败: %v", err)
	}
	t.Logf("StarRocks 表统计 (%s): rows=%d dataSize=%d indexSize=%d engine=%s",
		tableName, stats.RowCount, stats.DataSize, stats.IndexSize, stats.Engine)
}

// TestIntegrationStarRocksQuery 测试 StarRocks 查询执行
func TestIntegrationStarRocksQuery(t *testing.T) {
	skipIfNoIntegration(t)
	db := connectStarRocks(t)
	defer db.Close()

	// 测试简单查询
	result, err := ExecuteQuery(db, "SELECT 1 AS val, 'hello' AS msg")
	if err != nil {
		t.Fatalf("StarRocks ExecuteQuery 失败: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("StarRocks 查询错误: %s", result.Error)
	}
	t.Logf("StarRocks 查询结果: 列数=%d 行数=%d 耗时=%dms", len(result.Columns), len(result.Rows), result.Duration)
	if len(result.Rows) != 1 {
		t.Errorf("期望 1 行结果，得到 %d 行", len(result.Rows))
	}

	// 测试分页查询
	dbName, tables := findStarRocksDBWithTables(t, db)
	if dbName == "" || len(tables) == 0 {
		t.Skip("StarRocks 无含表的数据库用于分页测试")
	}

	tableName := tables[0].Name
	pageResult, err := QueryTableData(db, "starrocks", dbName, tableName, 1, 5, nil, nil)
	if err != nil {
		t.Fatalf("StarRocks QueryTableData 失败: %v", err)
	}
	if pageResult.Error != "" {
		t.Fatalf("StarRocks 分页查询错误: %s", pageResult.Error)
	}
	t.Logf("StarRocks 分页查询 (%s): total=%d 返回行数=%d 列数=%d",
		tableName, pageResult.Total, len(pageResult.Rows), len(pageResult.Columns))
}
