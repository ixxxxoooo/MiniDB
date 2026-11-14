package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

// failingCountExecutor 让 COUNT(*)（走 QueryRowContext）必然失败，其余照常执行，
// 用来模拟大表上 COUNT 超时的场景。
type failingCountExecutor struct{ *sql.DB }

func (f failingCountExecutor) QueryRowContext(ctx context.Context, _ string, _ ...any) *sql.Row {
	return f.DB.QueryRowContext(ctx, "SELECT * FROM __no_such_table__")
}

func seedNumbers(t *testing.T, db *sql.DB, n int) {
	t.Helper()
	if _, err := db.Exec("CREATE TABLE nums (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	tx, _ := db.Begin()
	for i := 1; i <= n; i++ {
		if _, err := tx.Exec(fmt.Sprintf("INSERT INTO nums VALUES (%d)", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// COUNT 失败不能退回全量查询：仍然只取一页，并用多取的 1 行判断后面还有没有。
func TestAutoLimitStillLimitsWhenCountFails(t *testing.T) {
	db := openTestSQLite(t)
	seedNumbers(t, db, 25)
	ex := failingCountExecutor{db}

	res, err := ExecuteQueryPagedContext(context.Background(), ex, "SELECT id FROM nums ORDER BY id", 1, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 10 || !res.AutoLimited || !res.TotalUnknown || res.Total != 11 {
		t.Fatalf("第一页应只有 10 行且总数未知(下界 11): rows=%d auto=%v unknown=%v total=%d",
			len(res.Rows), res.AutoLimited, res.TotalUnknown, res.Total)
	}

	last, err := ExecuteQueryPagedContext(context.Background(), ex, "SELECT id FROM nums ORDER BY id", 3, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Rows) != 5 || last.TotalUnknown || last.Total != 25 {
		t.Fatalf("最后一页总数应精确为 25: rows=%d unknown=%v total=%d", len(last.Rows), last.TotalUnknown, last.Total)
	}
}

func TestScanRowsHardLimit(t *testing.T) {
	db := openTestSQLite(t)
	seedNumbers(t, db, 50)
	old := maxScanRows
	maxScanRows = 20
	t.Cleanup(func() { maxScanRows = old })

	res, err := ExecuteQueryRaw(db, "SELECT id FROM nums")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Rows) != 20 {
		t.Fatalf("应在 20 行处截断: truncated=%v rows=%d", res.Truncated, len(res.Rows))
	}

	maxScanRows = 50
	res, err = ExecuteQueryRaw(db, "SELECT id FROM nums")
	if err != nil || res.Truncated || len(res.Rows) != 50 {
		t.Fatalf("恰好等于上限不应截断: err=%v truncated=%v rows=%d", err, res.Truncated, len(res.Rows))
	}
}

func TestQueryTableDataHonorsContext(t *testing.T) {
	db := openTestSQLite(t)
	seedNumbers(t, db, 3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := QueryTableData(ctx, db, SQLite, "", "nums", 1, 10, nil, nil); err == nil {
		t.Fatal("已取消的 ctx 应让取表数据返回错误")
	}
}
