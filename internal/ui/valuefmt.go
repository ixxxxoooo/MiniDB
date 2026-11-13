package ui

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"minidb/locales"
)

// 快速查看里识别出的值的种类。
const (
	kindText = iota
	kindJSON
	kindTimestamp
	kindDateTime
	kindUUID
	kindURL
	kindBase64
	kindNumber
)

// derived 是从一个值推出来的一种可读写法。
type derived struct {
	label string
	value string
}

// valueInfo 是对一个值的识别结果：它是什么，以及怎么读更方便。
type valueInfo struct {
	kind    int
	badge   string
	derived []derived
}

var dateTimeLayouts = []string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
}

// detectValue 看一个单元格的文本像什么。now 和 loc 由调用方给，便于测试。
func detectValue(text string, now time.Time, loc *time.Location) valueInfo {
	s := strings.TrimSpace(text)
	if s == "" {
		return valueInfo{}
	}
	if info, ok := detectJSON(s); ok {
		return info
	}
	if info, ok := detectTimestamp(s, now, loc); ok {
		return info
	}
	if info, ok := detectDateTime(s, now, loc); ok {
		return info
	}
	if info, ok := detectUUID(s); ok {
		return info
	}
	if info, ok := detectURL(s); ok {
		return info
	}
	if info, ok := detectNumber(s); ok {
		return info
	}
	if info, ok := detectBase64(s); ok {
		return info
	}
	return valueInfo{}
}

func detectJSON(s string) (valueInfo, bool) {
	if len(s) < 2 || (s[0] != '{' && s[0] != '[') {
		return valueInfo{}, false
	}
	var v any
	if json.Unmarshal([]byte(s), &v) != nil {
		return valueInfo{}, false
	}
	switch x := v.(type) {
	case map[string]any:
		return valueInfo{kind: kindJSON, badge: locales.QlKindJSONObject(len(x))}, true
	case []any:
		return valueInfo{kind: kindJSON, badge: locales.QlKindJSONArray(len(x))}, true
	}
	return valueInfo{}, false
}

// detectTimestamp 认 10 位（秒）、13 位（毫秒）、16 位（微秒）的 Unix 时间戳，限定在 1990 到 2100 年之间。
func detectTimestamp(s string, now time.Time, loc *time.Location) (valueInfo, bool) {
	if len(s) != 10 && len(s) != 13 && len(s) != 16 {
		return valueInfo{}, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || s[0] == '+' || s[0] == '-' {
		return valueInfo{}, false
	}
	var t time.Time
	var unit string
	switch len(s) {
	case 10:
		t, unit = time.Unix(n, 0), locales.QlUnitSeconds()
	case 13:
		t, unit = time.UnixMilli(n), locales.QlUnitMillis()
	default:
		t, unit = time.UnixMicro(n), locales.QlUnitMicros()
	}
	if y := t.UTC().Year(); y < 1990 || y > 2100 {
		return valueInfo{}, false
	}
	return valueInfo{kind: kindTimestamp, badge: locales.QlKindTimestamp(unit), derived: timeDerived(t, now, loc, false)}, true
}

func detectDateTime(s string, now time.Time, loc *time.Location) (valueInfo, bool) {
	if len(s) < 8 || len(s) > 40 || s[0] < '0' || s[0] > '9' {
		return valueInfo{}, false
	}
	for _, layout := range dateTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return valueInfo{kind: kindDateTime, badge: locales.QlKindDateTime(), derived: timeDerived(t, now, loc, true)}, true
		}
	}
	return valueInfo{}, false
}

// timeDerived 是一个时刻的各种读法。fromText 为真时（输入本来就是文本时间）补上 Unix 时间戳。
func timeDerived(t, now time.Time, loc *time.Location, fromText bool) []derived {
	out := []derived{
		{locales.QlLocal(), t.In(loc).Format("2006-01-02 15:04:05 MST")},
		{locales.QlUTC(), t.UTC().Format("2006-01-02 15:04:05")},
		{"ISO 8601", t.UTC().Format(time.RFC3339)},
		{locales.QlRelative(), relativeTime(t, now)},
	}
	if fromText {
		out = append(out,
			derived{locales.QlUnixSec(), strconv.FormatInt(t.Unix(), 10)},
			derived{locales.QlUnixMs(), strconv.FormatInt(t.UnixMilli(), 10)},
		)
	}
	return out
}

// relativeTime 把一个时刻说成"3 天前""2 小时后"。
func relativeTime(t, now time.Time) string {
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	if d < 5*time.Second {
		return locales.QlJustNow()
	}
	var text string
	switch {
	case d < time.Minute:
		text = locales.QlSeconds(int(d / time.Second))
	case d < time.Hour:
		text = locales.QlMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		text = locales.QlHours(int(d / time.Hour))
	case d < 30*24*time.Hour:
		text = locales.QlDays(int(d / (24 * time.Hour)))
	case d < 365*24*time.Hour:
		text = locales.QlMonths(int(d / (30 * 24 * time.Hour)))
	default:
		text = locales.QlYears(int(d / (365 * 24 * time.Hour)))
	}
	if future {
		return locales.QlLater(text)
	}
	return locales.QlAgo(text)
}

func detectUUID(s string) (valueInfo, bool) {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return valueInfo{}, false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return valueInfo{}, false
		}
	}
	out := []derived{
		{locales.QlUUIDVersion(), string(s[14])},
		{locales.QlNoDashes(), strings.ReplaceAll(s, "-", "")},
		{locales.QlUpper(), strings.ToUpper(s)},
	}
	return valueInfo{kind: kindUUID, badge: "UUID", derived: out}, true
}

func detectURL(s string) (valueInfo, bool) {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") || strings.ContainsAny(s, " \n\t") {
		return valueInfo{}, false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return valueInfo{}, false
	}
	out := []derived{{locales.QlScheme(), u.Scheme}, {locales.QlHost(), u.Host}}
	if u.Path != "" && u.Path != "/" {
		out = append(out, derived{locales.QlPath(), u.Path})
	}
	for k, vs := range u.Query() {
		out = append(out, derived{locales.QlParam(k), strings.Join(vs, ", ")})
	}
	if u.Fragment != "" {
		out = append(out, derived{locales.QlFragment(), u.Fragment})
	}
	// 查询参数是 map，顺序不定；按名字排好，界面才稳定
	sortDerivedFrom(out, 2)
	return valueInfo{kind: kindURL, badge: "URL", derived: out}, true
}

// sortDerivedFrom 把 out[from:] 按标签排序。
func sortDerivedFrom(out []derived, from int) {
	for i := from + 1; i < len(out); i++ {
		for j := i; j > from && out[j].label < out[j-1].label; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
}

// detectNumber 只在位数较多时才帮忙（加千分位）；小数字一眼就能读。
func detectNumber(s string) (valueInfo, bool) {
	body := strings.TrimPrefix(s, "-")
	intPart, frac, hasFrac := strings.Cut(body, ".")
	if len(intPart) < 5 || len(intPart) > 30 || (hasFrac && frac == "") {
		return valueInfo{}, false
	}
	for _, r := range intPart + frac {
		if r < '0' || r > '9' {
			return valueInfo{}, false
		}
	}
	if intPart[0] == '0' { // 前导零多半是编号、邮编，不是数量
		return valueInfo{}, false
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	text := b.String()
	if hasFrac {
		text += "." + frac
	}
	if strings.HasPrefix(s, "-") {
		text = "-" + text
	}
	out := []derived{{locales.QlThousands(), text}}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n >= 1024 {
		out = append(out, derived{locales.QlBytes(), humanBytes(n)})
	}
	return valueInfo{kind: kindNumber, badge: locales.QlKindNumber(), derived: out}, true
}

func detectBase64(s string) (valueInfo, bool) {
	if len(s) < 16 || len(s)%4 != 0 || len(s) > 1<<20 {
		return valueInfo{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil || !utf8.Valid(raw) {
		return valueInfo{}, false
	}
	text := string(raw)
	printable := 0
	total := 0
	for _, r := range text {
		total++
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			printable++
		}
	}
	if total == 0 || float64(printable)/float64(total) < 0.95 {
		return valueInfo{}, false
	}
	if len([]rune(text)) > 2000 {
		text = string([]rune(text)[:2000]) + "…"
	}
	return valueInfo{kind: kindBase64, badge: "Base64", derived: []derived{{locales.QlDecoded(), text}}}, true
}
