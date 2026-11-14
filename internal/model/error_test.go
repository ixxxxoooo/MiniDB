package model

import (
	"errors"
	"testing"
)

func TestErrf(t *testing.T) {
	e := Errf(CodeConnNotFound, "连接不存在: %s", "c1")
	if e.Error() != "连接不存在: c1" || e.Code != CodeConnNotFound || len(e.Args) != 1 || e.Args[0] != "c1" {
		t.Fatalf("Errf 结果不对: %+v", e)
	}
}

func TestWrapfKeepsCause(t *testing.T) {
	cause := errors.New("boom")
	e := Wrapf(CodeDBOpenFailed, "打开数据库失败: %w", cause)
	if e.Error() != "打开数据库失败: boom" {
		t.Fatalf("原文不对: %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is 应能穿过带码错误")
	}
	if len(e.Args) != 1 || e.Args[0] != "boom" {
		t.Fatalf("Args 里底层错误应是文字: %v", e.Args)
	}
	if CodeOf(e) != CodeDBOpenFailed || CodeOf(cause) != "" {
		t.Fatal("CodeOf 不对")
	}
}
