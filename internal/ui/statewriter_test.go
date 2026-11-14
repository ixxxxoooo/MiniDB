package ui

import (
	"sync"
	"testing"
)

// 后台写得慢时，同一个键排队的旧写入被新的取代；do 不会因为队列满而阻塞。
func TestStateWriterCoalescesSameKey(t *testing.T) {
	var w stateWriter
	gate := make(chan struct{})
	started := make(chan struct{})
	var mu sync.Mutex
	var ran []string
	rec := func(s string) func() {
		return func() { mu.Lock(); ran = append(ran, s); mu.Unlock() }
	}
	// 第一个写入卡住后台协程，让后面的都留在队列里
	w.do("block", func() { close(started); <-gate })
	<-started
	for i := 0; i < 500; i++ { // 远多于旧实现 64 的队列长度：旧实现会在这里阻塞
		w.do("a", rec("a-old"))
	}
	w.do("a", rec("a-new"))
	w.do("b", rec("b"))
	close(gate)
	w.wait()
	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 2 || ran[0] != "a-new" || ran[1] != "b" {
		t.Fatalf("同键只应保留最新一条，且保持先后顺序: %v", ran)
	}
}

func TestStateWriterWaitOnIdle(t *testing.T) {
	var w stateWriter
	w.wait() // 从没写过：不能卡住
	w.do("k", func() {})
	w.wait()
}
