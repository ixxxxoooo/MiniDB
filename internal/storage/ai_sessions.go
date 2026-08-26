package storage

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

// AISession 一次 AI 会话的持久化元数据。
type AISession struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ConnectionID string    `json:"connectionId,omitempty"`
	Database     string    `json:"database,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
}

// AISessionMessage 持久化的一条会话消息（含工具轨迹摘要，便于恢复与审计）。
type AISessionMessage struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"` // user | assistant | tool
	Content    string    `json:"content"`
	ToolName   string    `json:"toolName,omitempty"`
	ToolSQL    string    `json:"toolSql,omitempty"`
	ToolResult any       `json:"toolResult,omitempty"` // 结构化工具结果（rows/text），可被前端直接渲染
	CreatedAt  time.Time `json:"createdAt"`
}

// maxAISessionsPerDB 每个数据库单次会话上限（防止无限增长）。
const maxAISessionsPerDB = 200

// SaveAISession 保存会话元数据（upsert）。
func (s *Store) SaveAISession(session AISession) error {
	session.UpdatedAt = time.Now()
	return s.Put("ai_sessions", "meta:"+session.ID, session)
}

// GetAISession 读取会话元数据。
func (s *Store) GetAISession(id string) (AISession, error) {
	var session AISession
	err := s.Get("ai_sessions", "meta:"+id, &session)
	return session, err
}

// DeleteAISession 删除会话及其消息。
func (s *Store) DeleteAISession(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAISessions)
		if err := b.Delete([]byte("meta:" + id)); err != nil {
			return err
		}
		// 删除该会话全部消息（按前缀扫描）
		c := b.Cursor()
		prefix := []byte("msg:" + id + ":")
		for k, _ := c.Seek(prefix); k != nil && hasPrefix(k, prefix); k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
		}
		return nil
	})
}

// AppendAISessionMessage 追加一条会话消息。
func (s *Store) AppendAISessionMessage(sessionID string, msg AISessionMessage) error {
	msg.CreatedAt = time.Now()
	if err := s.Put("ai_sessions", "msg:"+sessionID+":"+msg.ID, msg); err != nil {
		return err
	}
	// 更新消息计数
	var session AISession
	if err := s.Get("ai_sessions", "meta:"+sessionID, &session); err == nil {
		session.MessageCount++
		session.UpdatedAt = time.Now()
		_ = s.Put("ai_sessions", "meta:"+sessionID, session)
	}
	return nil
}

// GetAISessionMessages 读取会话全部消息（按写入顺序）。
func (s *Store) GetAISessionMessages(sessionID string, limit int) ([]AISessionMessage, error) {
	var messages []AISessionMessage
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAISessions)
		c := b.Cursor()
		prefix := []byte("msg:" + sessionID + ":")
		count := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if !hasPrefix(k, prefix) {
				continue
			}
			var msg AISessionMessage
			if err := json.Unmarshal(v, &msg); err != nil {
				continue
			}
			messages = append(messages, msg)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 恢复写入顺序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// ListAISessions 列出最近会话（按更新时间倒序，limit<=0 不限）。
func (s *Store) ListAISessions(limit int) ([]AISession, error) {
	var sessions []AISession
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAISessions)
		c := b.Cursor()
		count := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			if !hasPrefix(k, []byte("meta:")) {
				continue
			}
			var session AISession
			if err := json.Unmarshal(v, &session); err != nil {
				continue
			}
			sessions = append(sessions, session)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// bbolt 键为 meta:<id>，字典序不保证更新时间序，这里按 UpdatedAt 排序
	// 简化：listAISessionsOrdered 处理。
	sessions = orderSessionsByUpdatedAt(sessions)
	// 裁剪超上限（仅保留最近 N 条）
	if len(sessions) > maxAISessionsPerDB {
		sessions = sessions[:maxAISessionsPerDB]
	}
	return sessions, nil
}

func hasPrefix(key, prefix []byte) bool {
	if len(key) < len(prefix) {
		return false
	}
	for i := range prefix {
		if key[i] != prefix[i] {
			return false
		}
	}
	return true
}

func orderSessionsByUpdatedAt(sessions []AISession) []AISession {
	// 简单插入排序（会话数 ≤ 200，性能足够）
	for i := 1; i < len(sessions); i++ {
		for j := i; j > 0 && sessions[j-1].UpdatedAt.Before(sessions[j].UpdatedAt); j-- {
			sessions[j-1], sessions[j] = sessions[j], sessions[j-1]
		}
	}
	return sessions
}
