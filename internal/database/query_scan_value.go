package database

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// normalizeOtherValue 处理 database/sql 标准类型以外的值，主要来自 ClickHouse 驱动：
// 窄整数、float32、UUID / Decimal / IP 等带 String() 的类型、数组与 Map、可空列的指针。
// 统一成前端能直接显示的形态：整数 → int64/uint64，浮点 → float64，有 String() 的 → 文本，
// 复合值 → JSON 文本。已经是基础类型的原样返回。
func normalizeOtherValue(val any, typeName string) any {
	switch v := val.(type) {
	case bool, string, float64:
		return v
	case float32:
		// 按 32 位精度取最短表示，避免 0.1 显示成 0.10000000149011612
		f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64)
		return f
	case fmt.Stringer:
		return v.String()
	}
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return normalizeScannedValue(rv.Elem().Interface(), typeName)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
		return int64(rv.Uint())
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		if b, err := json.Marshal(val); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(val)
}
