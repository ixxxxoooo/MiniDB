package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
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

// AISessionMessage 持久化的一条会话消息。助手消息的 Transcript 是这次回答的完整过程
// （推理、工具调用与结果、文本片段），由 app 层序列化，存储层不关心它的结构。
type AISessionMessage struct {
	ID         string          `json:"id"`
	Role       string          `json:"role"` // user | assistant
	Content    string          `json:"content"`
	Transcript json.RawMessage `json:"transcript,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}

const (
	// maxAISessionsPerDB 每个连接最多保留的会话数：超出时淘汰最久没更新的会话及其消息。
	maxAISessionsPerDB = 200
	// maxAIMessagesPerSession 单个会话最多保留的消息数：超出时淘汰最早的消息。
	maxAIMessagesPerSession = 400
)

// SaveAISession 保存会话元数据（upsert），并淘汰同一连接下超出上限的旧会话。
func (s *Store) SaveAISession(session AISession) error {
	session.UpdatedAt = time.Now()
	if err := s.Put("ai_sessions", "meta:"+session.ID, session); err != nil {
		return err
	}
	return s.pruneAISessions(session.ConnectionID, maxAISessionsPerDB)
}

// pruneAISessions 把连接 connectionID 下的会话裁到 keep 个，留下最近更新的。
func (s *Store) pruneAISessions(connectionID string, keep int) error {
	var sessions []AISession
	err := s.scanPrefix("ai_sessions", "meta:", false, func(_ string, v []byte) bool {
		var session AISession
		if json.Unmarshal(v, &session) == nil && session.ConnectionID == connectionID {
			sessions = append(sessions, session)
		}
		return true
	})
	if err != nil || len(sessions) <= keep {
		return err
	}
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	for _, old := range sessions[keep:] {
		if err := s.DeleteAISession(old.ID); err != nil {
			return err
		}
	}
	return nil
}

// GetAISession 读取会话元数据。
func (s *Store) GetAISession(id string) (AISession, error) {
	var session AISession
	err := s.Get("ai_sessions", "meta:"+id, &session)
	return session, err
}

// DeleteAISession 删除会话及其消息。
func (s *Store) DeleteAISession(id string) error {
	if err := s.Delete("ai_sessions", "meta:"+id); err != nil {
		return err
	}
	return s.deletePrefix("ai_sessions", "msg:"+id+":")
}

// AppendAISessionMessage 追加一条会话消息。消息、计数更新与超限淘汰在同一个事务里完成，
// 中途失败不会留下"消息写了但计数没跟上"的状态。
func (s *Store) AppendAISessionMessage(sessionID string, msg AISessionMessage) error {
	msg.CreatedAt = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const upsert = `INSERT INTO kv(bucket,k,v) VALUES(?,?,?) ON CONFLICT(bucket,k) DO UPDATE SET v=excluded.v`
	if _, err := tx.Exec(upsert, "ai_sessions", "msg:"+sessionID+":"+msg.ID, data); err != nil {
		return err
	}

	var raw []byte
	err = tx.QueryRow(`SELECT v FROM kv WHERE bucket=? AND k=?`, "ai_sessions", "meta:"+sessionID).Scan(&raw)
	if err == nil {
		var session AISession
		if json.Unmarshal(raw, &session) == nil {
			session.MessageCount++
			session.UpdatedAt = time.Now()
			if over := session.MessageCount - maxAIMessagesPerSession; over > 0 {
				// 键按消息 ID（时间序）排列，最早的消息排在最前
				res, err := tx.Exec(`DELETE FROM kv WHERE bucket=? AND k IN (
					SELECT k FROM kv WHERE bucket=? AND k>=? AND k<? ORDER BY k LIMIT ?)`,
					"ai_sessions", "ai_sessions", "msg:"+sessionID+":", prefixEnd("msg:"+sessionID+":"), over)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					session.MessageCount -= int(n)
				}
			}
			meta, err := json.Marshal(session)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(upsert, "ai_sessions", "meta:"+sessionID, meta); err != nil {
				return fmt.Errorf("更新会话消息数失败: %w", err)
			}
		}
	}
	return tx.Commit()
}

// GetAISessionMessages 读取会话最近 limit 条消息（limit<=0 为全部），按写入顺序返回。
func (s *Store) GetAISessionMessages(sessionID string, limit int) ([]AISessionMessage, error) {
	var messages []AISessionMessage
	err := s.scanPrefix("ai_sessions", "msg:"+sessionID+":", true, func(_ string, v []byte) bool {
		var msg AISessionMessage
		if json.Unmarshal(v, &msg) == nil {
			messages = append(messages, msg)
		}
		return limit <= 0 || len(messages) < limit
	})
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// DeleteLastAISessionMessages 删除会话末尾的 n 条消息（用于"重新生成"：丢掉上一轮问答再重新提问）。
func (s *Store) DeleteLastAISessionMessages(sessionID string, n int) error {
	var keys []string
	err := s.scanPrefix("ai_sessions", "msg:"+sessionID+":", true, func(key string, _ []byte) bool {
		keys = append(keys, key)
		return len(keys) < n
	})
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := s.Delete("ai_sessions", key); err != nil {
			return err
		}
	}
	var session AISession
	if err := s.Get("ai_sessions", "meta:"+sessionID, &session); err == nil {
		session.MessageCount = max(0, session.MessageCount-len(keys))
		return s.Put("ai_sessions", "meta:"+sessionID, session)
	}
	return nil
}

// ListAISessions 列出最近会话（按更新时间倒序，limit<=0 不限）。
// connectionID 非空时只列属于该连接的会话：不同数据库的对话互不混在一起。
func (s *Store) ListAISessions(connectionID string, limit int) ([]AISession, error) {
	var sessions []AISession
	err := s.scanPrefix("ai_sessions", "meta:", false, func(_ string, v []byte) bool {
		var session AISession
		if json.Unmarshal(v, &session) == nil && (connectionID == "" || session.ConnectionID == connectionID) {
			sessions = append(sessions, session)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	if len(sessions) > maxAISessionsPerDB {
		sessions = sessions[:maxAISessionsPerDB]
	}
	return sessions, nil
}
