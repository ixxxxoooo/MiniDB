package services

import (
	"fmt"
	"strings"
	"time"

	"minidb/internal/storage"
)

// AISessionView 对前端暴露的会话视图（与 storage.AISession 一致，字段名稳定）。
type AISessionView struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ConnectionID string    `json:"connectionId,omitempty"`
	Database     string    `json:"database,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
}

// AISessionMessageView 会话消息视图（含结构化工具结果）。
type AISessionMessageView struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolName   string         `json:"toolName,omitempty"`
	ToolSQL    string         `json:"toolSql,omitempty"`
	ToolResult map[string]any `json:"toolResult,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

// ListAISessions 列出最近 AI 会话（limit<=0 返回全部，后端裁剪上限）。
func (s *AIService) ListAISessions(limit int) ([]AISessionView, error) {
	sessions, err := s.store.ListAISessions(limit)
	if err != nil {
		return nil, err
	}
	out := make([]AISessionView, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, toSessionView(session))
	}
	return out, nil
}

// GetAISessionMessages 读取会话消息（limit<=0 全量）。
func (s *AIService) GetAISessionMessages(sessionID string, limit int) ([]AISessionMessageView, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("会话 ID 不能为空")
	}
	messages, err := s.store.GetAISessionMessages(sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AISessionMessageView, 0, len(messages))
	for _, msg := range messages {
		view := AISessionMessageView{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			ToolName:  msg.ToolName,
			ToolSQL:   msg.ToolSQL,
			CreatedAt: msg.CreatedAt,
		}
		if msg.ToolResult != nil {
			view.ToolResult = anyToMap(msg.ToolResult)
		}
		out = append(out, view)
	}
	return out, nil
}

// DeleteAISession 删除会话及消息。
func (s *AIService) DeleteAISession(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("会话 ID 不能为空")
	}
	return s.store.DeleteAISession(sessionID)
}

// persistAISession 持久化会话元数据（内部）。
func (s *AIService) persistAISession(id, title, connID, dbName string) error {
	session := storage.AISession{
		ID:           id,
		Title:        title,
		ConnectionID: connID,
		Database:     dbName,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return s.store.SaveAISession(session)
}

// persistAIMessage 追加消息（内部，供 ChatAIStream 使用）。
func (s *AIService) persistAIMessage(sessionID, role, content, toolName, toolSQL string, toolResult any) {
	if s.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	msg := storage.AISessionMessage{
		ID:        fmt.Sprintf("m%d", time.Now().UnixNano()),
		Role:      role,
		Content:   content,
		ToolName:  toolName,
		ToolSQL:   toolSQL,
		ToolResult: toolResult,
		CreatedAt: time.Now(),
	}
	_ = s.store.AppendAISessionMessage(sessionID, msg)
}

func toSessionView(s storage.AISession) AISessionView {
	return AISessionView{
		ID:           s.ID,
		Title:        s.Title,
		ConnectionID: s.ConnectionID,
		Database:     s.Database,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		MessageCount: s.MessageCount,
	}
}

func anyToMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": v}
}