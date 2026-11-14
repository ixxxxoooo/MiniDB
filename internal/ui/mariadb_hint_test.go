package ui

import (
	"testing"

	"minidb/internal/model"
)

// 服务器是 MariaDB、连接类型还是 MySQL 时只提示一次；类型已经对了或不是 MariaDB 不提示。
func TestMariaDBHintOnce(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.cfg.Type = model.MySQL
	w.setVersion("10.11.6-MariaDB-1:10.11.6+maria~ubu2204")
	if len(e.sh.toastActs) != 1 {
		t.Fatalf("应提示一次: %d", len(e.sh.toastActs))
	}
	w.setVersion("10.11.6-MariaDB")
	if len(e.sh.toastActs) != 1 {
		t.Fatalf("同一个连接不应重复提示: %d", len(e.sh.toastActs))
	}

	w.cfg.ID = "other"
	w.cfg.Type = model.MariaDB
	w.setVersion("10.11.6-MariaDB")
	w.cfg.Type = model.MySQL
	w.setVersion("8.4.0")
	if len(e.sh.toastActs) != 1 {
		t.Fatalf("类型已对或不是 MariaDB 都不提示: %d", len(e.sh.toastActs))
	}
}
