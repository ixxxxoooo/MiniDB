package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/logger"
	"minidb/internal/storage"
)

// AISessionView 对界面暴露的会话摘要。
type AISessionView struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ConnectionID string    `json:"connectionId,omitempty"`
	Database     string    `json:"database,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MessageCount int       `json:"messageCount"`
}

// AISessionMessageView 一条会话消息。助手消息带完整的 Transcript，界面据此原样还原当时的回答过程。
type AISessionMessageView struct {
	ID         string            `json:"id"`
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	Transcript *agent.Transcript `json:"transcript,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
}

// ListAISessions 列出某个连接下最近的会话（connID 为空则不按连接过滤）。
func (s *AIService) ListAISessions(connID string, limit int) ([]AISessionView, error) {
	defer logger.Perf("app.AI.ListAISessions", "conn", connID)()
	sessions, err := s.store.ListAISessions(strings.TrimSpace(connID), limit)
	if err != nil {
		return nil, err
	}
	out := make([]AISessionView, 0, len(sessions))
	for _, x := range sessions {
		out = append(out, AISessionView{ID: x.ID, Title: x.Title, ConnectionID: x.ConnectionID, Database: x.Database,
			CreatedAt: x.CreatedAt, UpdatedAt: x.UpdatedAt, MessageCount: x.MessageCount})
	}
	return out, nil
}

// GetAISessionMessages 读取会话消息（limit<=0 为全部），按时间顺序返回。
func (s *AIService) GetAISessionMessages(sessionID string, limit int) ([]AISessionMessageView, error) {
	defer logger.Perf("app.AI.GetAISessionMessages")()
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("会话 ID 不能为空")
	}
	messages, err := s.store.GetAISessionMessages(sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AISessionMessageView, 0, len(messages))
	for _, m := range messages {
		view := AISessionMessageView{ID: m.ID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt}
		if len(m.Transcript) > 0 {
			var tr agent.Transcript
			if json.Unmarshal(m.Transcript, &tr) == nil {
				view.Transcript = &tr
			}
		}
		out = append(out, view)
	}
	return out, nil
}

// DeleteAISession 删除会话及其全部消息。
func (s *AIService) DeleteAISession(sessionID string) error {
	defer logger.Perf("app.AI.DeleteAISession")()
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("会话 ID 不能为空")
	}
	return s.store.DeleteAISession(sessionID)
}

// DropAISessionTail 删除会话末尾的 count 条消息。"重新生成"前调用，
// 避免同一个问题和它的旧回答在保存的会话里重复出现。
func (s *AIService) DropAISessionTail(sessionID string, count int) error {
	defer logger.Perf("app.AI.DropAISessionTail")()
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || count <= 0 {
		return nil
	}
	return s.store.DeleteLastAISessionMessages(sessionID, count)
}

// saveUserMessage 保存本轮用户消息；会话第一次出现时先建会话。没有 sessionID 时不保存。
func (r *chatRun) saveUserMessage(connID, dbName string, msg ai.ChatMessage) {
	if r.sessionID == "" || msg.Role != "user" {
		return
	}
	if _, err := r.s.store.GetAISession(r.sessionID); err != nil {
		now := time.Now()
		session := storage.AISession{ID: r.sessionID, Title: generateSessionTitle(msg.Content), ConnectionID: connID, Database: dbName, CreatedAt: now, UpdatedAt: now}
		if err := r.s.store.SaveAISession(session); err != nil {
			logger.Warn("[AIService] 创建会话失败: %v", err)
			return
		}
	}
	r.appendMessage(storage.AISessionMessage{Role: "user", Content: msg.Content})
}

// generateSessionTitle 由首条用户消息生成会话标题：折叠空白并按字符（而非字节）截断，中文不会被截成乱码。
func generateSessionTitle(msg string) string {
	title := strings.Join(strings.Fields(msg), " ")
	if r := []rune(title); len(r) > 30 {
		title = string(r[:30]) + "…"
	}
	if title == "" {
		title = "AI 对话"
	}
	return title
}

// saveAssistantMessage 保存助手回复：正文用于后续轮次的上下文，Transcript 用于还原回答过程。
// 用户中途停止或出错时保存已有的部分，恢复会话时能看到停在了哪里。
func (r *chatRun) saveAssistantMessage() {
	if r.sessionID == "" {
		return
	}
	tr := r.snapshot()
	if len(tr.Parts) == 0 && tr.Error == "" {
		return
	}
	data, err := json.Marshal(tr)
	if err != nil {
		logger.Warn("[AIService] 序列化回答过程失败: %v", err)
		return
	}
	r.appendMessage(storage.AISessionMessage{Role: "assistant", Content: tr.Answer(), Transcript: data})
}

// appendMessage 追加一条消息；存储失败只记日志，不影响对话本身。
func (r *chatRun) appendMessage(msg storage.AISessionMessage) {
	msg.ID = fmt.Sprintf("m%d", time.Now().UnixNano())
	if err := r.s.store.AppendAISessionMessage(r.sessionID, msg); err != nil {
		logger.Warn("[AIService] 保存会话消息失败: session=%s err=%v", r.sessionID, err)
	}
}
