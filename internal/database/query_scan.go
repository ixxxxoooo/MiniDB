package database

import (
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func scanRows(rows *sql.Rows, start time.Time) (*QueryResult, error) {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var columns []ColumnMeta
	for _, ct := range colTypes {
		nullable, _ := ct.Nullable()
		columns = append(columns, ColumnMeta{
			Name:     ct.Name(),
			Type:     ct.DatabaseTypeName(),
			Nullable: nullable,
		})
	}

	var resultRows []map[string]interface{}
	scanCols := make([]interface{}, len(columns))
	scanVals := make([]interface{}, len(columns))
	for i := range scanCols {
		scanCols[i] = &scanVals[i]
	}

	var scannedBytes int64
	truncated := false
	for rows.Next() {
		if len(resultRows) >= maxScanRows || scannedBytes >= maxScanBytes {
			truncated = true
			break
		}
		if err := rows.Scan(scanCols...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			v := normalizeScannedValue(scanVals[i], col.Type)
			scannedBytes += approxValueSize(v)
			row[col.Name] = v
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if resultRows == nil {
		resultRows = []map[string]interface{}{}
	}

	return &QueryResult{
		Columns:   columns,
		Rows:      resultRows,
		Total:     int64(len(resultRows)),
		Truncated: truncated,
		Duration:  time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds(),
	}, nil
}

// approxValueSize 粗略估算一个值占用的内存字节数，只用于结果集的硬上限判断。
func approxValueSize(v interface{}) int64 {
	switch x := v.(type) {
	case string:
		return int64(len(x)) + 16
	case []byte:
		return int64(len(x)) + 24
	default:
		return 16
	}
}

// maxSafeJSInteger 是 JavaScript Number 能精确表示的最大整数（2^53-1）。
const maxSafeJSInteger = 1<<53 - 1

// normalizeScannedValue 把驱动扫描出的值归一化成前端可无损使用的形态：
//   - 超出 JS 安全整数范围的整数转为字符串，避免雪花 ID 等大整数精度丢失；
//   - 非 UTF-8 的二进制内容转为 0x 十六进制文本，避免 JSON 序列化后变成乱码；
//   - 时间值按列类型格式化为固定文本，文本协议与预处理协议的输出保持一致。
func normalizeScannedValue(val interface{}, typeName string) interface{} {
	switch v := val.(type) {
	case nil:
		return nil
	case []byte:
		if strings.EqualFold(typeName, "UNIQUEIDENTIFIER") {
			if s, ok := guidString(v); ok {
				return s
			}
		}
		if strings.EqualFold(typeName, "BIT") {
			return bitBytesToString(v)
		}
		if utf8.Valid(v) {
			return string(v)
		}
		return "0x" + hex.EncodeToString(v)
	case int64:
		if v > maxSafeJSInteger || v < -maxSafeJSInteger {
			return strconv.FormatInt(v, 10)
		}
		return v
	case uint64:
		if v > maxSafeJSInteger {
			return strconv.FormatUint(v, 10)
		}
		return v
	case time.Time:
		return formatScannedTime(v, typeName)
	default:
		return normalizeOtherValue(v, typeName)
	}
}

// bitBytesToString 把 BIT 列的大端字节序值转成十进制文本。
func bitBytesToString(b []byte) string {
	if len(b) == 0 || len(b) > 8 {
		return "0x" + hex.EncodeToString(b)
	}
	var n uint64
	for _, c := range b {
		n = n<<8 | uint64(c)
	}
	return strconv.FormatUint(n, 10)
}

func formatScannedTime(t time.Time, typeName string) string {
	upper := strings.ToUpper(typeName)
	switch {
	case upper == "DATE" || upper == "DATE32":
		return t.Format("2006-01-02")
	case upper == "TIME" || upper == "TIMETZ":
		return t.Format("15:04:05.999999")
	case strings.Contains(upper, "TZ") || strings.Contains(upper, "WITH TIME ZONE") || strings.Contains(upper, "OFFSET"):
		return t.Format("2006-01-02 15:04:05.999999-07:00")
	default:
		return t.Format("2006-01-02 15:04:05.999999")
	}
}
