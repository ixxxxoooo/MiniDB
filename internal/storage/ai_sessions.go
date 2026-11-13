package storage

import (
	"encoding/json"
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
		_ = s.Put("ai_sessions", "meta:"+sessionID, session)
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

// ListAISessions 列出最近会话（按更新时间倒序，limit<=0 不限）。
func (s *Store) ListAISessions(limit int) ([]AISession, error) {
	var sessions []AISession
	err := s.scanPrefix("ai_sessions", "meta:", false, func(_ string, v []byte) bool {
		var session AISession
		if json.Unmarshal(v, &session) == nil {
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
