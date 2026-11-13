package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupDialogSQLiteCopy(t *testing.T) {
	e := newEnv(t)
	out := filepath.Join(t.TempDir(), "users-backup.sqlite")
	e.sh.app = rebuildWithHost(e, fakeHost{save: out})
	openWorkspace(e)

	e.sh.openBackup()
	e.tt.Frame()
	e.has("备份数据库")
	e.has("SQLite 的备份是数据库文件的一份副本")
	e.shot("51-backup")
	e.sh.startBackup()
	for i := 0; i < 300; i++ {
		if st, err := os.Stat(out); err == nil && st.Size() > 0 {
			break
		}
		sleepMs(10)
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatalf("没有生成备份文件: %v", err)
	}
	if !e.sh.bak.open && e.sh.bak.err != "" {
		t.Fatalf("备份出错: %s", e.sh.bak.err)
	}
}

func TestRestoreDialogRefusesSQLiteAndAsksFirst(t *testing.T) {
	e := newEnv(t)
	dump := filepath.Join(t.TempDir(), "d.sql")
	os.WriteFile(dump, []byte("select 1;"), 0o600)
	e.sh.app = rebuildWithHost(e, fakeHost{open: dump})
	openWorkspace(e)

	e.sh.openRestore()
	e.tt.Frame()
	e.sh.confirmRestore() // 没选文件
	e.tt.Frame()
	e.has("请先选择备份文件")
	e.sh.pickRestoreFile()
	e.tt.Frame()
	e.sh.rst.newDB = true
	e.sh.rst.newName = "copy"
	e.tt.Frame()
	e.shot("52-restore")
	e.sh.confirmRestore()
	e.tt.Frame()
	e.has("还原")
	if !e.sh.confirm.open {
		t.Fatal("还原前应先确认")
	}
	e.sh.confirm.action()
	for i := 0; i < 100 && e.sh.rst.err == ""; i++ {
		sleepMs(10)
	}
	if e.sh.rst.err == "" {
		t.Fatal("SQLite 不能用工具还原，应给出提示")
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"} {
		if got := humanBytes(in); got != want {
			t.Errorf("%d → %s，应为 %s", in, got, want)
		}
	}
}
