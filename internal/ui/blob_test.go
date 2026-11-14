package ui

import (
	"bytes"
	"strings"
	"testing"

	"minidb/internal/model"
)

func TestIsBlobType(t *testing.T) {
	for typ, want := range map[string]bool{"BLOB": true, "longblob": true, "bytea": true, "VARBINARY(16)": true, "text": false, "int": false} {
		if isBlobType(typ) != want {
			t.Errorf("%s: want %v", typ, want)
		}
	}
}

func TestBlobBytes(t *testing.T) {
	if b, ok := blobBytes("0xFF00"); !ok || !bytes.Equal(b, []byte{0xff, 0}) {
		t.Fatalf("0x 文本: %v %v", b, ok)
	}
	for _, v := range []any{"hello", "0xZZ", nil, int64(3)} {
		if _, ok := blobBytes(v); ok {
			t.Errorf("%v 不应当是二进制", v)
		}
	}
}

func TestCellDisplay(t *testing.T) {
	col := model.ColumnMeta{Name: "d", Type: "BLOB"}
	if s, _ := cellDisplay(col, "0xFF00"); !strings.Contains(s, "2") || strings.Contains(s, "FF") {
		t.Errorf("二进制应显示成占位: %q", s)
	}
	if s, _ := cellDisplay(col, []byte("text")); s != "text" {
		t.Errorf("UTF-8 照常显示: %q", s)
	}
	if _, null := cellDisplay(col, nil); !null {
		t.Error("NULL 应当保持 NULL")
	}
	txt := model.ColumnMeta{Name: "t", Type: "text"}
	if s, _ := cellDisplay(txt, "0xFF00"); s != "0xFF00" {
		t.Errorf("非二进制列不动: %q", s)
	}
}

func TestHexTextRoundTrip(t *testing.T) {
	src := make([]byte, 40)
	for i := range src {
		src[i] = byte(i * 7)
	}
	text := formatHexText(src)
	if strings.Count(text, "\n") != 2 {
		t.Fatalf("每行 16 字节: %q", text)
	}
	got, err := parseHexText(text)
	if err != nil || !bytes.Equal(got, src) {
		t.Fatalf("往返不一致: %v %v", got, err)
	}
	for _, in := range []string{"0xDE AD, be\nef", "deadbeef"} {
		if b, err := parseHexText(in); err != nil || !bytes.Equal(b, []byte{0xde, 0xad, 0xbe, 0xef}) {
			t.Errorf("%q: %v %v", in, b, err)
		}
	}
	for _, in := range []string{"ABC", "GG", "12 3"} {
		if _, err := parseHexText(in); err == nil {
			t.Errorf("%q 应当报错", in)
		}
	}
	if b, err := parseHexText(""); err != nil || len(b) != 0 {
		t.Errorf("空串是空字节: %v %v", b, err)
	}
}

func TestHexDump(t *testing.T) {
	d := hexDump([]byte("Hello, MiniDB!\x00\xff"), 0)
	if !strings.HasPrefix(d, "00000000  48 65 6C 6C 6F") || !strings.Contains(d, "|Hello, MiniDB!..|") {
		t.Fatalf("格式不对:\n%s", d)
	}
	big := hexDump(make([]byte, 100), 32)
	if strings.Count(big, "\n") < 2 || !strings.Contains(big, "68") {
		t.Fatalf("超出部分应注明省略数量:\n%s", big)
	}
}

func TestBlobOf(t *testing.T) {
	col := model.ColumnMeta{Name: "d", Type: "BLOB"}
	if b, ok := blobOf(col, "plain"); !ok || string(b) != "plain" {
		t.Errorf("BLOB 列里的文本: %v %v", b, ok)
	}
	if _, ok := blobOf(col, nil); ok {
		t.Error("NULL 不进二进制模式")
	}
	if _, ok := blobOf(model.ColumnMeta{Type: "text"}, "0xFF"); ok {
		t.Error("文本列不进二进制模式")
	}
}
