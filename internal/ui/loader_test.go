package ui

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"
)

func TestLoaderDropsStaleResponse(t *testing.T) {
	var l loader[string]
	first := l.begin()  // 慢的旧请求
	second := l.begin() // 快的新请求
	if !l.finish(second, "新", nil) || l.Val != "新" || l.Loading {
		t.Fatalf("新请求先回来应生效: %+v", l)
	}
	if l.finish(first, "旧", nil) || l.Val != "新" {
		t.Fatalf("旧请求后到应被丢弃，Val = %q", l.Val)
	}
}

func TestLoaderLoadRunsDoneOnce(t *testing.T) {
	var l loader[string]
	up := uiThread{win: func() *mygo.Window { return nil }, sync: true}
	var applied []string
	l.load(up, func() (string, error) { return "旧", nil }, func(v string, _ error) { applied = append(applied, v) })
	l.load(up, func() (string, error) { return "新", nil }, func(v string, _ error) { applied = append(applied, v) })
	if l.Val != "新" || len(applied) != 2 || l.Loading {
		t.Fatalf("%+v applied=%v", l, applied)
	}
}

func TestLoaderSeqInvalidatesInFlight(t *testing.T) {
	var l loader[int]
	l.seq = 5
	mine := l.seq
	l.cancel()
	if mine == l.seq {
		t.Fatal("cancel 应使在途的加载作废")
	}
}

func TestLoaderKeepsValueOnError(t *testing.T) {
	var l loader[int]
	up := uiThread{win: func() *mygo.Window { return nil }, sync: true}
	l.load(up, func() (int, error) { return 7, nil }, nil)
	l.load(up, func() (int, error) { return 0, errors.New("boom") }, nil)
	if l.Val != 7 || l.Err != "boom" || l.Loading {
		t.Fatalf("出错时保留旧值并记下错误: %+v", l)
	}
}
