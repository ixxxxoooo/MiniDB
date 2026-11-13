package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// 右键点在多格选区里，"复制单元格"要把整块选中的值按 TSV 复制，而不只是被点的那一格。
func TestRightClickCopiesSelectedBlock(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	for range 3 {
		e.tt.Frame()
	}
	if err := e.tt.Click("用户002"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ClickWith(mui.Shift, "user3@example.com"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if b := tab.g.selected(); len(b.rows) != 2 || len(b.cols) != 2 {
		t.Fatalf("应选中 2 行 2 列，实际 %d 行 %v", len(b.rows), b.cols)
	}
	if err := e.tt.RightClick("user3@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("复制单元格"); err != nil {
		t.Fatalf("多格选区上的右键菜单应有「复制单元格」: %v", err)
	}
	got := e.tt.Clipboard()
	want := "用户002\tuser2@example.com\n用户003\tuser3@example.com"
	if strings.TrimSpace(got) != want {
		t.Fatalf("剪贴板内容不对:\n%q\n想要:\n%q", got, want)
	}
}

// 只选了一格时没有"复制单元格"，只有"复制值"。
func TestRightClickSingleCellHasNoBlockCopy(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	for range 3 {
		e.tt.Frame()
	}
	if err := e.tt.Click("user2@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.RightClick("user2@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.tt.ChooseMenuItem("复制单元格"); err == nil {
		t.Fatal("单格选区不应出现「复制单元格」")
	}
	e.tt.CloseMenu()
}
