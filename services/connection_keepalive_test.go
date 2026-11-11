package services

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"minidb/internal/database"
)

type eventSink struct {
	mu     sync.Mutex
	events []ConnectionStatusEvent
}

func (e *eventSink) emit(ev ConnectionStatusEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
}

func TestKeepaliveReconnectsLostConnectionAndReportsTransitions(t *testing.T) {
	svc := newTestConnectionService(t)
	cfg := database.ConnectionConfig{ID: "k1", Name: "k", Type: "sqlite", Database: filepath.Join(t.TempDir(), "k.db")}
	if err := svc.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	if ok, msg := svc.Connect("k1"); !ok {
		t.Fatalf("连接失败: %s", msg)
	}
	defer svc.manager.CloseAll()

	sink := &eventSink{}
	k := newKeepalive(svc, sink.emit)

	// 健康连接：不产生任何事件
	k.checkOnce(context.Background())
	if len(sink.events) != 0 {
		t.Fatalf("健康连接不应发事件: %+v", sink.events)
	}

	// 模拟连接被中断：直接关闭底层连接池
	db, _ := svc.manager.GetDB("k1")
	db.Close()

	k.checkOnce(context.Background())
	if len(sink.events) != 2 || sink.events[0].Status != "lost" || sink.events[1].Status != "connected" {
		t.Fatalf("应依次发出 lost -> connected，实际 %+v", sink.events)
	}
	// 重连后连接真的可用
	if err := svc.manager.Ping(context.Background(), "k1"); err != nil {
		t.Fatalf("重连后应可用: %v", err)
	}
	// 再探测一次：已恢复，不重复发事件
	k.checkOnce(context.Background())
	if len(sink.events) != 2 {
		t.Fatalf("恢复后不应重复发事件: %+v", sink.events)
	}
}

func TestReconnectDoesNotResurrectDisconnectedConnection(t *testing.T) {
	svc := newTestConnectionService(t)
	cfg := database.ConnectionConfig{ID: "k2", Name: "k", Type: "sqlite", Database: filepath.Join(t.TempDir(), "k2.db")}
	if err := svc.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	if ok, msg := svc.Connect("k2"); !ok {
		t.Fatal(msg)
	}
	svc.manager.Disconnect("k2") // 用户手动断开
	if err := svc.reconnect("k2"); err != database.ErrDisconnected {
		t.Fatalf("已断开的连接不应被自动重连复活，实际 err=%v", err)
	}
	if len(svc.manager.ConnectionIDs()) != 0 {
		t.Fatal("连接不应重新出现")
	}
}
