package dataimport

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readAll(t *testing.T, r Reader) [][]interface{} {
	t.Helper()
	var out [][]interface{}
	for {
		row, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
}

func TestCSVHeaderBOMQuotesAndRaggedRows(t *testing.T) {
	p := writeTemp(t, "a.csv", "\xEF\xBB\xBFid,name,note\n1,\"Al, \"\"Bob\"\"\",x\n2,short\n3,a,b,extra\n")
	r, err := Open(p, Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"id", "name", "note"}) {
		t.Fatalf("columns=%v（BOM 应被去掉）", r.Columns())
	}
	rows := readAll(t, r)
	want := [][]interface{}{
		{"1", `Al, "Bob"`, "x"},
		{"2", "short", nil}, // 缺失的列补 nil
		{"3", "a", "b"},     // 多余的列丢弃
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows=%v", rows)
	}
}

func TestCSVNoHeaderKeepsFirstRowAndAutoDelimiter(t *testing.T) {
	p := writeTemp(t, "a.csv", "1;a\n2;b\n")
	r, err := Open(p, Options{HasHeader: false})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"column_1", "column_2"}) {
		t.Fatalf("columns=%v", r.Columns())
	}
	rows := readAll(t, r)
	if len(rows) != 2 || rows[0][0] != "1" || rows[1][1] != "b" {
		t.Fatalf("无表头时首行应作为数据: %v", rows)
	}
}

func TestCSVDuplicateAndEmptyHeaders(t *testing.T) {
	p := writeTemp(t, "a.tsv", "a\ta\t\n1\t2\t3\n")
	r, err := Open(p, Options{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"a", "a_2", "column_3"}) {
		t.Fatalf("columns=%v", r.Columns())
	}
}

func TestJSONArrayKeepsKeyOrderAndNumbers(t *testing.T) {
	p := writeTemp(t, "a.json", `[{"id": 9007199254740993, "name":"A","tags":["x","y"]},{"name":"B","id":2,"extra":true,"n":null}]`)
	r, err := Open(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"id", "name", "tags", "extra", "n"}) {
		t.Fatalf("列应按首次出现顺序: %v", r.Columns())
	}
	rows := readAll(t, r)
	if rows[0][0] != "9007199254740993" {
		t.Fatalf("大整数不应丢精度: %v", rows[0][0])
	}
	if rows[0][2] != `["x","y"]` {
		t.Fatalf("数组应序列化为 JSON 字符串: %v", rows[0][2])
	}
	if rows[1][3] != true || rows[1][4] != nil || rows[1][2] != nil {
		t.Fatalf("rows[1]=%v", rows[1])
	}
}

func TestJSONLines(t *testing.T) {
	p := writeTemp(t, "a.jsonl", "{\"a\":1}\n{\"a\":2,\"b\":\"x\"}\n")
	r, err := Open(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !reflect.DeepEqual(r.Columns(), []string{"a", "b"}) {
		t.Fatalf("columns=%v", r.Columns())
	}
	rows := readAll(t, r)
	if len(rows) != 2 || rows[0][0] != "1" || rows[1][1] != "x" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestJSONRejectsNonObjects(t *testing.T) {
	for _, content := range []string{`[1,2,3]`, `"x"`, `[]`, ``} {
		p := writeTemp(t, "bad.json", content)
		if r, err := Open(p, Options{}); err == nil {
			r.Close()
			t.Fatalf("%q 应被拒绝", content)
		}
	}
}

func TestJSONMoreRowsThanSample(t *testing.T) {
	var b []byte
	b = append(b, '[')
	for i := 0; i < 450; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(`{"i":1}`)...)
	}
	b = append(b, ']')
	p := writeTemp(t, "big.json", string(b))
	prev, err := BuildPreview(p, Options{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if prev.TotalRows != 450 || len(prev.Rows) != 3 {
		t.Fatalf("total=%d preview=%d（预读缓冲之外的行不能丢）", prev.TotalRows, len(prev.Rows))
	}
}
