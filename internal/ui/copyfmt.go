package ui

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// Formats the grid can copy a block of cells in.
const (
	fmtTSV = iota
	fmtCSV
	fmtJSON
	fmtInsert
	fmtMarkdown
)

var copyFormatNames = []string{"TSV", "CSV", "JSON", "SQL INSERT", "Markdown"}

// blockOf picks the given columns of rows, in order.
type block struct {
	cols []string
	rows []map[string]any
}

func plain(v any) string {
	s, null := cellText(v)
	if null {
		return ""
	}
	return s
}

func (b block) format(kind int, dbType model.DBType, table string, header bool) string {
	switch kind {
	case fmtCSV:
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if header {
			_ = w.Write(b.cols)
		}
		for _, r := range b.rows {
			rec := make([]string, len(b.cols))
			for i, c := range b.cols {
				rec[i] = plain(r[c])
			}
			_ = w.Write(rec)
		}
		w.Flush()
		return buf.String()
	case fmtJSON:
		out := make([]map[string]any, 0, len(b.rows))
		for _, r := range b.rows {
			m := make(map[string]any, len(b.cols))
			for _, c := range b.cols {
				m[c] = jsonValue(r[c])
			}
			out = append(out, m)
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return string(data)
	case fmtInsert:
		var sb strings.Builder
		quoted := make([]string, len(b.cols))
		for i, c := range b.cols {
			quoted[i] = app.QuoteIdent(dbType, c)
		}
		for _, r := range b.rows {
			vals := make([]string, len(b.cols))
			for i, c := range b.cols {
				vals[i] = sqlLiteral(r[c])
			}
			fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES (%s);\n", app.QuoteIdent(dbType, table), strings.Join(quoted, ", "), strings.Join(vals, ", "))
		}
		return sb.String()
	case fmtMarkdown:
		var sb strings.Builder
		esc := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "\n", " ") }
		sb.WriteString("| " + strings.Join(mapStrings(b.cols, esc), " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat(" --- |", len(b.cols)) + "\n")
		for _, r := range b.rows {
			cells := make([]string, len(b.cols))
			for i, c := range b.cols {
				cells[i] = esc(plain(r[c]))
			}
			sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		}
		return sb.String()
	}
	// TSV
	var sb strings.Builder
	tsv := func(s string) string { return strings.NewReplacer("\t", " ", "\n", " ", "\r", "").Replace(s) }
	if header {
		sb.WriteString(strings.Join(mapStrings(b.cols, tsv), "\t") + "\n")
	}
	for _, r := range b.rows {
		cells := make([]string, len(b.cols))
		for i, c := range b.cols {
			cells[i] = tsv(plain(r[c]))
		}
		sb.WriteString(strings.Join(cells, "\t") + "\n")
	}
	return sb.String()
}

func mapStrings(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

func jsonValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

// sqlLiteral renders a value as a SQL literal.
func sqlLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if x {
			return "1"
		}
		return "0"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x)
	case float32, float64:
		s, _ := cellText(x)
		return s
	}
	s, _ := cellText(v)
	return sqltext.QuoteString(s)
}

// copyCellFormats 是"将值复制为"的几种写法的名字，顺序与 cellCopyText 的 kind 一致。
func copyCellFormats() []string {
	return []string{locales.TableCopyJSONValue(), locales.TableCopySQLLiteral(), locales.TableCopyWhere(), locales.TableCopyColumnName()}
}

// cellCopyText 把一个单元格按 kind 写成文本：0 JSON 值，1 SQL 字面量，2 WHERE 条件（`列` = 值，空值是 IS NULL），3 列名。
// 参数 dbType 决定标识符的引号，col 是列名，v 是单元格的值；返回要放进剪贴板的文本。
func cellCopyText(kind int, dbType model.DBType, col string, v any) string {
	switch kind {
	case 0:
		data, _ := json.Marshal(jsonValue(v))
		return string(data)
	case 1:
		return sqlLiteral(v)
	case 2:
		if v == nil {
			return app.QuoteIdent(dbType, col) + " IS NULL"
		}
		return app.QuoteIdent(dbType, col) + " = " + sqlLiteral(v)
	}
	return col
}
