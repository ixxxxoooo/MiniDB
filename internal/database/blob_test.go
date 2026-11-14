package database

import (
	"bytes"
	"database/sql"
	"testing"
)

// 二进制按方言写成十六进制字面量，文本照旧加引号。
func TestSQLLiteralForBinary(t *testing.T) {
	bin := []byte{0xff, 0x00, 0xab}
	cases := []struct {
		db   DBType
		v    interface{}
		want string
	}{
		{"mysql", bin, "X'FF00AB'"},
		{"sqlite", bin, "X'FF00AB'"},
		{"postgres", bin, `'\xFF00AB'`},
		{"mysql", []byte("abc"), "'abc'"}, // 合法 UTF-8 当文本
		{"mysql", "x", "'x'"},
	}
	for _, c := range cases {
		if got := SQLLiteralFor(c.db, c.v); got != c.want {
			t.Errorf("%s %v: got %s want %s", c.db, c.v, got, c.want)
		}
	}
	got := PreviewChanges("postgres", "app", "t", nil, nil, []RowChange{{PrimaryKey: map[string]interface{}{"id": 1}, Changes: map[string]interface{}{"b": bin}}})
	if got[0] != `UPDATE "t" SET "b" = '\xFF00AB' WHERE "id" = 1;` {
		t.Fatalf("预览: %q", got)
	}
}

// []byte 经 UpdateRow 按二进制参数绑定，读回来逐字节一致（含 0x00 和非 UTF-8）。
func TestUpdateRowBindsBytes(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE f (id INTEGER PRIMARY KEY, data BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO f VALUES (1, NULL)`); err != nil {
		t.Fatal(err)
	}
	want := []byte{0x00, 0xff, 0xfe, 'a', 0x00, 0x80}
	if err := UpdateRow(db, "sqlite", "", "f", map[string]interface{}{"id": 1}, map[string]interface{}{"data": want}); err != nil {
		t.Fatal(err)
	}
	var got []byte
	if err := db.QueryRow(`SELECT data FROM f WHERE id = 1`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
}
