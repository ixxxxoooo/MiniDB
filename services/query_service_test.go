package services

import (
	"path/filepath"
	"testing"
	"time"

	"minidb/internal/database"
)

func newSQLiteQueryService(t *testing.T) *QueryService {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "test", Type: "sqlite", Database: filepath.Join(t.TempDir(), "q.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatalf("连接 sqlite 失败: %v", err)
	}
	t.Cleanup(manager.CloseAll)
	return NewQueryService(manager, nil)
}

// 临时表只在创建它的连接上可见：三条语句都成功说明整批跑在同一条连接上。
func TestExecuteSQLBatchSharesOneSession(t *testing.T) {
	svc := newSQLiteQueryService(t)
	results, err := svc.ExecuteSQLBatch("r1", "c1", "", []string{
		"CREATE TEMP TABLE batch_tmp (v INTEGER)",
		"INSERT INTO batch_tmp VALUES (7)",
		"SELECT v FROM batch_tmp",
	}, 1, 100)
	if err != nil {
		t.Fatalf("批量执行失败: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("期望 3 条结果，实际 %d", len(results))
	}
	for i, r := range results {
		if r.Error != "" {
			t.Fatalf("第 %d 条不应出错: %s", i+1, r.Error)
		}
	}
	if len(results[2].Rows) != 1 || results[2].Rows[0]["v"] != int64(7) {
		t.Fatalf("同一会话内应读到刚插入的行，实际 %#v", results[2].Rows)
	}
}

func TestExecuteSQLBatchStopsAtFirstError(t *testing.T) {
	svc := newSQLiteQueryService(t)
	results, err := svc.ExecuteSQLBatch("r2", "c1", "", []string{
		"SELECT 1 AS a",
		"SELECT * FROM table_that_does_not_exist",
		"SELECT 2 AS b",
	}, 1, 100)
	if err != nil {
		t.Fatalf("批量执行失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("出错后应停止，期望 2 条结果，实际 %d", len(results))
	}
	if results[1].Error == "" {
		t.Fatal("第 2 条应带错误信息")
	}
}

// 批次里 BEGIN 后没有 COMMIT：连接归还前必须回滚，未提交的数据不能留下。
func TestExecuteSQLBatchRollsBackUnfinishedTransaction(t *testing.T) {
	svc := newSQLiteQueryService(t)
	if _, err := svc.ExecuteSQLBatch("r3", "c1", "", []string{"CREATE TABLE pending (v INTEGER)"}, 1, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ExecuteSQLBatch("r4", "c1", "", []string{"BEGIN", "INSERT INTO pending VALUES (1)"}, 1, 100); err != nil {
		t.Fatal(err)
	}
	results, err := svc.ExecuteSQLBatch("r5", "c1", "", []string{"SELECT COUNT(*) AS n FROM pending"}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := results[0].Rows[0]["n"]; got != int64(0) {
		t.Fatalf("未提交的事务应被回滚，实际行数 %#v", got)
	}
}

func TestCancelQueryInterruptsRunningStatement(t *testing.T) {
	svc := newSQLiteQueryService(t)
	// 一个跑不完的递归查询
	slow := "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 1000000000000) SELECT COUNT(*) AS n FROM c"

	type outcome struct {
		results []*database.QueryResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		results, err := svc.ExecuteSQLBatch("slow", "c1", "", []string{slow, "SELECT 1"}, 0, 0)
		done <- outcome{results, err}
	}()

	time.Sleep(200 * time.Millisecond)
	svc.CancelQuery("slow")

	select {
	case out := <-done:
		if out.err != nil {
			t.Fatalf("取消应以结果形式返回而不是错误: %v", out.err)
		}
		if len(out.results) != 1 {
			t.Fatalf("取消后不应继续执行后续语句，实际结果数 %d", len(out.results))
		}
		if out.results[0].ErrorCode != queryErrorCodeCancelled {
			t.Fatalf("期望 errorCode=cancelled，实际 %q (%s)", out.results[0].ErrorCode, out.results[0].Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("取消后查询没有在 10 秒内返回")
	}
}

func newReadOnlySQLiteQueryService(t *testing.T) (*QueryService, *database.Manager) {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "ro", Name: "ro", Type: "sqlite", Database: filepath.Join(t.TempDir(), "ro.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	// 先在可写状态下建表，再切换成只读
	svc := NewQueryService(manager, nil)
	if _, err := svc.ExecuteSQLBatch("setup", "ro", "", []string{"CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)", "INSERT INTO t VALUES (1, 'a')"}, 1, 10); err != nil {
		t.Fatal(err)
	}
	cfg.ReadOnly = true
	t.Cleanup(manager.CloseAll)
	return svc, manager
}

func TestReadOnlyConnectionRejectsWrites(t *testing.T) {
	svc, _ := newReadOnlySQLiteQueryService(t)

	res, err := svc.ExecuteSQLBatch("r1", "ro", "", []string{"SELECT * FROM t"}, 1, 10)
	if err != nil || res[0].Error != "" || len(res[0].Rows) != 1 {
		t.Fatalf("只读连接应允许 SELECT: %v %+v", err, res)
	}

	for _, stmt := range []string{"DELETE FROM t", "INSERT INTO t VALUES (2,'b')", "DROP TABLE t", "UPDATE t SET v='x'"} {
		res, err := svc.ExecuteSQLBatch("r2", "ro", "", []string{stmt}, 1, 10)
		if err != nil {
			t.Fatal(err)
		}
		if res[0].ErrorCode != "read_only" {
			t.Fatalf("%q 应被只读模式拒绝，实际 %+v", stmt, res[0])
		}
	}
	single, err := svc.ExecuteSQL("ro", "", "DELETE FROM t")
	if err != nil || single.ErrorCode != "read_only" {
		t.Fatalf("单条执行路径也应拒绝写入: %v %+v", err, single)
	}
	if err := svc.InsertRow("ro", "", "t", map[string]interface{}{"id": 3}); err == nil {
		t.Fatal("InsertRow 应被拒绝")
	}
	if err := svc.CommitTableDataChanges("ro", "", "t", nil, []map[string]interface{}{{"id": 4}}, nil); err == nil {
		t.Fatal("CommitTableDataChanges 应被拒绝")
	}
	// 数据未被改动
	res, _ = svc.ExecuteSQLBatch("r3", "ro", "", []string{"SELECT COUNT(*) AS n FROM t"}, 0, 0)
	if res[0].Rows[0]["n"] != int64(1) {
		t.Fatalf("只读模式下数据不应变化: %+v", res[0].Rows)
	}
}
