package app

import "testing"

func TestBusOffStopsDelivery(t *testing.T) {
	b := NewBus()
	var a, c int
	offA := b.On("t", func(any) { a++ })
	b.On("t", func(any) { c++ })
	b.Emit("t", 1)
	offA()
	offA() // 重复退订无副作用
	b.Emit("t", 2)
	if a != 1 || c != 2 {
		t.Fatalf("退订后 a 不应再收到: a=%d c=%d", a, c)
	}
}

func TestSubscribeFiltersByType(t *testing.T) {
	b := NewBus()
	var got []int
	off := Subscribe(b, "n", func(v int) { got = append(got, v) })
	b.Emit("n", 1)
	b.Emit("n", "不是 int")
	b.Emit("n", 3)
	off()
	b.Emit("n", 4)
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("got = %v", got)
	}
}
