package sqltext

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// 流式分句的结果必须和整体 Split 完全一致，不管输入怎么被切块（含一次只给 1 字节）。
func TestSplitReaderMatchesSplit(t *testing.T) {
	script := "-- 开头注释\nCREATE TABLE t(a text); /* 块注释; 里面的分号 */\n" +
		"INSERT INTO t VALUES ('a;b'), ('it''s;');\n" +
		"CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;\n" +
		"INSERT INTO t VALUES ('中文;分号');\nSELECT 1"
	want := Split(script)
	for name, wrap := range map[string]func(io.Reader) io.Reader{
		"整块":   func(r io.Reader) io.Reader { return r },
		"逐字节":  iotest.OneByteReader,
		"半块返回": iotest.HalfReader,
	} {
		var got []Statement
		err := SplitReader(wrap(strings.NewReader(script)), EditorLexOptions, 0, func(s Statement) error {
			got = append(got, s)
			return nil
		})
		if err != nil || len(got) != len(want) {
			t.Fatalf("%s: err=%v got=%d want=%d", name, err, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: 第 %d 条不一致\n got %+v\nwant %+v", name, i, got[i], want[i])
			}
		}
	}
}

// 跨过多个块的大输入：语句数量和内容都对，且单条语句超限时报错而不是攒内存。
func TestSplitReaderLargeAndLimits(t *testing.T) {
	var b strings.Builder
	const n = 60000
	for i := 0; i < n; i++ {
		b.WriteString("INSERT INTO t VALUES (1, 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx;');\n")
	}
	count := 0
	if err := SplitReader(strings.NewReader(b.String()), EditorLexOptions, 0, func(Statement) error { count++; return nil }); err != nil || count != n {
		t.Fatalf("count=%d err=%v", count, err)
	}
	huge := "SELECT '" + strings.Repeat("x", 3<<20) + "';"
	err := SplitReader(strings.NewReader(huge), EditorLexOptions, 1<<20, func(Statement) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "单条语句超过") {
		t.Fatalf("超限应报错: %v", err)
	}
	stop := errors.New("stop")
	if err := SplitReader(strings.NewReader("a;b;c"), EditorLexOptions, 0, func(Statement) error { return stop }); err != stop {
		t.Fatalf("fn 的错误应原样返回: %v", err)
	}
}
