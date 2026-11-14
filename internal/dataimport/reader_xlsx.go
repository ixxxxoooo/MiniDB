package dataimport

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// xlsxReader 用 excelize 的流式行迭代器读一张工作表，不会把整张表读进内存。
// 单元格取的是 Excel 里显示的文字（日期、百分比等按单元格格式），空单元格给 NULL；
// 整行为空的行（Excel 里常见的空白尾行）直接跳过。
type xlsxReader struct {
	f       *excelize.File
	rows    *excelize.Rows
	columns []string
	pending []string // 无表头时，已读出用于确定列数的首行
}

// XLSXSheets 返回工作簿里全部工作表的名字（按工作簿顺序）。
func XLSXSheets(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Excel 失败: %w", err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

func newXLSXReader(path string, opts Options) (*xlsxReader, error) {
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		return nil, errors.New("Excel 文件不支持压缩，请先解压")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Excel 失败（仅支持 .xlsx，旧版 .xls 请先另存为 .xlsx）: %w", err)
	}
	sheet := opts.Sheet
	if sheet == "" {
		list := f.GetSheetList()
		if len(list) == 0 {
			_ = f.Close()
			return nil, errors.New("工作簿里没有工作表")
		}
		sheet = list[0]
	}
	rows, err := f.Rows(sheet)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
	}
	rd := &xlsxReader{f: f, rows: rows}
	first, err := rd.nextRecord()
	if err == io.EOF {
		_ = rd.Close()
		return nil, errors.New("工作表为空")
	}
	if err != nil {
		_ = rd.Close()
		return nil, err
	}
	first = trimTrailingEmpty(first)
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

// nextRecord 读下一条非空行；只含空单元格的行会被跳过。
func (x *xlsxReader) nextRecord() ([]string, error) {
	for x.rows.Next() {
		rec, err := x.rows.Columns()
		if err != nil {
			return nil, err
		}
		if len(trimTrailingEmpty(rec)) > 0 {
			return rec, nil
		}
	}
	if err := x.rows.Error(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// trimTrailingEmpty 去掉行尾的空白单元格。
func trimTrailingEmpty(rec []string) []string {
	n := len(rec)
	for n > 0 && strings.TrimSpace(rec[n-1]) == "" {
		n--
	}
	return rec[:n]
}

func (x *xlsxReader) Columns() []string { return x.columns }

func (x *xlsxReader) Next() ([]interface{}, error) {
	rec := x.pending
	if rec != nil {
		x.pending = nil
	} else {
		var err error
		if rec, err = x.nextRecord(); err != nil {
			return nil, err
		}
	}
	row := make([]interface{}, len(x.columns))
	for i := range row {
		if i < len(rec) && rec[i] != "" {
			row[i] = rec[i]
		}
	}
	return row, nil
}

func (x *xlsxReader) Close() error {
	_ = x.rows.Close()
	return x.f.Close()
}
