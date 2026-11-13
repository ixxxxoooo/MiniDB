package ui

import (
	"strings"
	"testing"
	"time"
)

func derivedMap(v valueInfo) map[string]string {
	m := map[string]string{}
	for _, d := range v.derived {
		m[d.label] = d.value
	}
	return m
}

func TestDetectValue(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		text string
		kind int
	}{
		{`{"a":1}`, kindJSON}, {`[1,2]`, kindJSON}, {`{bad`, kindText},
		{"1700000000", kindTimestamp}, {"1700000000123", kindTimestamp}, {"1700000000123456", kindTimestamp},
		{"1234567890123456789", kindNumber}, {"123456", kindNumber}, {"12", kindText}, {"00123456", kindText},
		{"2026-11-13 08:30:00", kindDateTime}, {"2026-11-13", kindDateTime}, {"2026-11-13T08:30:00Z", kindDateTime},
		{"550e8400-e29b-41d4-a716-446655440000", kindUUID},
		{"https://example.com/a/b?x=1&y=2", kindURL}, {"example.com", kindText},
		{"5Lit5paH5rWL6K+V5pWw5o2u", kindBase64}, {"hello world", kindText}, {"", kindText},
		{"9999999999", kindNumber}, // 超出 1990–2100 的 10 位数不当时间戳
	}
	for _, c := range cases {
		if got := detectValue(c.text, now, loc).kind; got != c.kind {
			t.Errorf("detectValue(%q).kind = %d，应为 %d", c.text, got, c.kind)
		}
	}

	ts := derivedMap(detectValue("1700000000", now, loc))
	if ts["本地时间"] != "2023-11-15 06:13:20 CST" || ts["UTC 时间"] != "2023-11-14 22:13:20" {
		t.Errorf("时间戳换算不对: %v", ts)
	}
	if !strings.HasSuffix(ts["相对现在"], "前") {
		t.Errorf("相对时间: %q", ts["相对现在"])
	}
	dt := derivedMap(detectValue("2026-11-13 20:00:00", now, loc))
	if dt["Unix 秒"] != "1794571200" || dt["UTC 时间"] != "2026-11-13 12:00:00" || dt["相对现在"] != "刚刚" {
		t.Errorf("日期时间换算不对: %v", dt)
	}
	if got := derivedMap(detectValue("1234567", now, loc))["千分位"]; got != "1,234,567" {
		t.Errorf("千分位 %q", got)
	}
	if got := derivedMap(detectValue("5Lit5paH5rWL6K+V5pWw5o2u", now, loc))["Base64 解码"]; got != "中文测试数据" {
		t.Errorf("base64 解码 %q", got)
	}
	u := derivedMap(detectValue("https://example.com/a/b?x=1&y=2", now, loc))
	if u["主机"] != "example.com" || u["路径"] != "/a/b" || u["参数 x"] != "1" {
		t.Errorf("URL: %v", u)
	}
	if got := detectValue(`{"a":1,"b":2}`, now, loc).badge; got != "JSON 对象 · 2 个键" {
		t.Errorf("badge %q", got)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for d, want := range map[time.Duration]string{
		-2 * time.Second: "刚刚", -90 * time.Second: "1 分钟前", -3 * time.Hour: "3 小时前",
		-48 * time.Hour: "2 天前", 26 * time.Hour: "1 天后", -400 * 24 * time.Hour: "1 年前",
	} {
		if got := relativeTime(now.Add(d), now); got != want {
			t.Errorf("relativeTime(%v)=%q 应为 %q", d, got, want)
		}
	}
}
