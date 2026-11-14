package export

import (
	"bytes"
	"encoding/json"

	"io"

	"time"
)

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
