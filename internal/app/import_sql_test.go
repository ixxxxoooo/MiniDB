package app

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"minidb/internal/sqltext"
)

func TestIsSQLFile(t *testing.T) {
	for path, want := range map[string]bool{"a.sql": true, "A.SQL": true, "a.sql.gz": true, "a.csv": false, "sql": false, "a.sql.bak": false} {
		if got := IsSQLFile(path); got != want {
			t.Errorf("IsSQLFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// readSQLStatements 把转储文件的全部语句收集成切片（只给测试用，生产代码是流式处理的）。
func readSQLStatements(path string) ([]string, error) {
	var out []string
	err := walkSQLStatements(path, func(st sqltext.Statement) error {
		out = append(out, st.Text)
		return nil
	})
	return out, err
}

func TestReadSQLStatementsSplitsAndStripsBOM(t *testing.T) {
	p := writeFile(t, "d.sql", "\ufeffCREATE TABLE a(id int); -- 注释\nINSERT INTO a VALUES (1);\nINSERT INTO a VALUES ('x;y');")
	stmts, err := readSQLStatements(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 3 {
		t.Fatalf("应拆出 3 条语句（字符串里的分号不算），实际 %d: %q", len(stmts), stmts)
	}
	if !strings.HasPrefix(stmts[0], "CREATE TABLE") {
		t.Fatalf("BOM 没有被去掉: %q", stmts[0])
	}
}

func TestReadSQLStatementsGzip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.sql.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	_, _ = zw.Write([]byte("SELECT 1; SELECT 2;"))
	_ = zw.Close()
	_ = f.Close()
	stmts, err := readSQLStatements(p)
	if err != nil || len(stmts) != 2 {
		t.Fatalf("gz 读取 = %q, %v", stmts, err)
	}
}

func TestReadSQLStatementsBadGzipAndMissingFile(t *testing.T) {
	if _, err := readSQLStatements(writeFile(t, "bad.sql.gz", "不是压缩文件")); err == nil {
		t.Fatal("损坏的压缩文件应报错")
	}
	if _, err := readSQLStatements(filepath.Join(t.TempDir(), "nope.sql")); err == nil {
		t.Fatal("文件不存在应报错")
	}
}

func TestPreviewSQLFileFlagsRiskyStatements(t *testing.T) {
	p := writeFile(t, "d.sql", "CREATE TABLE a(id int);\nINSERT INTO a VALUES (1);\nDROP TABLE old;\n/* x */ DELETE FROM a;\nUPDATE a SET id=2;\nALTER TABLE a ADD c int;\nTRUNCATE a;")
	prev, err := (&ImportService{}).PreviewSQLFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Count != 7 || len(prev.Head) != 5 {
		t.Fatalf("Count=%d Head=%d", prev.Count, len(prev.Head))
	}
	if len(prev.Risky) != 5 {
		t.Fatalf("DROP/DELETE/UPDATE/ALTER/TRUNCATE 都应标为危险，实际 %d: %q", len(prev.Risky), prev.Risky)
	}
}

func TestPreviewSQLFileEmpty(t *testing.T) {
	if _, err := (&ImportService{}).PreviewSQLFile(writeFile(t, "e.sql", "  -- 只有注释\n")); err == nil {
		t.Fatal("没有语句应报错")
	}
}

func TestSnippetTruncatesAndCollapsesWhitespace(t *testing.T) {
	if got := snippet("SELECT\n  1\t FROM   t"); got != "SELECT 1 FROM t" {
		t.Fatalf("空白应折叠: %q", got)
	}
	long := snippet(strings.Repeat("字", 300))
	if r := []rune(long); len(r) != 121 || r[120] != '…' {
		t.Fatalf("应截到 120 个字加省略号，实际 %d 个字符", len(r))
	}
}
