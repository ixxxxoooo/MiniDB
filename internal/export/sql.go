package export

import (
	"fmt"
	"io"
	"minidb/internal/sqltext"

	"strings"
)

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
