package database

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// ClickHouse 的集成测试：它没有外键、事务，建表要带引擎，所以不放进通用的 serviceTargets。
// 本地运行：
//
//	INTEGRATION_TEST=1 MINIDB_IT_CLICKHOUSE_HOST=127.0.0.1 go test ./internal/database/ -run TestIntegrationClickHouse -count=1
func TestIntegrationClickHouse(t *testing.T) {
	skipIfNoIntegration(t)
	host := os.Getenv("MINIDB_IT_CLICKHOUSE_HOST")
	if host == "" {
		t.Skip("未设置 MINIDB_IT_CLICKHOUSE_HOST，跳过 ClickHouse")
	}
	cfg := &ConnectionConfig{
		ID: "it-clickhouse", Name: "ClickHouse", Type: ClickHouse,
		Host: host, Port: integrationPort("MINIDB_IT_CLICKHOUSE_PORT", 9000),
		User: envOr("MINIDB_IT_CLICKHOUSE_USER", "default"), Password: os.Getenv("MINIDB_IT_CLICKHOUSE_PASSWORD"),
	}
	m := NewManager()
	t.Cleanup(func() { _ = m.Disconnect(cfg.ID) })
	if err := m.Connect(cfg); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	root, err := m.GetDB(cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Exec("CREATE DATABASE IF NOT EXISTS minidb_it"); err != nil {
		t.Fatal(err)
	}
	_ = m.Disconnect(cfg.ID)
	cfg.Database = "minidb_it"
	if err := m.Connect(cfg); err != nil {
		t.Fatalf("连接 minidb_it 失败: %v", err)
	}
	db, err := m.GetDB(cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"DROP TABLE IF EXISTS it_view", "DROP TABLE IF EXISTS it_events", "DROP TABLE IF EXISTS it_import",
		`CREATE TABLE it_events (
			id UInt64, name String, note Nullable(String), score Float32, price Decimal(10, 2), flag Bool,
			tags Array(String), attrs Map(String, String), uid UUID, day Date, ts DateTime,
			kind String DEFAULT 'x' COMMENT '类别',
			INDEX idx_name name TYPE bloom_filter GRANULARITY 3
		) ENGINE = MergeTree ORDER BY (id, day) COMMENT '事件'`,
		"CREATE VIEW it_view AS SELECT id, name FROM it_events",
		`INSERT INTO it_events (id, name, note, score, price, flag, tags, attrs, uid, day, ts) VALUES
			(1, 'a', NULL, 0.1, 12.50, true, ['x','y'], {'k':'v'}, '61f0c404-5cb3-11e7-907b-a6006ad3dba0', '2024-05-06', '2024-05-06 07:08:09'),
			(18446744073709551615, 'b', 'n', 2.5, 0, false, [], {}, '00000000-0000-0000-0000-000000000000', '2024-01-01', '2024-01-01 00:00:00')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	ctx := context.Background()

	if v, err := GetServerVersion(ctx, db, ClickHouse); err != nil || v == "" {
		t.Errorf("版本 = %q, %v", v, err)
	}
	dbs, err := GetDatabases(ctx, db, ClickHouse)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, d := range dbs {
		if d.Name == "system" {
			t.Errorf("库列表不该含 system")
		}
		found = found || d.Name == "minidb_it"
	}
	if !found {
		t.Errorf("库列表里没有 minidb_it: %+v", dbs)
	}

	tables, err := GetTables(ctx, db, ClickHouse, "minidb_it")
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, tb := range tables {
		types[tb.Name] = tb.Type
	}
	if types["it_events"] != "table" || types["it_view"] != "view" {
		t.Errorf("表/视图类型不对: %+v", tables)
	}

	cols, err := GetColumns(ctx, db, ClickHouse, "minidb_it", "it_events")
	if err != nil || len(cols) != 12 {
		t.Fatalf("GetColumns = %d 列, %v", len(cols), err)
	}
	by := map[string]ColumnInfo{}
	for _, c := range cols {
		by[c.Name] = c
	}
	if !by["note"].Nullable || by["name"].Nullable || !by["id"].IsPrimary || by["score"].IsPrimary {
		t.Errorf("可空/主键不对: %+v", by)
	}
	if d := by["kind"].DefaultValue; d == nil || *d != "'x'" || by["kind"].Comment != "类别" {
		t.Errorf("默认值/备注不对: %+v", by["kind"])
	}

	idx, err := GetIndexes(ctx, db, ClickHouse, "minidb_it", "it_events")
	if err != nil {
		t.Fatal(err)
	}
	var primary, skip bool
	for _, ix := range idx {
		primary = primary || ix.IsPrimary && len(ix.Columns) == 2 && ix.Columns[0] == "id"
		skip = skip || ix.Name == "idx_name" && strings.Contains(ix.Type, "bloom")
	}
	if !primary || !skip {
		t.Errorf("索引不对: %+v", idx)
	}

	ddl, err := GetDDL(ctx, db, ClickHouse, "minidb_it", "it_events")
	if err != nil || !strings.Contains(ddl, "MergeTree") {
		t.Errorf("DDL = %q, %v", ddl, err)
	}
	if _, err := GetDDL(ctx, db, ClickHouse, "minidb_it", "no_such_table"); err == nil {
		t.Error("不存在的表应返回错误")
	}
	if st, err := GetTableStats(ctx, db, ClickHouse, "minidb_it", "it_events"); err != nil || st.Engine != "MergeTree" || st.RowCount != 2 {
		t.Errorf("统计 = %+v, %v", st, err)
	}
	ov, err := GetTableOverview(ctx, db, ClickHouse, "minidb_it")
	if err != nil || len(ov) != 2 {
		t.Fatalf("概览 = %+v, %v", ov, err)
	}

	// 各种列类型都要能扫成前端可用的值
	res, err := ExecuteQueryPagedFor(ctx, ClickHouse, db, "SELECT * FROM it_events ORDER BY id", 1, 100, true)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("行数 = %d", len(res.Rows))
	}
	r := res.Rows[0]
	t.Logf("第一行: %#v", r)
	for _, k := range []string{"id", "name", "score", "price", "flag", "tags", "attrs", "uid", "day", "ts"} {
		if _, ok := r[k]; !ok {
			t.Errorf("缺少列 %s", k)
		}
	}
	if r["note"] != nil {
		t.Errorf("NULL 应为 nil: %#v", r["note"])
	}
	if r["score"] != 0.1 || r["day"] != "2024-05-06" || r["ts"] != "2024-05-06 07:08:09" || r["uid"] != "61f0c404-5cb3-11e7-907b-a6006ad3dba0" {
		t.Errorf("标量值不对: %#v", r)
	}
	if fmt.Sprint(r["tags"]) != `["x","y"]` || fmt.Sprint(r["attrs"]) != `{"k":"v"}` || fmt.Sprint(r["price"]) != "12.5" && fmt.Sprint(r["price"]) != "12.50" {
		t.Errorf("复合值不对: %#v", r)
	}
	if res.Rows[1]["id"] != "18446744073709551615" {
		t.Errorf("大整数应转成文本: %#v", res.Rows[1]["id"])
	}
	if res.Rows[1]["note"] != "n" {
		t.Errorf("可空列的值: %#v", res.Rows[1]["note"])
	}

	// 分页与计数
	page, err := ExecuteQueryPagedFor(ctx, ClickHouse, db, "SELECT id FROM it_events ORDER BY id", 2, 1, true)
	if err != nil || len(page.Rows) != 1 || page.Rows[0]["id"] != "18446744073709551615" {
		t.Errorf("第二页 = %+v, %v", page, err)
	}
	// 表数据页：筛选（含跨列搜索，走 toString）、排序、计数
	td, err := QueryTableData(ctx, db, ClickHouse, "minidb_it", "it_events", 1, 50,
		[]Filter{{Column: "name", Operator: "=", Value: "b"}}, []Sort{{Column: "id", Direction: "DESC"}})
	if err != nil || len(td.Rows) != 1 || td.Total != 1 {
		t.Errorf("表数据 = %+v, %v", td, err)
	}
	if n, err := CountTableData(ctx, db, ClickHouse, "minidb_it", "it_events", []Filter{{Column: "name", Operator: "LIKE", Value: "%a%"}}); err != nil || n != 1 {
		t.Errorf("计数 = %d, %v", n, err)
	}
	if n, err := CountTableData(ctx, db, ClickHouse, "minidb_it", "it_events", []Filter{{Column: "__any", Operator: "LIKE", Value: "2024"}}); err != nil || n != 2 {
		t.Errorf("跨列搜索计数 = %d, %v", n, err)
	}
	// 带参数的查询（占位符由驱动在客户端绑定）
	if q, err := db.QueryContext(ctx, "SELECT count() FROM it_events WHERE name = ? OR note LIKE ?", "a", "%"); err != nil {
		t.Errorf("带参查询: %v", err)
	} else {
		q.Close()
	}

	// 导入：建表带引擎，写入走多行 VALUES，ClickHouse 没有事务所以不会声称可回滚
	create, err := BuildCreateImportTableSQL(ClickHouse, "minidb_it", "it_import", []NewTableColumn{{Name: "a", Kind: "int"}, {Name: "b", Kind: "float"}, {Name: "c", Kind: "text"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(create); err != nil {
		t.Fatalf("%s: %v", create, err)
	}
	sess, err := OpenSession(ctx, db, ClickHouse, "minidb_it")
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	n := 0
	next := func() ([]interface{}, error) {
		n++
		if n > 250 {
			return nil, io.EOF
		}
		return []interface{}{int64(n), float64(n) / 4, fmt.Sprintf("it's \\ %d", n)}, nil
	}
	ir, err := ImportRows(ctx, sess, ClickHouse, "minidb_it", "it_import",
		[]ImportColumn{{Source: 0, Target: "a"}, {Source: 1, Target: "b"}, {Source: 2, Target: "c"}}, next,
		ImportOptions{Atomic: true, BatchSize: 100}, nil)
	if err != nil || ir.Inserted != 250 || ir.RolledBack {
		t.Fatalf("导入 = %+v, %v", ir, err)
	}
	var cnt int64
	var sample string
	if err := db.QueryRowContext(ctx, "SELECT count(), anyIf(c, a = 7) FROM it_import").Scan(&cnt, &sample); err != nil || cnt != 250 || sample != `it's \ 7` {
		t.Errorf("导入后 count=%d sample=%q err=%v", cnt, sample, err)
	}

	// 进程列表与终止查询
	done := make(chan error, 1)
	go func() {
		_, err := db.ExecContext(ctx, "SELECT sum(sipHash64(number)) FROM numbers(100000000000) FORMAT Null")
		done <- err
	}()
	var victim string
	deadline := time.Now().Add(10 * time.Second)
	for victim == "" && time.Now().Before(deadline) {
		ps, err := GetProcessList(ctx, db, ClickHouse)
		if err != nil {
			t.Fatalf("进程列表: %v", err)
		}
		for _, p := range ps {
			if strings.Contains(p.Info, "sipHash64") {
				victim = p.ID
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if victim == "" {
		t.Fatal("进程列表里没看到长查询")
	}
	if err := KillProcess(ctx, db, ClickHouse, victim, true); err != nil {
		t.Fatalf("KillProcess: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Error("被终止的查询应该返回错误")
		}
	case <-time.After(15 * time.Second):
		t.Error("终止后查询没有结束")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
