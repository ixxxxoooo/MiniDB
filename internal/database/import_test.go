package database

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

func newImportSession(t *testing.T) (*Session, func(q string) int) {
	t.Helper()
	m := NewManager()
	cfg := &ConnectionConfig{ID: "i1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "i.db")}
	if err := m.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.CloseAll)
	db, err := m.GetDB("i1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT NOT NULL, age INTEGER)`); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSession(context.Background(), db, "sqlite", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	count := func(q string) int {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	return s, count
}

func sliceSource(rows [][]interface{}) func() ([]interface{}, error) {
	i := 0
	return func() ([]interface{}, error) {
		if i >= len(rows) {
			return nil, io.EOF
		}
		i++
		return rows[i-1], nil
	}
}

func manyRows(n int) [][]interface{} {
	rows := make([][]interface{}, n)
	for i := range rows {
		rows[i] = []interface{}{int64(i + 1), "n", "30"}
	}
	return rows
}

var peopleCols = []ImportColumn{{0, "id"}, {1, "name"}, {2, "age"}}

func TestImportRowsBatchesAndMapping(t *testing.T) {
	s, count := newImportSession(t)
	var progress []int64
	res, err := ImportRows(context.Background(), s, "sqlite", "", "people", peopleCols,
		sliceSource(manyRows(1203)), ImportOptions{Atomic: true, BatchSize: 500},
		func(d int64) { progress = append(progress, d) })
	if err != nil || res.Inserted != 1203 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if got := count(`SELECT COUNT(*) FROM people`); got != 1203 {
		t.Fatalf("表内行数=%d", got)
	}
	if len(progress) != 3 || progress[2] != 1203 {
		t.Fatalf("进度回调=%v", progress)
	}
}

func TestImportRowsEmptyAsNullAndSkippedColumn(t *testing.T) {
	s, count := newImportSession(t)
	rows := [][]interface{}{{"1", "A", ""}, {"2", "B", "7"}}
	// 只映射 id 与 age；name 未映射会触发 NOT NULL 失败，所以这里映射 id,name,age 并检查空串→NULL
	if _, err := ImportRows(context.Background(), s, "sqlite", "", "people", peopleCols,
		sliceSource(rows), ImportOptions{Atomic: true, EmptyAsNull: true}, nil); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT COUNT(*) FROM people WHERE age IS NULL`); got != 1 {
		t.Fatalf("空串应写成 NULL，NULL 行数=%d", got)
	}
	// 源列下标越界视为 NULL
	rows2 := [][]interface{}{{"3", "C"}}
	if _, err := ImportRows(context.Background(), s, "sqlite", "", "people", peopleCols,
		sliceSource(rows2), ImportOptions{Atomic: true}, nil); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT COUNT(*) FROM people WHERE age IS NULL`); got != 2 {
		t.Fatalf("缺失列应为 NULL，实际 %d", got)
	}
}

func TestImportRowsAtomicRollsBackOnError(t *testing.T) {
	s, count := newImportSession(t)
	rows := manyRows(30)
	rows[25] = []interface{}{int64(26), nil, "1"} // name NOT NULL 违反，落在第二批
	res, err := ImportRows(context.Background(), s, "sqlite", "", "people", peopleCols,
		sliceSource(rows), ImportOptions{Atomic: true, BatchSize: 10}, nil)
	if err == nil {
		t.Fatal("期望出错")
	}
	if !res.RolledBack || res.Inserted != 0 {
		t.Fatalf("res=%+v", res)
	}
	if got := count(`SELECT COUNT(*) FROM people`); got != 0 {
		t.Fatalf("原子导入失败后表应为空，实际 %d", got)
	}
}

func TestImportRowsNonAtomicKeepsCommittedBatches(t *testing.T) {
	s, count := newImportSession(t)
	rows := manyRows(30)
	rows[25] = []interface{}{int64(26), nil, "1"}
	res, err := ImportRows(context.Background(), s, "sqlite", "", "people", peopleCols,
		sliceSource(rows), ImportOptions{Atomic: false, BatchSize: 10}, nil)
	if err == nil {
		t.Fatal("期望出错")
	}
	if res.Inserted != 20 || res.RolledBack {
		t.Fatalf("前两批应保留: %+v", res)
	}
	if got := count(`SELECT COUNT(*) FROM people`); got != 20 {
		t.Fatalf("表内行数=%d", got)
	}
}

func TestImportRowsCancelRollsBack(t *testing.T) {
	s, count := newImportSession(t)
	ctx, cancel := context.WithCancel(context.Background())
	// 读取到第 15 行时取消
	i := 0
	src := func() ([]interface{}, error) {
		i++
		if i == 15 {
			cancel()
		}
		if i > 100 {
			return nil, io.EOF
		}
		return []interface{}{int64(i), "n", "1"}, nil
	}
	// session 的 ctx 需与取消联动，这里直接依赖 ImportRows 自身检查 ctx
	_, err := ImportRows(ctx, s, "sqlite", "", "people", peopleCols, src, ImportOptions{Atomic: true, BatchSize: 10}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if got := count(`SELECT COUNT(*) FROM people`); got != 0 {
		t.Fatalf("取消后应回滚，实际 %d 行", got)
	}
}

func TestImportRowsRejectsBadMapping(t *testing.T) {
	s, _ := newImportSession(t)
	for _, cols := range [][]ImportColumn{nil, {{0, "id"}, {1, "ID"}}, {{0, ""}}} {
		if _, err := ImportRows(context.Background(), s, "sqlite", "", "people", cols, sliceSource(nil), ImportOptions{}, nil); err == nil {
			t.Fatalf("映射 %v 应被拒绝", cols)
		}
	}
}

func TestBuildImportInsertPlaceholders(t *testing.T) {
	q, args := buildImportInsert("postgres", "INSERT INTO t (a, b) VALUES ", 2, [][]interface{}{{1, 2}, {3, 4}})
	if q != "INSERT INTO t (a, b) VALUES ($1, $2), ($3, $4)" || len(args) != 4 {
		t.Fatalf("q=%s", q)
	}
	q, _ = buildImportInsert("mysql", "INSERT INTO t (a) VALUES ", 1, [][]interface{}{{1}, {2}})
	if q != "INSERT INTO t (a) VALUES (?), (?)" {
		t.Fatalf("q=%s", q)
	}
}
