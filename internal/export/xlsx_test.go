package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSheetTitle(t *testing.T) {
	cases := map[string]string{
		"users":                 "users",
		"a:b/c\\d?e*f[g]h":      "a_b_c_d_e_f_g_h",
		"":                      "Sheet1",
		strings.Repeat("长", 40): strings.Repeat("长", 31),
	}
	for in, want := range cases {
		if got := sheetTitle(in); got != want {
			t.Errorf("sheetTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestColRef(t *testing.T) {
	for i, want := range map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"} {
		if got := colRef(i); got != want {
			t.Errorf("colRef(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestXMLEscape(t *testing.T) {
	if got := xmlEscape(`a<b>&"c"`); strings.ContainsAny(got, `<>"`) || !strings.Contains(got, "&amp;") {
		t.Fatalf("xmlEscape = %q", got)
	}
	// XML 1.0 不允许的控制字符不能原样写进去，否则 Excel 会报文件损坏
	if got := xmlEscape("a\x00b\x0bc"); strings.ContainsAny(got, "\x00\x0b") {
		t.Fatalf("控制字符应被去掉: %q", got)
	}
}

func TestXLSXIsAValidZipWithSheet(t *testing.T) {
	var buf bytes.Buffer
	w := newXLSXWriter(&buf, "t")
	if err := w.Begin([]string{"id", "name"}); err != nil {
		t.Fatal(err)
	}
	rows := []map[string]interface{}{{"id": 1, "name": "A<B"}, {"id": 2, "name": nil}}
	if err := w.WriteRows([]string{"id", "name"}, rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("输出应是合法 zip: %v", err)
	}
	var sheet string
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
		if strings.Contains(f.Name, "sheet1.xml") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			sheet = string(b)
		}
	}
	for _, must := range []string{"[Content_Types].xml", "xl/workbook.xml"} {
		if !names[must] {
			t.Errorf("缺少 %s: %v", must, names)
		}
	}
	if !strings.Contains(sheet, "A&lt;B") || strings.Contains(sheet, "A<B") {
		t.Fatalf("单元格文本应做 XML 转义: %q", sheet)
	}
}
