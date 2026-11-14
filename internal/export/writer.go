package export

import (
	"fmt"
	"io"

	"strconv"

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
	// RawFormulas 为真时原样写出以 = + - @ 开头的文字。默认（零值）会在它们前面加一个 '：
	// 这类单元格在 Excel / WPS 里会被当成公式执行，库里存的恶意内容就能借导出文件攻击打开它的人。
	RawFormulas bool
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

// ---- JSON ----

// ---- SQL ----
