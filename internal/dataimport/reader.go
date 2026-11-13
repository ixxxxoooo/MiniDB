// Package dataimport 解析导入文件（CSV/TSV/JSON/JSON Lines），以流式方式逐行产出，
// 不会把整个文件读进内存。写库逻辑见 database.ImportRows。
package dataimport

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/transform"
)

// Format 导入文件格式
const (
	FormatCSV  = "csv"
	FormatJSON = "json"
)

// Options 解析选项
type Options struct {
	// Format 为空时按扩展名判断：.json/.jsonl/.ndjson → json，其余按 csv。
	Format string `json:"format"`
	// HasHeader CSV 首行是否为列名；false 时列名为 column_1…。
	HasHeader bool `json:"hasHeader"`
	// Delimiter CSV 分隔符，空表示自动识别（, ; Tab |）。
	Delimiter string `json:"delimiter"`
	// Encoding CSV 的文字编码，空为 UTF-8；可选值见 Encodings。
	Encoding string `json:"encoding"`
	// Quote CSV 的引号字符：空为双引号，"'" 为单引号，"none" 表示没有引号。
	Quote string `json:"quote"`
}

// Reader 逐行读取导入文件。Next 返回 io.EOF 表示结束；每行长度恒等于 len(Columns())。
type Reader interface {
	Columns() []string
	Next() ([]interface{}, error)
	Close() error
}

// DetectFormat 根据扩展名判断格式
func DetectFormat(path string) string {
	switch strings.ToLower(filepath.Ext(strings.TrimSuffix(strings.ToLower(path), ".gz"))) {
	case ".json", ".jsonl", ".ndjson":
		return FormatJSON
	default:
		return FormatCSV
	}
}

// DetectFormatOf 返回实际使用的格式：显式指定优先，否则按扩展名。
func DetectFormatOf(path string, opts Options) string {
	if opts.Format != "" {
		return opts.Format
	}
	return DetectFormat(path)
}

// Open 打开导入文件并读出列信息。调用方负责 Close。
func Open(path string, opts Options) (Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	format := opts.Format
	if format == "" {
		format = DetectFormat(path)
	}
	var r Reader
	switch format {
	case FormatJSON:
		if strings.HasSuffix(strings.ToLower(path), ".gz") {
			err = errors.New("JSON 压缩文件请先解压")
			break
		}
		r, err = newJSONReader(f)
	case FormatCSV:
		r, err = newCSVReader(f, path, opts)
	default:
		err = fmt.Errorf("不支持的导入格式: %s", format)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return r, nil
}

// ---------- CSV ----------

// recordReader 读一条 CSV 记录；encoding/csv 和自带的任意引号读取器都满足它。
type recordReader interface {
	Read() ([]string, error)
}

type csvReader struct {
	closer  io.Closer
	r       recordReader
	columns []string
	pending []string // 无表头时，已读出用于确定列数的首行
}

// multiCloser 依次关闭解压层和文件。
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var first error
	for _, c := range m {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func newCSVReader(f *os.File, path string, opts Options) (*csvReader, error) {
	closers := multiCloser{f}
	var src io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("读取压缩文件失败: %w", err)
		}
		closers = append(multiCloser{gz}, closers...)
		src = gz
	}
	if dec, err := decoderFor(opts.Encoding); err != nil {
		return nil, err
	} else if dec != nil {
		src = transform.NewReader(src, dec)
	}
	br := bufio.NewReaderSize(src, 64*1024)
	// 去掉 UTF-8 BOM（Excel 导出的 CSV 常带）
	if head, err := br.Peek(3); err == nil && bytes.Equal(head, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	delim := resolveDelimiter(br, strings.TrimSuffix(strings.ToLower(path), ".gz"), opts.Delimiter)
	var rr recordReader
	if opts.Quote == "" || opts.Quote == `"` {
		cr := csv.NewReader(br)
		cr.Comma = delim
		cr.LazyQuotes = true
		cr.FieldsPerRecord = -1
		rr = cr
	} else {
		q := rune(0) // 0 表示没有引号
		if opts.Quote != "none" {
			q = []rune(opts.Quote)[0]
		}
		rr = &quoteReader{br: br, comma: delim, quote: q}
	}
	rd := &csvReader{closer: closers, r: rr}

	first, err := rr.Read()
	if err == io.EOF {
		return nil, errors.New("文件为空")
	}
	if err != nil {
		return nil, fmt.Errorf("读取 CSV 失败: %w", err)
	}
	if opts.HasHeader {
		rd.columns = uniqueColumns(first)
	} else {
		rd.columns = make([]string, len(first))
		for i := range first {
			rd.columns[i] = fmt.Sprintf("column_%d", i+1)
		}
		rd.pending = first
	}
	return rd, nil
}

// quoteReader 是可换引号字符的 CSV 记录读取器：引号只在字段开头生效，
// 引号里两个引号表示一个引号字符，行为和 LazyQuotes 的 encoding/csv 一致。
type quoteReader struct {
	br    *bufio.Reader
	comma rune
	quote rune // 0 为没有引号
}

func (q *quoteReader) Read() ([]string, error) {
	var rec []string
	var field strings.Builder
	atStart, inQuote, any := true, false, false
	for {
		r, _, err := q.br.ReadRune()
		if err == io.EOF {
			if !any {
				return nil, io.EOF
			}
			return append(rec, field.String()), nil
		}
		if err != nil {
			return nil, err
		}
		any = true
		switch {
		case inQuote:
			if r != q.quote {
				field.WriteRune(r)
				continue
			}
			next, _, err := q.br.ReadRune()
			switch {
			case err == nil && next == q.quote:
				field.WriteRune(q.quote)
			default:
				inQuote = false
				if err == nil {
					_ = q.br.UnreadRune()
				}
			}
		case q.quote != 0 && r == q.quote && atStart:
			inQuote, atStart = true, false
		case r == q.comma:
			rec = append(rec, field.String())
			field.Reset()
			atStart = true
		case r == '\n':
			return append(rec, strings.TrimSuffix(field.String(), "\r")), nil
		default:
			field.WriteRune(r)
			atStart = false
		}
	}
}

func resolveDelimiter(br *bufio.Reader, path, want string) rune {
	switch want {
	case "\\t", "tab", "\t":
		return '\t'
	case ",", ";", "|":
		return rune(want[0])
	}
	if strings.EqualFold(filepath.Ext(path), ".tsv") {
		return '\t'
	}
	// 自动识别：在首行（引号之外）统计候选分隔符出现次数，取最多者
	peek, _ := br.Peek(8192)
	line := peek
	if i := bytes.IndexByte(peek, '\n'); i >= 0 {
		line = peek[:i]
	}
	best, bestN := ',', 0
	for _, cand := range []rune{',', ';', '\t', '|'} {
		n, inQuote := 0, false
		for _, b := range line {
			if b == '"' {
				inQuote = !inQuote
			} else if !inQuote && rune(b) == cand {
				n++
			}
		}
		if n > bestN {
			best, bestN = cand, n
		}
	}
	return best
}

// uniqueColumns 保证列名非空且不重复（空名→column_N，重名追加 _2、_3…）。
func uniqueColumns(raw []string) []string {
	out := make([]string, len(raw))
	seen := map[string]int{}
	for i, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		key := strings.ToLower(name)
		seen[key]++
		if n := seen[key]; n > 1 {
			name = fmt.Sprintf("%s_%d", name, n)
		}
		out[i] = name
	}
	return out
}

func (c *csvReader) Columns() []string { return c.columns }

func (c *csvReader) Next() ([]interface{}, error) {
	rec := c.pending
	if rec != nil {
		c.pending = nil
	} else {
		var err error
		rec, err = c.r.Read()
		if err != nil {
			return nil, err
		}
	}
	row := make([]interface{}, len(c.columns))
	for i := range row {
		if i < len(rec) {
			row[i] = rec[i]
		}
	}
	return row, nil
}

func (c *csvReader) Close() error { return c.closer.Close() }

// ---------- JSON / JSON Lines ----------

type jsonReader struct {
	closer  io.Closer
	dec     *json.Decoder
	isArray bool
	columns []string
	buffer  []map[string]interface{} // 为确定列而预读的行
	done    bool
}

// 预读多少行来确定列集合
const jsonSampleRows = 200

func newJSONReader(f *os.File) (*jsonReader, error) {
	br := bufio.NewReaderSize(f, 64*1024)
	if head, err := br.Peek(3); err == nil && bytes.Equal(head, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	dec := json.NewDecoder(br)
	dec.UseNumber()
	jr := &jsonReader{closer: f, dec: dec}

	tok, err := dec.Token()
	if err == io.EOF {
		return nil, errors.New("文件为空")
	}
	if err != nil {
		return nil, fmt.Errorf("读取 JSON 失败: %w", err)
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '[':
			jr.isArray = true
		case '{':
			// JSON Lines / 连续对象：把已吞掉的 '{' 补回去比较麻烦，直接重新定位。
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
			br2 := bufio.NewReaderSize(f, 64*1024)
			if head, err := br2.Peek(3); err == nil && bytes.Equal(head, []byte{0xEF, 0xBB, 0xBF}) {
				_, _ = br2.Discard(3)
			}
			jr.dec = json.NewDecoder(br2)
			jr.dec.UseNumber()
		default:
			return nil, errors.New("JSON 必须是对象数组或每行一个对象")
		}
	default:
		return nil, errors.New("JSON 必须是对象数组或每行一个对象")
	}

	order := []string{}
	seen := map[string]bool{}
	for len(jr.buffer) < jsonSampleRows {
		obj, keys, err := jr.readObject()
		if err == io.EOF {
			jr.done = true
			break
		}
		if err != nil {
			return nil, err
		}
		jr.buffer = append(jr.buffer, obj)
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				order = append(order, k)
			}
		}
	}
	if len(order) == 0 {
		return nil, errors.New("JSON 中没有可导入的对象")
	}
	jr.columns = order
	return jr, nil
}

// readObject 读下一个对象，并按出现顺序返回其键。
func (j *jsonReader) readObject() (map[string]interface{}, []string, error) {
	if j.isArray && !j.dec.More() {
		return nil, nil, io.EOF
	}
	var raw json.RawMessage
	if err := j.dec.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil, nil, io.EOF
		}
		return nil, nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	keys, err := orderedKeys(raw)
	if err != nil {
		return nil, nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var obj map[string]interface{}
	if err := d.Decode(&obj); err != nil {
		return nil, nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	return obj, keys, nil
}

// orderedKeys 返回对象顶层键的出现顺序；元素不是对象时报错。
func orderedKeys(raw []byte) ([]string, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, errors.New("JSON 数组元素必须是对象")
	}
	var keys []string
	for d.More() {
		kt, err := d.Token()
		if err != nil {
			return nil, err
		}
		keys = append(keys, kt.(string))
		var skip json.RawMessage
		if err := d.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

func (j *jsonReader) Columns() []string { return j.columns }

func (j *jsonReader) Next() ([]interface{}, error) {
	var obj map[string]interface{}
	switch {
	case len(j.buffer) > 0:
		obj, j.buffer = j.buffer[0], j.buffer[1:]
	case j.done:
		return nil, io.EOF
	default:
		o, _, err := j.readObject()
		if err != nil {
			return nil, err
		}
		obj = o
	}
	row := make([]interface{}, len(j.columns))
	for i, col := range j.columns {
		row[i] = jsonValue(obj[col])
	}
	return row, nil
}

// jsonValue 把解码值转成可交给驱动的值：数字保持原文本（避免大整数/小数精度丢失），
// 嵌套对象和数组序列化成 JSON 字符串。
func jsonValue(v interface{}) interface{} {
	switch x := v.(type) {
	case nil, bool, string:
		return x
	case json.Number:
		return x.String()
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

func (j *jsonReader) Close() error { return j.closer.Close() }

// ---------- 预览与计数 ----------

// Preview 文件预览
type Preview struct {
	Format    string     `json:"format"`
	Columns   []string   `json:"columns"`
	Rows      [][]string `json:"rows"`      // 前 N 行，NULL 显示为空串
	TotalRows int64      `json:"totalRows"` // 数据行总数（不含表头）
}

// BuildPreview 读取列、前 limit 行以及总行数。
func BuildPreview(path string, opts Options, limit int) (*Preview, error) {
	if opts.Format == "" {
		opts.Format = DetectFormat(path)
	}
	r, err := Open(path, opts)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := &Preview{Format: opts.Format, Columns: r.Columns(), Rows: [][]string{}}
	for {
		row, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("第 %d 行解析失败: %w", p.TotalRows+1, err)
		}
		if len(p.Rows) < limit {
			cells := make([]string, len(row))
			for i, v := range row {
				if v != nil {
					cells[i] = fmt.Sprint(v)
				}
			}
			p.Rows = append(p.Rows, cells)
		}
		p.TotalRows++
	}
	return p, nil
}
