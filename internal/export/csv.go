package export

import (
	"io"

	"strconv"
	"strings"
)

type csvWriter struct {
	w io.Writer
	o CSVOptions
}

func (c *csvWriter) delim() string {
	switch c.o.Delimiter {
	case "":
		return ","
	case "tab":
		return "\t"
	}
	return c.o.Delimiter
}

func (c *csvWriter) quote() string {
	switch c.o.Quote {
	case "":
		return `"`
	case "none":
		return ""
	}
	return c.o.Quote
}

func (c *csvWriter) newline() string {
	if c.o.CRLF {
		return "\r\n"
	}
	return "\n"
}

// field 写一个字段：含分隔符、引号或换行时加引号，引号双写。
func (c *csvWriter) field(s string) string {
	q := c.quote()
	if q == "" {
		return s
	}
	if strings.Contains(s, c.delim()) || strings.Contains(s, q) || strings.ContainsAny(s, "\r\n") {
		return q + strings.ReplaceAll(s, q, q+q) + q
	}
	return s
}

// formulaLead 在 Excel / WPS 里会让单元格被当成公式的开头字符（含制表符、回车：它们能绕过简单的前缀检查）。
const formulaLead = "=+-@\t\r"

// neutralize 给可能被当成公式的文字前面加一个 '，使表格软件把它当普通文本。
// 数值本身（包括 -5、+3.2 这样的写法）不是公式，原样保留。
func (c *csvWriter) neutralize(s string, orig any) string {
	if c.o.RawFormulas || s == "" || !strings.ContainsRune(formulaLead, rune(s[0])) {
		return s
	}
	switch orig.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return s
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return s
	}
	return "'" + s
}

func (c *csvWriter) record(fields []string) error {
	var b strings.Builder
	d := c.delim()
	for i, f := range fields {
		if i > 0 {
			b.WriteString(d)
		}
		b.WriteString(c.field(f))
	}
	b.WriteString(c.newline())
	_, err := io.WriteString(c.w, b.String())
	return err
}

func (c *csvWriter) Begin(columns []string) error {
	if !c.o.NoBOM {
		if _, err := c.w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return err
		}
	}
	if c.o.NoHeader {
		return nil
	}
	header := make([]string, len(columns))
	for i, col := range columns {
		header[i] = c.neutralize(col, nil)
	}
	return c.record(header)
}

func (c *csvWriter) WriteRows(columns []string, rows []map[string]interface{}) error {
	rec := make([]string, len(columns))
	for _, row := range rows {
		for i, col := range columns {
			s, ok := text(row[col])
			switch {
			case !ok:
				s = c.o.NullText
			case c.o.DecimalComma:
				if _, isFloat := row[col].(float64); isFloat {
					s = strings.Replace(s, ".", ",", 1)
				}
			}
			if ok {
				s = c.neutralize(s, row[col])
			}
			rec[i] = s
		}
		if err := c.record(rec); err != nil {
			return err
		}
	}
	return nil
}

func (c *csvWriter) Close() error { return nil }
