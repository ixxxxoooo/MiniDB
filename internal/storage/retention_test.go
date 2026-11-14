package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 同一连接下的会话超过上限时，淘汰最久没更新的会话和它们的消息；别的连接不受影响。
func TestSaveAISessionEvictsOldestPerConnection(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	base := time.Now().Add(-time.Hour)
	// 直接写元数据，绕开 SaveAISession 里的 UpdatedAt = now，制造出确定的新旧顺序
	for i := 0; i < maxAISessionsPerDB; i++ {
		id := fmt.Sprintf("s%03d", i)
		if err := store.Put("ai_sessions", "meta:"+id, AISession{ID: id, ConnectionID: "c1", UpdatedAt: base.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.AppendAISessionMessage("s000", AISessionMessage{ID: "m1", Role: "user"}); err != nil {
		t.Fatal(err)
	}
	// 追加消息会刷新更新时间，这里再把它改回最旧
	if err := store.Put("ai_sessions", "meta:s000", AISession{ID: "s000", ConnectionID: "c1", UpdatedAt: base.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("ai_sessions", "meta:other", AISession{ID: "other", ConnectionID: "c2", UpdatedAt: base.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveAISession(AISession{ID: "fresh", ConnectionID: "c1"}); err != nil {
		t.Fatal(err)
	}
	mine, _ := store.ListAISessions("c1", 0)
	if len(mine) != maxAISessionsPerDB {
		t.Fatalf("c1 应保留 %d 个会话, 实际 %d", maxAISessionsPerDB, len(mine))
	}
	if _, err := store.GetAISession("s000"); err == nil {
		t.Fatal("最旧的会话应被淘汰")
	}
	if msgs, _ := store.GetAISessionMessages("s000", 0); len(msgs) != 0 {
		t.Fatal("被淘汰会话的消息也要一起删")
	}
	if _, err := store.GetAISession("other"); err != nil {
		t.Fatal("别的连接的会话不能被淘汰")
	}
}

func TestAppendAISessionMessageCapsMessages(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	if err := store.SaveAISession(AISession{ID: "s"}); err != nil {
		t.Fatal(err)
	}
	total := maxAIMessagesPerSession + 25
	for i := 0; i < total; i++ {
		if err := store.AppendAISessionMessage("s", AISessionMessage{ID: fmt.Sprintf("m%05d", i), Role: "user"}); err != nil {
			t.Fatal(err)
		}
	}
	msgs, err := store.GetAISessionMessages("s", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != maxAIMessagesPerSession {
		t.Fatalf("消息数应被限制在 %d, 实际 %d", maxAIMessagesPerSession, len(msgs))
	}
	if msgs[0].ID != "m00025" || msgs[len(msgs)-1].ID != fmt.Sprintf("m%05d", total-1) {
		t.Fatalf("应淘汰最早的消息, 保留最新的: 首=%s 尾=%s", msgs[0].ID, msgs[len(msgs)-1].ID)
	}
	if meta, _ := store.GetAISession("s"); meta.MessageCount != maxAIMessagesPerSession {
		t.Fatalf("计数应与实际一致: %d", meta.MessageCount)
	}
}

func TestMigrateRecordsVersionAndRejectsNewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	var v int
	if err := store.Get(metaBucket, metaSchemaVersion, &v); err != nil || v != currentSchemaVersion() {
		t.Fatalf("新库应记录最新版本 %d: v=%d err=%v", currentSchemaVersion(), v, err)
	}
	// 重复打开不会重复迁移，也不会报错
	store.Close()
	store, err = OpenStore(path)
	if err != nil {
		t.Fatalf("重复打开失败: %v", err)
	}
	// 模拟用户降级：库的版本比程序新
	if err := store.Put(metaBucket, metaSchemaVersion, currentSchemaVersion()+1); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if _, err := OpenStore(path); err == nil {
		t.Fatal("库版本比程序新时应拒绝打开")
	}
}

func TestOpenStoreFilesArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.db")
	// 模拟旧版本留下的宽松权限
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Put("b", "k", 1); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(path + suffix)
		if err != nil {
			continue // 不是每个环境都会留下 -wal/-shm
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s 不应对其他用户开放: %v", path+suffix, info.Mode().Perm())
		}
	}
}
