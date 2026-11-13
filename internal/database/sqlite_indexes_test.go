package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// 并发读取多张表的索引时，连接池很小也不能卡死：
// 读取索引清单的游标必须在查询各索引的列之前关闭，否则每个 goroutine 占着一条连接等第二条，互相等死，
// 读锁一直不放，后面的写入会撞上 SQLITE_BUSY。
func TestGetSQLiteIndexesConcurrentWithTinyPool(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 8; i++ {
		for _, stmt := range []string{
			fmt.Sprintf("CREATE TABLE t%d (id INTEGER PRIMARY KEY, a TEXT, b TEXT, c TEXT)", i),
			fmt.Sprintf("CREATE INDEX t%d_a ON t%d(a)", i, i),
			fmt.Sprintf("CREATE UNIQUE INDEX t%d_bc ON t%d(b, c)", i, i),
		} {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
		}
	}
	db.SetMaxOpenConns(2)

	done := make(chan struct{})
	results := make([][]IndexInfo, 8)
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], _ = GetIndexes(db, "sqlite", "", fmt.Sprintf("t%d", i))
			}()
		}
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("reading indexes deadlocked on the connection pool")
	}
	for i, idx := range results {
		byName := map[string]IndexInfo{}
		for _, x := range idx {
			byName[x.Name] = x
		}
		a, bc := byName[fmt.Sprintf("t%d_a", i)], byName[fmt.Sprintf("t%d_bc", i)]
		if len(a.Columns) != 1 || a.Columns[0] != "a" || a.IsUnique {
			t.Errorf("t%d_a: %+v", i, a)
		}
		if len(bc.Columns) != 2 || bc.Columns[1] != "c" || !bc.IsUnique {
			t.Errorf("t%d_bc: %+v", i, bc)
		}
	}
}
