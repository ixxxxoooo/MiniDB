package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"minidb/internal/sqltext"
	"strconv"
	"strings"
	"time"
)

// Options 描述一次导出：格式、要导出的列，以及各格式自己的选项。
type Options struct {
	Format  string   // csv、json、sql、xlsx
	Gzip    bool     // 压缩成 .gz（xlsx 本身已压缩，忽略）
	Columns []string // 只导出这些列，空为全部

	CSV  CSVOptions
	JSON JSONOptions
	SQL  SQLOptions
}

// JSONOptions 是 JSON 的写法。零值是一个带缩进的数组，每行一个对象。
type JSONOptions struct {
	Lines bool // JSON Lines：不要外层数组，每行一个对象（扩展名 .jsonl，适合流式处理）
}

// CSVOptions 是 CSV 的写法。零值是逗号分隔、双引号、LF 换行、NULL 写成空、带 BOM 与表头。
type CSVOptions struct {
	Delimiter    string // 空为逗号；可为分号、制表符、竖线
	Quote        string // 空为双引号；"none" 表示从不加引号
	CRLF         bool   // 用 \r\n 换行
	DecimalComma bool   // 小数点写成逗号（欧洲格式）
	NullText     string // NULL 写成什么，默认空串
	NoHeader     bool   // 不写表头
	NoBOM        bool   // 不写 UTF-8 BOM
}

// SQLOptions 是 SQL 导出的内容：结构、先删表、数据。零值只导出数据。
type SQLOptions struct {
	Structure bool // 写 CREATE TABLE
	DropTable bool // 写 DROP TABLE IF EXISTS
	NoData    bool // 不写 INSERT
	// BatchSize 大于 1 时把多行合并成一条 INSERT（多行 VALUES），导入时快得多；0 或 1 为一行一条。
	BatchSize int
	DDL       string // 建表语句，Structure 时由调用方给出
	// QuoteIdent 给表名和列名加引号，空则原样。
	QuoteIdent func(string) string
	// BackslashEscape 为真时字符串里的反斜杠要写成两个（MySQL 系）。
	BackslashEscape bool
}

// Writer 逐批写出行。Begin 在第一批之前调用，Close 收尾；Close 之后不能再用。
type Writer interface {
	Begin(columns []string) error
	WriteRows(columns []string, rows []map[string]interface{}) error
	Close() error
}

// Extension 是格式的文件扩展名，含 gzip 时的 .gz。
func (o Options) Extension() string {
	ext := o.Format
	if o.Format == "xlsx" {
		return "xlsx"
	}
	if o.Format == "json" && o.JSON.Lines {
		ext = "jsonl"
	}
	if o.Gzip {
		ext += ".gz"
	}
	return ext
}

// NewWriter 按 opts.Format 建立写入器。table 用于 SQL 的表名。
func NewWriter(w io.Writer, table string, opts Options) (Writer, error) {
	switch opts.Format {
	case "csv":
		return &csvWriter{w: w, o: opts.CSV}, nil
	case "json":
		return &jsonWriter{w: w, first: true, lines: opts.JSON.Lines}, nil
	case "sql":
		return &sqlWriter{w: w, table: table, o: opts.SQL}, nil
	case "xlsx":
		return newXLSXWriter(w, table), nil
	}
	return nil, fmt.Errorf("不支持的导出格式: %s", opts.Format)
}

// PickColumns 把 all 缩到 want 里出现的列，保持 all 的顺序；want 为空或一个都没对上时返回 all。
func PickColumns(all, want []string) []string {
	if len(want) == 0 {
		return all
	}
	keep := map[string]bool{}
	for _, c := range want {
		keep[c] = true
	}
	var out []string
	for _, c := range all {
		if keep[c] {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

// ---- 值的文字写法 ----

func formatTime(t time.Time) string {
	if t.Nanosecond() != 0 {
		return t.Format("2006-01-02 15:04:05.999999")
	}
	return t.Format("2006-01-02 15:04:05")
}

// text 把一个单元格写成文字；ok 为假表示 NULL。
func text(v interface{}) (s string, ok bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case []byte:
		return string(x), true
	case string:
		return x, true
	case time.Time:
		return formatTime(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	}
	return fmt.Sprint(v), true
}

// ---- CSV ----

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
	return c.record(columns)
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
			rec[i] = s
		}
		if err := c.record(rec); err != nil {
			return err
		}
	}
	return nil
}

func (c *csvWriter) Close() error { return nil }

// ---- JSON ----

type jsonWriter struct {
	w     io.Writer
	first bool
	lines bool
}

func (j *jsonWriter) Begin([]string) error {
	if j.lines {
		return nil
	}
	_, err := io.WriteString(j.w, "[\n")
	return err
}

func (j *jsonWriter) WriteRows(columns []string, rows []map[string]interface{}) error {
	for _, row := range rows {
		var b bytes.Buffer
		switch {
		case j.lines:
			// 每行一个对象，没有逗号和缩进
		case !j.first:
			b.WriteString(",\n  {")
		default:
			b.WriteString("  {")
		}
		if j.lines {
			b.WriteString("{")
		}
		j.first = false
		for i, col := range columns {
			if i > 0 {
				b.WriteString(", ")
			}
			k, _ := json.Marshal(col)
			b.Write(k)
			b.WriteString(": ")
			v := row[col]
			switch x := v.(type) {
			case []byte:
				v = string(x)
			case time.Time:
				v = formatTime(x)
			}
			enc, err := marshalNoEscape(v)
			if err != nil {
				return err
			}
			b.Write(enc)
		}
		b.WriteString("}")
		if j.lines {
			b.WriteString("\n")
		}
		if _, err := j.w.Write(b.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (j *jsonWriter) Close() error {
	if j.lines {
		return nil
	}
	_, err := io.WriteString(j.w, "\n]\n")
	return err
}

// marshalNoEscape 编码一个值，不把 < > & 转成 \u003c 之类。
func marshalNoEscape(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// ---- SQL ----

type sqlWriter struct {
	w     io.Writer
	table string
	o     SQLOptions
}

func (s *sqlWriter) q(name string) string {
	if s.o.QuoteIdent != nil {
		return s.o.QuoteIdent(name)
	}
	return name
}

func (s *sqlWriter) Begin([]string) error {
	var b strings.Builder
	if s.o.DropTable {
		fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n\n", s.q(s.table))
	}
	if s.o.Structure && strings.TrimSpace(s.o.DDL) != "" {
		b.WriteString(strings.TrimRight(strings.TrimSpace(s.o.DDL), ";"))
		b.WriteString(";\n\n")
	}
	_, err := io.WriteString(s.w, b.String())
	return err
}

// literal 写一个 SQL 字面量。
func (s *sqlWriter) literal(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x)
	case float32, float64:
		str, _ := text(x)
		return str
	case bool:
		if x {
			return "1"
		}
		return "0"
	}
	str, _ := text(v)
	if s.o.BackslashEscape {
		return sqltext.QuoteString(strings.ReplaceAll(str, `\`, `\\`)) // 反斜杠先加倍，再由 QuoteString 加倍单引号
	}
	return sqltext.QuoteString(str)
}

func (s *sqlWriter) WriteRows(columns []string, rows []map[string]interface{}) error {
	if s.o.NoData {
		return nil
	}
	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = s.q(c)
	}
	head := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", s.q(s.table), strings.Join(names, ", "))
	batch := max(s.o.BatchSize, 1)
	var b strings.Builder
	for n, row := range rows {
		switch {
		case n%batch == 0:
			b.WriteString(head)
			if batch > 1 {
				b.WriteString("\n")
			}
		default:
			b.WriteString(",\n")
		}
		b.WriteString("(")
		for i, col := range columns {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(s.literal(row[col]))
		}
		b.WriteString(")")
		if (n+1)%batch == 0 || n == len(rows)-1 {
			b.WriteString(";\n")
		}
	}
	_, err := io.WriteString(s.w, b.String())
	return err
}

func (s *sqlWriter) Close() error { return nil }
