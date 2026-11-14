package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"minidb/internal/model"
)

// MySQL / PostgreSQL 的元数据集成测试，由 CI 里的服务容器提供真实实例。
// 本地运行：
//
//	INTEGRATION_TEST=1 MINIDB_IT_MYSQL_HOST=127.0.0.1 MINIDB_IT_MYSQL_USER=root MINIDB_IT_MYSQL_PASSWORD=pw \
//	  go test ./internal/database/ -run TestIntegrationServices -count=1
//
// 没有设置主机变量的类型会被跳过。

type serviceTarget struct {
	name   string
	dbType DBType
	envKey string // 环境变量前缀
	port   int
	dbName string
	// 建表语句（不同方言的自增、类型写法不同）
	ddl []string
	// createDB 建测试库的语句；空表示库由服务容器建好（PostgreSQL）
	createDB string
}

var serviceTargets = []serviceTarget{
	{
		name: "MySQL", dbType: MySQL, envKey: "MINIDB_IT_MYSQL", port: 3306, dbName: "minidb_it", createDB: "CREATE DATABASE IF NOT EXISTS minidb_it",
		ddl: []string{
			"DROP TABLE IF EXISTS it_orders", "DROP TABLE IF EXISTS it_users",
			"CREATE TABLE it_users (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(64) NOT NULL COMMENT '姓名', email VARCHAR(128) DEFAULT 'a@b.c')",
			"CREATE UNIQUE INDEX idx_it_users_email ON it_users(email)",
			"CREATE TABLE it_orders (id BIGINT PRIMARY KEY, user_id BIGINT, amount DECIMAL(10,2), CONSTRAINT fk_it_orders_user FOREIGN KEY (user_id) REFERENCES it_users(id) ON DELETE CASCADE)",
			"CREATE INDEX idx_it_orders_user_amount ON it_orders(user_id, amount)",
		},
	},
	{
		name: "MariaDB", dbType: MariaDB, envKey: "MINIDB_IT_MARIADB", port: 3306, dbName: "minidb_it", createDB: "CREATE DATABASE IF NOT EXISTS minidb_it",
		ddl: []string{
			"DROP TABLE IF EXISTS it_orders", "DROP TABLE IF EXISTS it_users",
			"CREATE TABLE it_users (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(64) NOT NULL COMMENT '姓名', email VARCHAR(128) DEFAULT 'a@b.c')",
			"CREATE UNIQUE INDEX idx_it_users_email ON it_users(email)",
			"CREATE TABLE it_orders (id BIGINT PRIMARY KEY, user_id BIGINT, amount DECIMAL(10,2), CONSTRAINT fk_it_orders_user FOREIGN KEY (user_id) REFERENCES it_users(id) ON DELETE CASCADE)",
			"CREATE INDEX idx_it_orders_user_amount ON it_orders(user_id, amount)",
		},
	},
	{
		name: "SQL Server", dbType: SQLServer, envKey: "MINIDB_IT_MSSQL", port: 1433, dbName: "minidb_it",
		createDB: "IF DB_ID('minidb_it') IS NULL CREATE DATABASE minidb_it",
		ddl: []string{
			"DROP TABLE IF EXISTS it_orders", "DROP TABLE IF EXISTS it_users",
			"CREATE TABLE it_users (id BIGINT IDENTITY(1,1) PRIMARY KEY, name NVARCHAR(64) NOT NULL, email NVARCHAR(128) DEFAULT 'a@b.c')",
			"CREATE UNIQUE INDEX idx_it_users_email ON it_users(email)",
			"CREATE TABLE it_orders (id BIGINT PRIMARY KEY, user_id BIGINT, amount DECIMAL(10,2), CONSTRAINT fk_it_orders_user FOREIGN KEY (user_id) REFERENCES it_users(id) ON DELETE CASCADE)",
			"CREATE INDEX idx_it_orders_user_amount ON it_orders(user_id, amount)",
		},
	},
	{
		name: "PostgreSQL", dbType: Postgres, envKey: "MINIDB_IT_PG", port: 5432, dbName: "minidb_it",
		ddl: []string{
			"DROP TABLE IF EXISTS it_orders", "DROP TABLE IF EXISTS it_users",
			"CREATE TABLE it_users (id BIGSERIAL PRIMARY KEY, name VARCHAR(64) NOT NULL, email VARCHAR(128) DEFAULT 'a@b.c')",
			"CREATE UNIQUE INDEX idx_it_users_email ON it_users(email)",
			"CREATE TABLE it_orders (id BIGINT PRIMARY KEY, user_id BIGINT, amount NUMERIC(10,2), CONSTRAINT fk_it_orders_user FOREIGN KEY (user_id) REFERENCES it_users(id) ON DELETE CASCADE)",
			"CREATE INDEX idx_it_orders_user_amount ON it_orders(user_id, amount)",
		},
	},
}

func TestIntegrationServices(t *testing.T) {
	skipIfNoIntegration(t)
	for _, tg := range serviceTargets {
		t.Run(tg.name, func(t *testing.T) { runServiceIntegration(t, tg) })
	}
}

func runServiceIntegration(t *testing.T, tg serviceTarget) {
	host := os.Getenv(tg.envKey + "_HOST")
	if host == "" {
		t.Skipf("未设置 %s_HOST，跳过 %s", tg.envKey, tg.name)
	}
	cfg := &ConnectionConfig{
		ID: "it-" + strings.ToLower(tg.name), Name: tg.name, Type: tg.dbType,
		Host: host, Port: integrationPort(tg.envKey+"_PORT", tg.port),
		User: os.Getenv(tg.envKey + "_USER"), Password: os.Getenv(tg.envKey + "_PASSWORD"),
	}
	m := NewManager()
	t.Cleanup(func() { _ = m.Disconnect(cfg.ID) })

	// MySQL 系和 SQL Server 要先建库再连过去；PostgreSQL 的库由服务容器的环境变量创建好。
	if tg.createDB != "" {
		if err := m.Connect(cfg); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		root, err := m.GetDB(cfg.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := root.Exec(tg.createDB); err != nil {
			t.Fatal(err)
		}
		_ = m.Disconnect(cfg.ID)
	}
	cfg.Database = tg.dbName
	if err := m.Connect(cfg); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	db, err := m.GetDB(cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range tg.ddl {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	ctx := context.Background()

	if v, err := GetServerVersion(ctx, db, tg.dbType); err != nil || v == "" {
		t.Errorf("版本 = %q, %v", v, err)
	}

	dbs, err := GetDatabases(ctx, db, tg.dbType)
	if err != nil {
		t.Fatalf("GetDatabases: %v", err)
	}
	var foundDB bool
	for _, d := range dbs {
		foundDB = foundDB || d.Name == tg.dbName || tg.dbType == Postgres
	}
	if !foundDB {
		t.Errorf("库列表里没有 %s: %+v", tg.dbName, dbs)
	}

	tables, err := GetTables(ctx, db, tg.dbType, tg.dbName)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	names := map[string]bool{}
	for _, tb := range tables {
		names[tb.Name] = true
	}
	if !names["it_users"] || !names["it_orders"] {
		t.Errorf("表列表缺少测试表: %+v", tables)
	}

	cols, err := GetColumns(ctx, db, tg.dbType, tg.dbName, "it_users")
	if err != nil || len(cols) != 3 {
		t.Fatalf("GetColumns = %+v, %v", cols, err)
	}
	byName := map[string]ColumnInfo{}
	for _, c := range cols {
		byName[c.Name] = c
	}
	if !byName["id"].IsPrimary || byName["name"].Nullable || !byName["email"].Nullable {
		t.Errorf("主键/可空信息不对: %+v", byName)
	}
	ocols, err := GetColumns(ctx, db, tg.dbType, tg.dbName, "it_orders")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ocols {
		if c.Name == "user_id" && !strings.Contains(c.ForeignKey, "it_users") {
			t.Errorf("user_id 外键 = %q", c.ForeignKey)
		}
	}

	idx, err := GetIndexes(ctx, db, tg.dbType, tg.dbName, "it_orders")
	if err != nil {
		t.Fatalf("GetIndexes: %v", err)
	}
	var foundIdx bool
	for _, ix := range idx {
		if ix.Name == "idx_it_orders_user_amount" {
			foundIdx = true
			if ix.IsUnique || len(ix.Columns) != 2 || ix.Columns[0] != "user_id" {
				t.Errorf("索引 = %+v", ix)
			}
		}
	}
	if !foundIdx {
		t.Errorf("没有读到联合索引: %+v", idx)
	}

	fks, err := GetForeignKeys(ctx, db, tg.dbType, tg.dbName, "it_orders")
	if err != nil {
		t.Fatalf("GetForeignKeys: %v", err)
	}
	if len(fks) != 1 || fks[0].RefTable != "it_users" || !strings.EqualFold(fks[0].OnDelete, "CASCADE") {
		t.Errorf("外键 = %+v", fks)
	}

	ddl, err := GetDDL(ctx, db, tg.dbType, tg.dbName, "it_users")
	if err != nil || !strings.Contains(strings.ToLower(ddl), "create table") {
		t.Errorf("DDL = %q, %v", ddl, err)
	}
	if _, err := GetDDL(ctx, db, tg.dbType, tg.dbName, "no_such_table"); err == nil {
		t.Error("不存在的表应返回错误")
	}

	// 监控：两次采样要有连接数；慢查询数据源没开时必须是带开启办法的 CodeMonitorUnavailable，而不是别的错误
	if CapsOf(tg.dbType).Monitor {
		s1, err := GetMonitorSample(ctx, db, tg.dbType)
		if err != nil {
			t.Fatalf("监控采样: %v", err)
		}
		for i := 0; i < 5; i++ {
			_, _ = db.ExecContext(ctx, "SELECT COUNT(*) FROM it_users")
		}
		time.Sleep(1100 * time.Millisecond)
		s2, err := GetMonitorSample(ctx, db, tg.dbType)
		if err != nil {
			t.Fatalf("第二次监控采样: %v", err)
		}
		pt := MonitorPointOf(&s1, s2)
		if pt.Values[MetricConnections] < 1 {
			t.Errorf("连接数应不小于 1: %+v", pt.Values)
		}
		if q, ok := pt.Values[MetricQueries]; !ok || q < 0 {
			t.Errorf("语句或事务速率不对: %+v", pt.Values)
		}
		if slow, err := GetSlowQueries(ctx, db, tg.dbType, 5); err != nil && model.CodeOf(err) != model.CodeMonitorUnavailable {
			t.Errorf("慢查询应返回数据或「不可用」提示，实际: %v", err)
		} else {
			t.Logf("%s 慢查询: %d 条, err=%v", tg.name, len(slow), err)
		}
	}

	// 执行计划：普通与 ANALYZE 都要解析成树，并且能看到对 it_orders 的访问
	for _, analyze := range []bool{false, true} {
		if !CapsOf(tg.dbType).Explain || analyze && !CapsOf(tg.dbType).ExplainAnalyze {
			continue
		}
		res, err := ExplainQuery(ctx, db, tg.dbType, "SELECT * FROM it_orders WHERE user_id = 1", analyze)
		if err != nil {
			if analyze && tg.dbType == MySQL {
				t.Logf("MySQL 版本不支持 EXPLAIN ANALYZE，跳过: %v", err)
				continue
			}
			t.Errorf("ExplainQuery(analyze=%v): %v", analyze, err)
			continue
		}
		if len(res.Roots) == 0 {
			t.Errorf("计划没有解析成树(analyze=%v): note=%q raw=%q", analyze, res.Note, res.Raw)
			continue
		}
		var seen bool
		for _, pr := range res.Flat() {
			seen = seen || strings.Contains(pr.Node.Object, "it_orders")
		}
		if !seen {
			t.Errorf("计划里没有 it_orders (analyze=%v): %s", analyze, res.Raw)
		}
		if analyze && !res.Analyzed {
			t.Error("analyze 结果应标记 Analyzed")
		}
	}
}
