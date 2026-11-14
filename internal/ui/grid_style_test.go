package ui

import (
	"testing"

	"minidb/locales"
)

// 表里有没提交的修改时，内容区底部浮出一条：数量、分项、查看 SQL、丢弃、提交；点「丢弃」后条消失。
func TestPendingFloatShowsBreakdownAndDiscards(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.hasNot(locales.PendingBarTitle(1)) // 没有修改：完全不占位

	tab.setCell(0, "name", "改过")
	tab.addRow()
	tab.g.chosen.Clear()
	tab.g.chosen.Add(2)
	tab.deleteSelected()
	e.tt.Frame()
	n := tab.pendingCount()
	e.has(locales.PendingBarTitle(n))
	e.has(locales.PendingBarMod("1") + " · " + locales.PendingBarAdd("1") + " · " + locales.PendingBarDel("1"))
	e.shot("p4-pending-bar")

	if err := e.tt.Click(locales.TableDiscard()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if tab.pendingCount() != 0 {
		t.Fatalf("点丢弃应清掉所有修改，剩 %d", tab.pendingCount())
	}
	e.hasNot(locales.PendingBarTitle(n))
}

// 底栏不再有丢弃 / 提交：它们只在悬浮条里（只出现一处）。
func TestBottomBarHasNoCommitButtons(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "name", "改过")
	e.tt.Frame()
	count := 0
	for _, s := range e.tt.Texts() {
		if s == locales.TableDiscard() {
			count++
		}
	}
	if count != 2 { // 一个按钮在文字清单里出现两次：无障碍标签 + 按钮文字
		t.Fatalf("「丢弃」应只有悬浮条里的一个按钮，实际文字出现 %d 次", count)
	}
}

// 数值列靠右，其余靠左；设置里关掉后全部靠左。
func TestNumericColumnsAlignRight(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	idx := map[string]int{}
	for k, i := range tab.g.vis {
		idx[tab.g.cols[i].Name] = k + 1
	}
	if !tab.g.numEnd(idx["id"]) || !tab.g.numEnd(idx["age"]) {
		t.Fatal("整数列应靠右")
	}
	if tab.g.numEnd(idx["name"]) || tab.g.numEnd(0) {
		t.Fatal("文本列和行号不应靠右")
	}
	numRightAlign = false
	defer func() { numRightAlign = true }()
	if tab.g.numEnd(idx["id"]) {
		t.Fatal("关掉「数字右对齐」后应靠左")
	}
}

// 行预览停靠栏：头部是「行 N」和字段 / JSON 分段开关，默认宽度 300。
func TestRowPaneHeadAndJSONTab(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 115)
	tab.togglePreview()
	e.tt.Frame()
	e.tt.Frame()
	if tab.previewW != sizeRowPane {
		t.Fatalf("停靠栏默认宽度应为 %d，实际 %v", sizeRowPane, tab.previewW)
	}
	e.has(locales.PreviewRowTitle("1"))
	if err := e.tt.Click(locales.PreviewTabJSON()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if !tab.pv.asJSON {
		t.Fatal("点 JSON 应切到整行 JSON 视图")
	}
	e.shot("p4-rowpane-json")
}
