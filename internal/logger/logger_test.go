package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"minidb/internal/appdata"
)

func TestCleanupOldLogsOnlyRemovesExpiredDatedLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := appdata.LogsRootPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -30).Format("2006-01-02") + ".log"
	recent := time.Now().AddDate(0, 0, -2).Format("2006-01-02") + ".log"
	for _, name := range []string{old, recent, "update.log", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cleanupOldLogs(14)
	for name, wantExist := range map[string]bool{old: false, recent: true, "update.log": true, "notes.txt": true} {
		_, err := os.Stat(filepath.Join(dir, name))
		if (err == nil) != wantExist {
			t.Errorf("%s exists=%v want %v", name, err == nil, wantExist)
		}
	}
}

func TestRotatesWhenDayChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	defer Close()
	mu.Lock()
	logFileDay = "2000-01-01" // 模拟文件属于过去的某一天
	mu.Unlock()
	Info("after rotation")
	want := filepath.Join(appdata.LogsRootPath(), time.Now().Format("2006-01-02")+".log")
	if got := CurrentPath(); got != want {
		t.Fatalf("CurrentPath=%s want %s", got, want)
	}
}
