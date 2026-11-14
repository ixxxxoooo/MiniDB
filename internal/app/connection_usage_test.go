package app

import (
	"testing"
	"time"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 连接的使用记录：固定、最近使用时间，且不能影响密码等其他字段。

func findConn(t *testing.T, s *ConnectionService, id string) model.ConnectionConfig {
	t.Helper()
	all, err := s.GetConnections()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range all {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("连接 %s 不存在", id)
	return model.ConnectionConfig{}
}

func TestSetPinnedPersistsAndKeepsOtherFields(t *testing.T) {
	s := newTestConnectionService(t)
	id := saveTestConn(t, s, "a", "")
	if findConn(t, s, id).Pinned {
		t.Fatal("新连接不该是固定的")
	}
	if err := s.SetPinned(id, true); err != nil {
		t.Fatal(err)
	}
	got := findConn(t, s, id)
	if !got.Pinned || got.Name != "a" || got.Database != ":memory:" {
		t.Fatalf("固定后其他字段应不变: %+v", got)
	}
	if err := s.SetPinned(id, false); err != nil || findConn(t, s, id).Pinned {
		t.Fatalf("取消固定失败: %v", err)
	}
	if model.CodeOf(s.SetPinned("nope", true)) != model.CodeConnNotFound {
		t.Fatal("固定不存在的连接应报 CodeConnNotFound")
	}
}

func TestConnectMarksLastUsed(t *testing.T) {
	s := newTestConnectionService(t)
	id := saveTestConn(t, s, "a", "")
	if findConn(t, s, id).LastUsed != 0 {
		t.Fatal("没连接过时 LastUsed 应为 0")
	}
	before := time.Now().Unix()
	if err := s.Connect(id); err != nil {
		t.Fatal(err)
	}
	if got := findConn(t, s, id).LastUsed; got < before {
		t.Fatalf("连接成功后应记下时间: LastUsed=%d before=%d", got, before)
	}
}
