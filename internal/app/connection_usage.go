package app

import (
	"time"

	"minidb/internal/logger"
	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 连接的使用记录：最近一次连接成功的时间（欢迎页的「最近使用」）和是否固定（「已固定」）。
// 都直接改存储里的原始记录（密码、密钥仍是密文），不经过 SaveConnection 的加解密。

// markUsed 把连接 id 的最近使用时间记成现在。写失败只记日志：这是展示用的辅助信息，不该让连接失败。
func (s *ConnectionService) markUsed(id string) {
	if err := s.updateRaw(id, func(c *model.ConnectionConfig) { c.LastUsed = time.Now().Unix() }); err != nil {
		logger.Warn("[ConnectionService] 记录最近使用失败: id=%s err=%v", id, err)
	}
}

// SetPinned 固定或取消固定连接 id。
func (s *ConnectionService) SetPinned(id string, pinned bool) error {
	defer logger.Perf("app.Connections.SetPinned", "conn", id)()
	return s.updateRaw(id, func(c *model.ConnectionConfig) { c.Pinned = pinned })
}

// updateRaw 读出连接 id 的原始记录，交给 change 改完再写回。
func (s *ConnectionService) updateRaw(id string, change func(*model.ConnectionConfig)) error {
	var cfg model.ConnectionConfig
	if err := s.store.Get("connections", id, &cfg); err != nil {
		return model.Errf(model.CodeConnNotFound, "连接不存在: %s", id)
	}
	change(&cfg)
	return s.store.Put("connections", id, cfg)
}
