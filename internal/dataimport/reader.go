// Package dataimport 解析导入文件（CSV/TSV/JSON/JSON Lines），以流式方式逐行产出，
// 不会把整个文件读进内存。写库逻辑见 database.ImportRows。
package dataimport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// ---------- JSON / JSON Lines ----------

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
