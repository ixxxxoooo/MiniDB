package app

import (
	"testing"

	"minidb/internal/database"
)

// countRows 用连接池里另一条连接数行：它只能看到已提交的数据。
func countCommitted(t *testing.T, svc *QueryService) int64 {
	t.Helper()
	db, err := svc.manager.GetDB("c1")
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.QueryRow("SELECT COUNT(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("统计已提交行数失败: %v", err)
	}
	return n
}

// countOwn 走服务层（手动提交时复用事务所在的会话），能看到未提交的数据。
func countOwn(t *testing.T, svc *QueryService) int64 {
	t.Helper()
	r, err := svc.ExecuteSQL("c1", "", "SELECT COUNT(*) AS n FROM t")
	if err != nil || r.Error != "" {
		t.Fatalf("统计失败: %v %v", err, r)
	}
	n, _ := r.Rows[0]["n"].(int64)
	return n
}

func mustExec(t *testing.T, svc *QueryService, sql string) {
	t.Helper()
	r, err := svc.ExecuteSQL("c1", "", sql)
	if err != nil || (r != nil && r.Error != "") {
		t.Fatalf("执行 %q 失败: %v %+v", sql, err, r)
	}
}

func TestManualTxCommitAndRollback(t *testing.T) {
	svc := newSQLiteQueryService(t)
	mustExec(t, svc, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")

	if err := svc.SetManualTransaction("c1", "", true); err != nil {
		t.Fatalf("开启手动提交失败: %v", err)
	}
	if st := svc.TransactionStatus("c1", ""); !st.Manual || st.Active {
		t.Fatalf("刚开启时应无事务: %+v", st)
	}
	// 只读语句不开事务
	if countOwn(t, svc) != 0 || svc.TransactionStatus("c1", "").Active {
		t.Fatal("SELECT 不应开启事务")
	}

	mustExec(t, svc, "INSERT INTO t (v) VALUES ('a')")
	mustExec(t, svc, "INSERT INTO t (v) VALUES ('b')")
	st := svc.TransactionStatus("c1", "")
	if !st.Active || st.Pending != 2 {
		t.Fatalf("两次写入后应有 2 条未提交: %+v", st)
	}
	if got := countOwn(t, svc); got != 2 {
		t.Fatalf("事务内应看到自己的 2 行，实际 %d", got)
	}
	if got := countCommitted(t, svc); got != 0 {
		t.Fatalf("未提交的行不应对其他连接可见，实际 %d", got)
	}
	if err := svc.SetManualTransaction("c1", "", false); err == nil {
		t.Fatal("有未提交更改时不应允许关闭手动提交")
	}

	if err := svc.RollbackTransaction("c1", ""); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if st := svc.TransactionStatus("c1", ""); st.Active || st.Pending != 0 {
		t.Fatalf("回滚后应无事务: %+v", st)
	}
	if got := countOwn(t, svc); got != 0 {
		t.Fatalf("回滚后应为 0 行，实际 %d", got)
	}

	mustExec(t, svc, "INSERT INTO t (v) VALUES ('c')")
	if err := svc.CommitTransaction("c1", ""); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if got := countCommitted(t, svc); got != 1 {
		t.Fatalf("提交后其他连接应看到 1 行，实际 %d", got)
	}

	if err := svc.SetManualTransaction("c1", "", false); err != nil {
		t.Fatalf("事务结束后应能关闭: %v", err)
	}
	// 关闭后恢复自动提交
	mustExec(t, svc, "INSERT INTO t (v) VALUES ('d')")
	if got := countCommitted(t, svc); got != 2 {
		t.Fatalf("自动提交模式下写入应立即可见，实际 %d", got)
	}
}

// 批量执行结束时不能替用户回滚；用户自己写的 BEGIN / COMMIT 要同步到状态里。
func TestManualTxBatchKeepsTransaction(t *testing.T) {
	svc := newSQLiteQueryService(t)
	mustExec(t, svc, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	if err := svc.SetManualTransaction("c1", "", true); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ExecuteSQLBatch("r", "c1", "", []string{"INSERT INTO t (v) VALUES ('a')", "INSERT INTO t (v) VALUES ('b')"}, 1, 100)
	if err != nil || len(res) != 2 || res[1].Error != "" {
		t.Fatalf("批量执行失败: %v %+v", err, res)
	}
	if st := svc.TransactionStatus("c1", ""); !st.Active || st.Pending != 2 {
		t.Fatalf("批次结束后事务应保留: %+v", st)
	}
	// 用户自己 COMMIT
	if res, err := svc.ExecuteSQLBatch("r", "c1", "", []string{"COMMIT"}, 1, 100); err != nil || res[0].Error != "" {
		t.Fatalf("COMMIT 失败: %v %+v", err, res)
	}
	if st := svc.TransactionStatus("c1", ""); st.Active {
		t.Fatalf("用户 COMMIT 后应无事务: %+v", st)
	}
	if got := countCommitted(t, svc); got != 2 {
		t.Fatalf("COMMIT 后应有 2 行，实际 %d", got)
	}
}

// 表格里的行编辑在手动提交模式下进入事务，而不是各自提交。
func TestManualTxRowEditsStayInTransaction(t *testing.T) {
	svc := newSQLiteQueryService(t)
	mustExec(t, svc, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	mustExec(t, svc, "INSERT INTO t (id, v) VALUES (1, 'x')")
	if err := svc.SetManualTransaction("c1", "", true); err != nil {
		t.Fatal(err)
	}
	err := svc.CommitTableDataChanges("c1", "", "t", nil,
		[]map[string]interface{}{{"id": 2, "v": "y"}},
		[]RowUpdate{{PrimaryKey: map[string]interface{}{"id": 1}, Changes: map[string]interface{}{"v": "z"}}})
	if err != nil {
		t.Fatalf("行编辑失败: %v", err)
	}
	if st := svc.TransactionStatus("c1", ""); !st.Active || st.Pending != 2 {
		t.Fatalf("两处改动应留在事务里: %+v", st)
	}
	if got := countCommitted(t, svc); got != 1 {
		t.Fatalf("未提交时其他连接仍只看到 1 行，实际 %d", got)
	}
	if err := svc.RollbackTransaction("c1", ""); err != nil {
		t.Fatal(err)
	}
	if got := countOwn(t, svc); got != 1 {
		t.Fatalf("回滚后应只剩原来的 1 行，实际 %d", got)
	}
}

// 断开连接时手动提交会话随之丢弃，事务被回滚。
func TestManualTxDiscardedOnDisconnect(t *testing.T) {
	svc := newSQLiteQueryService(t)
	mustExec(t, svc, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	if err := svc.SetManualTransaction("c1", "", true); err != nil {
		t.Fatal(err)
	}
	mustExec(t, svc, "INSERT INTO t (v) VALUES ('a')")
	cfg, _ := svc.manager.GetConfig("c1")
	if err := svc.manager.Disconnect("c1"); err != nil {
		t.Fatal(err)
	}
	if st := svc.TransactionStatus("c1", ""); st.Manual {
		t.Fatalf("断开后不应再有手动提交会话: %+v", st)
	}
	if err := svc.manager.Connect(&database.ConnectionConfig{ID: cfg.ID, Name: cfg.Name, Type: cfg.Type, Database: cfg.Database}); err != nil {
		t.Fatal(err)
	}
	if got := countCommitted(t, svc); got != 0 {
		t.Fatalf("断开时未提交的行应被回滚，实际 %d", got)
	}
}
