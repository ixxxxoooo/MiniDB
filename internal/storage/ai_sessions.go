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
	if err := s.Delete("ai_sessions", "meta:"+id); err != nil {
		return err
	}
	return s.deletePrefix("ai_sessions", "msg:"+id+":")
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
		// 消息已写入，只是计数没更新成功；把错误交给调用方，而不是悄悄吞掉
		if err := s.Put("ai_sessions", "meta:"+sessionID, session); err != nil {
			return fmt.Errorf("更新会话消息数失败: %w", err)
		}
	}
	return nil
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
