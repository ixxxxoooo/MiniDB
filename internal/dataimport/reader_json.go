package dataimport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

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
