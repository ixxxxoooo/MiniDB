package ui

import (
	"testing"

	"minidb/internal/app"
)

// @author ixxxxoooo
//
// 监控概览把「每秒收到」「每秒发出」合并成一张「网络流量」卡。

func TestWithNetworkSumsAndDoesNotMutate(t *testing.T) {
	pts := []app.MonitorPoint{
		{Values: map[string]float64{app.MetricBytesIn: 100, app.MetricBytesOut: 40, app.MetricConnections: 3}},
		{Values: map[string]float64{app.MetricConnections: 4}}, // 第一次采样还没有速率
	}
	got := withNetwork(pts)
	if got[0].Values[monitorNet] != 140 {
		t.Fatalf("网络流量应是收到 + 发出 = 140，实际 %v", got[0].Values[monitorNet])
	}
	if _, ok := got[1].Values[monitorNet]; ok {
		t.Fatal("没有速率的点不应带网络流量")
	}
	if _, ok := pts[0].Values[monitorNet]; ok {
		t.Fatal("不应改动监控会话缓冲里的原始点")
	}
}
