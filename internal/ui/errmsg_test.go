package ui

import (
	"errors"
	"fmt"
	"testing"

	"minidb/internal/model"
)

func TestErrMessageLocalizes(t *testing.T) {
	err := fmt.Errorf("外层: %w", model.Errf(model.CodeConnNotFound, "连接不存在: %s", "c1"))
	if got := errMessageIn("en", err); got != "Connection not found: c1" {
		t.Fatalf("英文 = %q", got)
	}
	if got := errMessageIn("zh-Hans", err); got != "连接不存在：c1" {
		t.Fatalf("中文 = %q", got)
	}

	wrapped := model.Wrapf(model.CodeDBOpenFailed, "打开数据库失败: %w", errors.New("boom"))
	if got := errMessageIn("en", wrapped); got != "Could not open the database: boom" {
		t.Fatalf("带底层错误 = %q", got)
	}
}

func TestErrMessageFallsBack(t *testing.T) {
	if got := errMessage(errors.New("driver: bad")); got != "driver: bad" {
		t.Fatalf("无码错误应原样显示: %q", got)
	}
	unknown := model.Errf("noSuchCodeXYZ", "原文 %s", "x")
	if got := errMessage(unknown); got != "原文 x" {
		t.Fatalf("词条缺失应退回原文: %q", got)
	}
	if errMessage(nil) != "" {
		t.Fatal("nil 应返回空串")
	}
}
