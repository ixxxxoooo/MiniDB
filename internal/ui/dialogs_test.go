package ui

import (
	"reflect"
	"sort"
	"testing"
)

// dialogs 里每个对话框状态都必须登记在 Shell.modals 里，否则它既不会被画出来，
// 打开时也挡不住全局快捷键。
func TestEveryDialogIsRegistered(t *testing.T) {
	e := newEnv(t)
	var fields []string
	ty := reflect.TypeOf(dialogs{})
	for i := 0; i < ty.NumField(); i++ {
		if n := ty.Field(i).Name; n != "modalList" {
			fields = append(fields, n)
		}
	}
	var registered []string
	for _, m := range e.sh.modals() {
		registered = append(registered, m.name())
	}
	sort.Strings(fields)
	sort.Strings(registered)
	if !reflect.DeepEqual(fields, registered) {
		t.Fatalf("对话框登记与 dialogs 字段不一致:\n字段: %v\n登记: %v", fields, registered)
	}
}

func TestModalOpenFollowsRegistry(t *testing.T) {
	e := newEnv(t)
	if e.sh.modalOpen() {
		t.Fatal("刚启动不应有对话框打开")
	}
	e.sh.pal.open = true
	if !e.sh.modalOpen() || !e.sh.anyModalOpen() {
		t.Fatal("命令面板打开后应占住键盘")
	}
	e.sh.pal.open = false
	e.sh.notesOpen = true
	if !e.sh.modalOpen() || e.sh.anyModalOpen() {
		t.Fatal("通知中心占键盘，但不属于对话框清单")
	}
}
