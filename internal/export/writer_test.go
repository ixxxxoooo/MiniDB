package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, o Options, table string, cols []string, rows []map[string]interface{}) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf, table, o)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Begin(cols); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRows(cols, rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCSVOptions(t *testing.T) {
	cols := []string{"id", "name", "price"}
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "a;b", "price": 1.5},
		{"id": int64(2), "name": nil, "price": 2.0},
		{"id": int64(3), "name": "say \"hi\"\nnow", "price": 0.25},
	}
	got := string(write(t, Options{Format: "csv", CSV: CSVOptions{Delimiter: ";", DecimalComma: true, NullText: "NULL", NoBOM: true, CRLF: true}}, "t", cols, rows))
	want := "id;name;price\r\n1;\"a;b\";1,5\r\n2;NULL;2\r\n3;\"say \"\"hi\"\"\nnow\";0,25\r\n"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	// 默认：逗号、BOM、表头、NULL 为空
	got = string(write(t, Options{Format: "csv"}, "t", cols, rows[:2]))
	if !strings.HasPrefix(got, "\xEF\xBB\xBFid,name,price\n") || !strings.Contains(got, "2,,2\n") {
		t.Fatalf("默认 CSV 不对: %q", got)
	}
	// 不加引号、没有表头
	got = string(write(t, Options{Format: "csv", CSV: CSVOptions{Quote: "none", NoHeader: true, NoBOM: true}}, "t", cols, rows[:1]))
	if got != "1,a;b,1.5\n" {
		t.Fatalf("got %q", got)
	}
}

func TestSQLOptions(t *testing.T) {
	cols := []string{"id", "name"}
	rows := []map[string]interface{}{
		{"id": int64(1), "name": `it's a\b`},
		{"id": int64(2), "name": nil},
		{"id": int64(3), "name": time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)},
	}
	q := func(s string) string { return "`" + s + "`" }
	got := string(write(t, Options{Format: "sql", SQL: SQLOptions{
		Structure: true, DropTable: true, DDL: "CREATE TABLE `t` (id int);", QuoteIdent: q, BackslashEscape: true,
	}}, "t", cols, rows))
	for _, w := range []string{
		"DROP TABLE IF EXISTS `t`;\n",
		"CREATE TABLE `t` (id int);\n",
		"INSERT INTO `t` (`id`, `name`) VALUES (1, 'it''s a\\\\b');\n",
		"VALUES (2, NULL);\n",
		"VALUES (3, '2024-01-02 03:04:05');\n",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("缺少 %q，实际:\n%s", w, got)
		}
	}
	got = string(write(t, Options{Format: "sql", SQL: SQLOptions{Structure: true, NoData: true, DDL: "CREATE TABLE t (id int)"}}, "t", cols, rows))
	if strings.Contains(got, "INSERT") || !strings.Contains(got, "CREATE TABLE t (id int);") {
		t.Fatalf("只要结构: %q", got)
	}
}

func TestJSONColumnsOrder(t *testing.T) {
	got := string(write(t, Options{Format: "json"}, "t", []string{"b", "a"}, []map[string]interface{}{
		{"a": 1, "b": []byte("x<y"), "c": "skip"},
	}))
	if !strings.Contains(got, `{"b": "x<y", "a": 1}`) {
		t.Fatalf("got %s", got)
	}
}

func TestXLSX(t *testing.T) {
	data := write(t, Options{Format: "xlsx"}, "users/1", []string{"id", "name"}, []map[string]interface{}{
		{"id": int64(1), "name": "a&b <c>\x01"},
		{"id": int64(2), "name": nil},
	})
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}
	sheet := files["xl/worksheets/sheet1.xml"]
	for _, w := range []string{`<c r="A2"><v>1</v></c>`, `a&amp;b &lt;c&gt;`, `<row r="3">`, `</sheetData>`} {
		if !strings.Contains(sheet, w) {
			t.Errorf("sheet 缺少 %q:\n%s", w, sheet)
		}
	}
	if strings.Contains(sheet, "\x01") {
		t.Error("控制字符没有去掉")
	}
	if !strings.Contains(files["xl/workbook.xml"], `name="users_1"`) {
		t.Errorf("工作表名: %s", files["xl/workbook.xml"])
	}
	if colRef(0) != "A" || colRef(25) != "Z" || colRef(26) != "AA" || colRef(701) != "ZZ" || colRef(702) != "AAA" {
		t.Error("colRef 不对")
	}
}

func TestPickColumns(t *testing.T) {
	all := []string{"a", "b", "c"}
	if got := PickColumns(all, []string{"c", "a"}); strings.Join(got, "") != "ac" {
		t.Fatal(got)
	}
	if got := PickColumns(all, nil); len(got) != 3 {
		t.Fatal(got)
	}
	if got := PickColumns(all, []string{"zz"}); len(got) != 3 {
		t.Fatal(got)
	}
}

func TestExtension(t *testing.T) {
	if (Options{Format: "sql", Gzip: true}).Extension() != "sql.gz" || (Options{Format: "xlsx", Gzip: true}).Extension() != "xlsx" {
		t.Fatal("扩展名不对")
	}
}

func TestJSONLinesAndSQLBatch(t *testing.T) {
	cols := []string{"id", "name"}
	rows := []map[string]interface{}{
		{"id": int64(1), "name": "a"}, {"id": int64(2), "name": nil}, {"id": int64(3), "name": "c"},
	}
	got := string(write(t, Options{Format: "json", JSON: JSONOptions{Lines: true}}, "t", cols, rows[:2]))
	if got != "{\"id\": 1, \"name\": \"a\"}\n{\"id\": 2, \"name\": null}\n" {
		t.Fatalf("JSON Lines: %q", got)
	}
	if (Options{Format: "json", JSON: JSONOptions{Lines: true}, Gzip: true}).Extension() != "jsonl.gz" {
		t.Fatal("JSON Lines 的扩展名应是 jsonl")
	}
	// 普通 JSON 不变
	got = string(write(t, Options{Format: "json"}, "t", cols, rows[:2]))
	if got != "[\n  {\"id\": 1, \"name\": \"a\"},\n  {\"id\": 2, \"name\": null}\n]\n" {
		t.Fatalf("JSON: %q", got)
	}
	// 多行 INSERT：每 2 行一条，最后不足的单独收尾
	got = string(write(t, Options{Format: "sql", SQL: SQLOptions{BatchSize: 2}}, "t", cols, rows))
	want := "INSERT INTO t (id, name) VALUES \n(1, 'a'),\n(2, NULL);\nINSERT INTO t (id, name) VALUES \n(3, 'c');\n"
	if got != want {
		t.Fatalf("批量 INSERT:\n%q\nwant\n%q", got, want)
	}
	// 默认仍是一行一条
	got = string(write(t, Options{Format: "sql"}, "t", cols, rows[:1]))
	if got != "INSERT INTO t (id, name) VALUES (1, 'a');\n" {
		t.Fatalf("单行 INSERT: %q", got)
	}
}

// 以 = + - @ 开头的文字默认被转义，数值本身不动；RawFormulas 可以关掉转义。
func TestCSVNeutralizesFormulas(t *testing.T) {
	rows := []map[string]interface{}{{
		"a": `=HYPERLINK("http://x")`, "b": "+1+2", "c": "@SUM(A1)", "d": "-5", "e": int64(-7), "f": 3.5, "g": "普通", "h": "\t=cmd",
	}}
	cols := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	run := func(o CSVOptions) string {
		var buf bytes.Buffer
		w := &csvWriter{w: &buf, o: o}
		o.NoHeader = true
		w.o = o
		if err := w.WriteRows(cols, rows); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	got := run(CSVOptions{NoBOM: true})
	want := `"'=HYPERLINK(""http://x"")",'+1+2,'@SUM(A1),-5,-7,3.5,普通,'` + "\t=cmd\n"
	if got != want {
		t.Fatalf("默认应转义:\n got %q\nwant %q", got, want)
	}
	raw := run(CSVOptions{NoBOM: true, RawFormulas: true})
	if !strings.HasPrefix(raw, `"=HYPERLINK(`) {
		t.Fatalf("关闭转义后应原样输出: %q", raw)
	}
	// 表头同样要转义
	var buf bytes.Buffer
	w := &csvWriter{w: &buf, o: CSVOptions{NoBOM: true}}
	if err := w.Begin([]string{"=x", "ok"}); err != nil || buf.String() != "'=x,ok\n" {
		t.Fatalf("表头: %q err=%v", buf.String(), err)
	}
}
