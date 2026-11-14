package ui

import "testing"

// 命令只在 commands 里登记一次，动作只在 commandRuns 里登记一次，两边必须一一对应：
// 加了命令忘写动作（快捷键、菜单会没反应），或动作留着已删除的命令，都在这里失败。
func TestEveryCommandIsWired(t *testing.T) {
	e := newEnv(t)
	runs := e.sh.commandRuns()
	ids := map[string]bool{}
	for _, c := range commands {
		ids[c.id] = true
		if runs[c.id] == nil {
			t.Errorf("命令 %q 没有对应的动作", c.id)
		}
	}
	for id := range runs {
		if !ids[id] {
			t.Errorf("动作 %q 没有登记在 commands 里", id)
		}
	}
	for id := range editorHandled {
		if !ids[id] {
			t.Errorf("editorHandled 里的 %q 不是已登记的命令", id)
		}
	}
}
