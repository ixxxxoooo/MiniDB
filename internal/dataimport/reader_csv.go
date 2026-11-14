package dataimport

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"

	"errors"
	"fmt"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
