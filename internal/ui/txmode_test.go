package ui

import (
	"slices"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// settleFrames 让后台任务（测试里同步执行）和随后的重画走完。
func (e *env) settleFrames() {
	e.tt.Frame()
	e.tt.Frame()
}

// 手动提交：开关、写入后出现未提交条数、回滚/提交按钮可用，且数据真的按预期回滚或落盘。
func TestManualTransactionFlow(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.settleFrames()
	e.shot("70-tx-off")

	// 命令面板里也能找到
	e.sh.cmdTxToggle()
	e.settleFrames()
	if st := w.txStatus(); !st.Manual {
		t.Fatalf("应已开启手动提交: %+v", st)
	}
	e.has("已开启手动提交")

	q := w.newQuery("INSERT INTO users (name, email, age) VALUES ('tx', 'tx@example.com', 9)")
	e.tt.Frame()
	q.run(true)
	e.settleFrames()
	if st := w.txStatus(); !st.Active || st.Pending != 1 {
		t.Fatalf("执行写语句后应有 1 条未提交: %+v failed=%q", st, q.failed)
	}
	e.shot("71-tx-pending")

	// ⌘⌥Z 回滚
	time.Sleep(200 * time.Millisecond)
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyZ)
	e.settleFrames()
	if st := w.txStatus(); st.Active || st.Pending != 0 {
		t.Fatalf("回滚后应无事务: %+v", st)
	}
	e.has("事务已回滚")

	// 再写一条，用 ⌘S 兜底提交
	q.run(true)
	e.settleFrames()
	if !w.txStatus().Active {
		t.Fatal("再次写入后应有事务")
	}
	time.Sleep(200 * time.Millisecond)
	e.tt.Key(mui.Cmd, mui.KeyS)
	e.settleFrames()
	if w.txStatus().Active {
		t.Fatal("⌘S 应提交手动事务")
	}
	e.has("事务已提交")

	// 关闭手动提交
	e.sh.cmdTxToggle()
	e.settleFrames()
	if w.txStatus().Manual {
		t.Fatal("应已关闭手动提交")
	}

	// 控制台日志要能看到事务边界：隐式的 BEGIN、回滚、再来一次 BEGIN、提交（BEGIN 在 SQLite 上就是 "BEGIN"）
	var got []string
	for _, en := range e.sh.console {
		switch en.sql {
		case "BEGIN", "COMMIT", "ROLLBACK":
			got = append(got, en.sql)
		}
	}
	if want := []string{"BEGIN", "ROLLBACK", "BEGIN", "COMMIT"}; !slices.Equal(got, want) {
		t.Fatalf("控制台里的事务语句应为 %v，实际 %v", want, got)
	}
}

// 有未提交更改时：关闭手动提交先确认；断开连接前也要确认。
func TestManualTransactionGuards(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.cmdTxToggle()
	e.settleFrames()
	q := w.newQuery("INSERT INTO users (name, email, age) VALUES ('g', 'g@example.com', 1)")
	e.tt.Frame()
	q.run(true)
	e.settleFrames()
	if w.txPendingTotal() != 1 {
		t.Fatalf("应有 1 条未提交: %d", w.txPendingTotal())
	}
	e.sh.cmdTxToggle()
	e.tt.Frame()
	e.has("关闭手动提交？")
	e.has("提交并关闭")
	e.shot("72-tx-close-confirm")
	e.tt.Click("提交并关闭")
	e.settleFrames()
	if w.txStatus().Manual {
		t.Fatal("确认后应已提交并关闭")
	}
}
