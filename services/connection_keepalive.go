package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
)

const (
	keepaliveInterval = 30 * time.Second
	keepalivePingTime = 5 * time.Second
	// 事件名：前端据此更新连接状态灯并提示用户
	EventConnectionStatus = "connection:status"
)

// ConnectionStatusEvent 连接健康状态变化（只在状态变化时发送，不会每 30 秒刷屏）。
type ConnectionStatusEvent struct {
	ConnID string `json:"connId"`
	// connected：已恢复 / lost：连接丢失，正在重试
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// keepalive 定期探测所有已连接的数据库，连接被服务端/网络中断（休眠唤醒、VPN 切换、空闲超时）时自动重连。
type keepalive struct {
	svc   *ConnectionService
	emit  func(ConnectionStatusEvent)
	mu    sync.Mutex
	lost  map[string]bool // 当前处于"连接丢失"状态的连接
	stop  context.CancelFunc
	pings map[string]bool // 正在探测中的连接，避免上一轮没结束又起一轮
}

func newKeepalive(svc *ConnectionService, emit func(ConnectionStatusEvent)) *keepalive {
	return &keepalive{svc: svc, emit: emit, lost: map[string]bool{}, pings: map[string]bool{}}
}

func (k *keepalive) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	k.stop = cancel
	go func() {
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				k.checkOnce(ctx)
			}
		}
	}()
}

func (k *keepalive) Stop() {
	if k.stop != nil {
		k.stop()
	}
}

// checkOnce 对每个连接并发探测一次。
func (k *keepalive) checkOnce(ctx context.Context) {
	var wg sync.WaitGroup
	for _, id := range k.svc.manager.ConnectionIDs() {
		k.mu.Lock()
		if k.pings[id] {
			k.mu.Unlock()
			continue
		}
		k.pings[id] = true
		k.mu.Unlock()

		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			defer func() { k.mu.Lock(); delete(k.pings, id); k.mu.Unlock() }()
			k.checkConnection(ctx, id)
		}(id)
	}
	wg.Wait()
}

func (k *keepalive) checkConnection(ctx context.Context, id string) {
	pingCtx, cancel := context.WithTimeout(ctx, keepalivePingTime)
	err := k.svc.manager.Ping(pingCtx, id)
	cancel()
	if err == nil {
		k.markRecovered(id)
		return
	}
	if ctx.Err() != nil {
		return
	}
	logger.Warn("[Keepalive] 连接探测失败: id=%s err=%v，尝试重连", id, err)
	k.markLost(id, err.Error())

	if rerr := k.svc.reconnect(id); rerr != nil {
		if errors.Is(rerr, database.ErrDisconnected) {
			k.forget(id)
			return
		}
		logger.Warn("[Keepalive] 重连失败: id=%s err=%v", id, rerr)
		return
	}
	logger.Info("[Keepalive] 重连成功: id=%s", id)
	k.markRecovered(id)
}

func (k *keepalive) markLost(id, msg string) {
	k.mu.Lock()
	already := k.lost[id]
	k.lost[id] = true
	k.mu.Unlock()
	if !already {
		k.emit(ConnectionStatusEvent{ConnID: id, Status: "lost", Error: msg})
	}
}

func (k *keepalive) markRecovered(id string) {
	k.mu.Lock()
	was := k.lost[id]
	delete(k.lost, id)
	k.mu.Unlock()
	if was {
		k.emit(ConnectionStatusEvent{ConnID: id, Status: "connected"})
	}
}

func (k *keepalive) forget(id string) {
	k.mu.Lock()
	delete(k.lost, id)
	k.mu.Unlock()
}

// reconnect 用已保存的配置重建连接池。
func (s *ConnectionService) reconnect(id string) error {
	var cfg database.ConnectionConfig
	if err := s.store.Get("connections", id, &cfg); err != nil {
		return err
	}
	cfg, err := decryptConnectionConfig(cfg)
	if err != nil {
		return err
	}
	return s.manager.Reconnect(&cfg)
}

// StartKeepalive 启动后台连接保活（应用启动时调用一次）。
//
//wails:ignore
func (s *ConnectionService) StartKeepalive() {
	if s.keepalive != nil {
		return
	}
	s.keepalive = newKeepalive(s, func(ev ConnectionStatusEvent) {
		if s.app != nil {
			s.app.Event.Emit(EventConnectionStatus, ev)
		}
	})
	s.keepalive.Start()
}

// StopKeepalive 停止后台保活（应用退出时调用）。
//
//wails:ignore
func (s *ConnectionService) StopKeepalive() {
	if s.keepalive != nil {
		s.keepalive.Stop()
	}
}
