package export

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// xlsxMaxRows 是 Excel 单个工作表的行数上限（含表头）。
const xlsxMaxRows = 1048576

// xlsxWriter 流式写一个只有一张工作表的 .xlsx。
// xlsx 是 zip 包；这里用内联字符串，不需要共享字符串表，行可以边读边写。
type xlsxWriter struct {
	out   io.Writer
	zw    *zip.Writer
	sheet *bufio.Writer
	rows  int
	name  string
}

func newXLSXWriter(w io.Writer, sheetName string) *xlsxWriter {
	return &xlsxWriter{out: w, name: sheetName}
}

// sheetTitle 把表名整理成合法的工作表名：最长 31 字符，且不含 : \ / ? * [ ]。
func sheetTitle(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:\/?*[]`, r) {
			return '_'
		}
		return r
	}, name)
	r := []rune(name)
	if len(r) > 31 {
		r = r[:31]
	}
	if len(r) == 0 {
		return "Sheet1"
	}
	return string(r)
}

func (x *xlsxWriter) put(name, body string) error {
	f, err := x.zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, body)
	return err
}

func (x *xlsxWriter) Begin(columns []string) error {
	x.zw = zip.NewWriter(x.out)
	const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`</Types>`},
		{"_rels/.rels", xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`},
		{"xl/workbook.xml", xmlHead + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="` + xmlEscape(sheetTitle(x.name)) + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`</Relationships>`},
	}
	for _, p := range parts {
		if err := x.put(p.name, p.body); err != nil {
			return err
		}
	}
	f, err := x.zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return err
	}
	x.sheet = bufio.NewWriterSize(f, 64*1024)
	if _, err := x.sheet.WriteString(xmlHead + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`); err != nil {
		return err
	}
	cells := make([]interface{}, len(columns))
	for i, c := range columns {
		cells[i] = c
	}
	return x.row(cells)
}

// colRef 把 0 起的列号写成 A、B…Z、AA…。
func colRef(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// xmlEscape 转义 XML 文本，并去掉 XML 1.0 不允许的控制字符。
func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || r >= 0x10000:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (x *xlsxWriter) row(cells []interface{}) error {
	x.rows++
	if x.rows > xlsxMaxRows {
		return fmt.Errorf("超过 Excel 单个工作表的行数上限 %d，请改用 CSV 或拆分导出", xlsxMaxRows)
	}
	var b strings.Builder
	b.WriteString(`<row r="` + strconv.Itoa(x.rows) + `">`)
	for i, v := range cells {
		ref := colRef(i) + strconv.Itoa(x.rows)
		switch n := v.(type) {
		case nil:
			continue // NULL 留空格子
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			fmt.Fprintf(&b, `<c r="%s"><v>%d</v></c>`, ref, n)
		case float32, float64:
			s, _ := text(n)
			fmt.Fprintf(&b, `<c r="%s"><v>%s</v></c>`, ref, s)
		case bool:
			bit := 0
			if n {
				bit = 1
			}
			fmt.Fprintf(&b, `<c r="%s" t="b"><v>%d</v></c>`, ref, bit)
		case time.Time:
			fmt.Fprintf(&b, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xmlEscape(formatTime(n)))
		default:
			s, _ := text(v)
			fmt.Fprintf(&b, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, xmlEscape(s))
		}
	}
	b.WriteString(`</row>`)
	_, err := x.sheet.WriteString(b.String())
	return err
}

func (x *xlsxWriter) WriteRows(columns []string, rows []map[string]interface{}) error {
	cells := make([]interface{}, len(columns))
	for _, row := range rows {
		for i, c := range columns {
			cells[i] = row[c]
		}
		if err := x.row(cells); err != nil {
			return err
		}
	}
	return nil
}

func (x *xlsxWriter) Close() error {
	if x.sheet == nil {
		return nil
	}
	if _, err := x.sheet.WriteString(`</sheetData></worksheet>`); err != nil {
		return err
	}
	if err := x.sheet.Flush(); err != nil {
		return err
	}
	return x.zw.Close()
}
