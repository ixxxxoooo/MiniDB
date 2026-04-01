package services

import (
	"context"
	"strings"
	"sync"
	"tableplus-ai/internal/ai"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AIService AI 功能服务
type AIService struct {
	ctx     context.Context
	client  *ai.Client
	manager *database.Manager
	store   *storage.Store
	// customSystemPrompt 用于持久化用户在设置中配置的会话提示词
	customSystemPrompt string
	// schemaCache 用于缓存 schema，降低高频 Chat 场景的元数据读取开销
	schemaCache   map[string]schemaCacheEntry
	schemaCacheMu sync.RWMutex
}

type schemaCacheEntry struct {
	schema    *ai.SchemaContext
	expiresAt time.Time
}

type ChatStreamEvent struct {
	RequestID string `json:"requestId"`
	Type      string `json:"type"`
	Delta     string `json:"delta,omitempty"`
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

const (
	schemaCacheTTL    = 5 * time.Minute
	maxChatContextMsg = 12
)

// NewAIService 创建 AI 服务
func NewAIService(manager *database.Manager, store *storage.Store) *AIService {
	// 从存储中加载 AI 配置
	var cfg ai.Config
	err := store.Get("settings", "ai_config", &cfg)
	if err != nil {
		cfg = ai.Config{
			BaseURL:     "https://api.openai.com/v1",
			Model:       "gpt-4o",
			MaxTokens:   4096,
			Temperature: 0.3,
		}
	}

	return &AIService{
		client:             ai.NewClient(&cfg),
		manager:            manager,
		store:              store,
		customSystemPrompt: strings.TrimSpace(cfg.SystemPrompt),
		schemaCache:        make(map[string]schemaCacheEntry),
	}
}

// SetContext 注入 Wails 上下文，用于推送流式事件
func (s *AIService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// ReloadConfig 重新加载 AI 配置
func (s *AIService) ReloadConfig() {
	var cfg ai.Config
	err := s.store.Get("settings", "ai_config", &cfg)
	if err == nil {
		s.client.UpdateConfig(&cfg)
		s.customSystemPrompt = strings.TrimSpace(cfg.SystemPrompt)
		logger.Debug("[AIService] 已加载会话提示词: len=%d", len(s.customSystemPrompt))
	}
}

// NaturalLanguageToSQL 自然语言转 SQL
func (s *AIService) NaturalLanguageToSQL(connID, dbName, prompt string) (map[string]interface{}, error) {
	s.ReloadConfig()
	schema, err := s.getSchemaWithCache(connID, dbName)
	if err != nil {
		return nil, err
	}

	result, err := s.client.NaturalLanguageToSQL(context.Background(), schema, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"sql":         result.SQL,
		"explanation": result.Explanation,
		"confidence":  result.Confidence,
	}, nil
}

// ExplainSQL SQL 解释
func (s *AIService) ExplainSQL(sqlStr string) (string, error) {
	s.ReloadConfig()
	return s.client.ExplainSQL(context.Background(), sqlStr)
}

// AnalyzeData 数据洞察分析
func (s *AIService) AnalyzeData(columns []string, rows []map[string]interface{}, question string) (string, error) {
	s.ReloadConfig()
	return s.client.AnalyzeData(context.Background(), columns, rows, question)
}

// GenerateTableDoc 生成表文档
func (s *AIService) GenerateTableDoc(connID, dbName, tableName string) (string, error) {
	s.ReloadConfig()
	schema, err := s.getSchemaWithCache(connID, dbName)
	if err != nil {
		return "", err
	}
	return s.client.GenerateTableDoc(context.Background(), schema, tableName)
}

// DiagnoseError 错误诊断
func (s *AIService) DiagnoseError(sqlStr, errorMsg string) (string, error) {
	s.ReloadConfig()
	return s.client.DiagnoseError(context.Background(), sqlStr, errorMsg)
}

// ChatAI 会话式 AI 助手，支持多轮对话
func (s *AIService) ChatAI(connID, dbName string, messages []ai.ChatMessage) (map[string]interface{}, error) {
	s.ReloadConfig()
	logger.Info("[AIService] ChatAI 开始: connID=%s dbName=%s messages_count=%d", connID, dbName, len(messages))

	// 记录每条消息内容
	for i, m := range messages {
		contentPreview := m.Content
		if len(contentPreview) > 150 {
			contentPreview = contentPreview[:150] + "..."
		}
		logger.Debug("[AIService] 消息[%d]: role=%s content=%s", i, m.Role, contentPreview)
	}

	// 构建表结构上下文
	schemaStr := ""
	if connID != "" && dbName != "" {
		schema, err := s.getSchemaWithCache(connID, dbName)
		if err == nil {
			schemaStr = buildSchemaStr(schema)
			logger.Debug("[AIService] 数据库 schema 已加载: tables_count=%d schema_len=%d", len(schema.Tables), len(schemaStr))
		} else {
			logger.Warn("[AIService] 加载数据库 schema 失败: %v", err)
		}
	}

	systemPrompt := s.buildChatSystemPrompt(schemaStr)

	contextMessages := trimContextMessages(messages)
	logger.Info("[AIService] ChatAI 上下文裁剪: 原始=%d 裁剪后=%d", len(messages), len(contextMessages))

	resp, err := s.client.ChatWithMessages(context.Background(), systemPrompt, contextMessages)
	if err != nil {
		logger.Error("[AIService] ChatAI 失败: %v", err)
		return nil, err
	}

	logger.Info("[AIService] ChatAI 成功: response_len=%d", len(resp))
	return map[string]interface{}{
		"content": resp,
	}, nil
}

// ChatAIStream 会话式 AI 助手（流式输出）
func (s *AIService) ChatAIStream(connID, dbName string, messages []ai.ChatMessage, requestID string) (map[string]interface{}, error) {
	s.ReloadConfig()
	logger.Info("[AIService] ChatAIStream 开始: connID=%s dbName=%s requestID=%s messages_count=%d", connID, dbName, requestID, len(messages))

	schemaStr := ""
	if connID != "" && dbName != "" {
		schema, err := s.getSchemaWithCache(connID, dbName)
		if err == nil {
			schemaStr = buildSchemaStr(schema)
			logger.Debug("[AIService] ChatAIStream schema 已加载: tables_count=%d schema_len=%d", len(schema.Tables), len(schemaStr))
		} else {
			logger.Warn("[AIService] ChatAIStream 加载 schema 失败: %v", err)
		}
	}

	systemPrompt := s.buildChatSystemPrompt(schemaStr)

	contextMessages := trimContextMessages(messages)
	logger.Info("[AIService] ChatAIStream 上下文裁剪: 原始=%d 裁剪后=%d", len(messages), len(contextMessages))

	resp, err := s.client.ChatWithMessagesStream(
		context.Background(),
		systemPrompt,
		contextMessages,
		func(delta string) {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "delta",
				Delta:     delta,
			})
		},
	)
	if err != nil {
		s.emitStreamEvent(ChatStreamEvent{
			RequestID: requestID,
			Type:      "error",
			Error:     err.Error(),
		})
		logger.Error("[AIService] ChatAIStream 失败: %v", err)
		return nil, err
	}

	s.emitStreamEvent(ChatStreamEvent{
		RequestID: requestID,
		Type:      "done",
		Content:   resp,
	})
	logger.Info("[AIService] ChatAIStream 成功: response_len=%d", len(resp))
	return map[string]interface{}{
		"content": resp,
	}, nil
}

func (s *AIService) emitStreamEvent(event ChatStreamEvent) {
	if s.ctx == nil {
		logger.Warn("[AIService] emitStreamEvent 时上下文为空: requestID=%s type=%s", event.RequestID, event.Type)
		return
	}
	runtime.EventsEmit(s.ctx, "ai:chat_stream", event)
}

func buildSchemaStr(schema *ai.SchemaContext) string {
	result := ""
	for _, t := range schema.Tables {
		result += "表 " + t.Name + ": "
		cols := []string{}
		for _, c := range t.Columns {
			cols = append(cols, c.Name+"("+c.Type+")")
		}
		if len(cols) > 10 {
			cols = cols[:10]
			cols = append(cols, "...")
		}
		result += strings.Join(cols, ", ") + "\n"
	}
	return result
}

func trimContextMessages(messages []ai.ChatMessage) []ai.ChatMessage {
	if len(messages) <= maxChatContextMsg {
		return messages
	}
	return messages[len(messages)-maxChatContextMsg:]
}

func (s *AIService) buildChatSystemPrompt(schemaStr string) string {
	// 基础系统提示词：约束数据库助手职责与输出格式
	basePrompt := `你是一个智能数据库助手。你可以帮助用户查询数据、生成 SQL、解释 SQL、分析数据等。

规则：
1. 如果用户想查询数据，生成 SQL 并用 ` + "```sql" + ` 代码块包裹
2. 如果用户想要数据结果，生成 SQL 后在末尾加上标记 ` + "`[AUTO_EXECUTE]`" + `，系统会自动执行并返回数据
3. 使用 Markdown 格式输出，支持表格、列表、代码块等
4. 回答要简洁专业
5. 根据提供的表结构信息生成准确的 SQL`

	if s.customSystemPrompt != "" {
		basePrompt += "\n\n用户自定义会话提示词:\n" + s.customSystemPrompt
	}
	if schemaStr != "" {
		basePrompt += "\n\n当前数据库表结构:\n" + schemaStr
	}
	return basePrompt
}

func (s *AIService) getSchemaWithCache(connID, dbName string) (*ai.SchemaContext, error) {
	cacheKey := connID + "::" + dbName
	now := time.Now()

	s.schemaCacheMu.RLock()
	entry, ok := s.schemaCache[cacheKey]
	s.schemaCacheMu.RUnlock()
	if ok && entry.expiresAt.After(now) && entry.schema != nil {
		logger.Debug("[AIService] 命中 schema 缓存: connID=%s db=%s", connID, dbName)
		return entry.schema, nil
	}

	logger.Debug("[AIService] schema 缓存未命中，开始构建: connID=%s db=%s", connID, dbName)
	schema, err := s.buildSchema(connID, dbName)
	if err != nil {
		return nil, err
	}

	s.schemaCacheMu.Lock()
	s.schemaCache[cacheKey] = schemaCacheEntry{
		schema:    schema,
		expiresAt: now.Add(schemaCacheTTL),
	}
	s.schemaCacheMu.Unlock()
	logger.Debug("[AIService] schema 缓存写入成功: connID=%s db=%s ttl=%s", connID, dbName, schemaCacheTTL.String())
	return schema, nil
}

// buildSchema 构建表结构上下文
func (s *AIService) buildSchema(connID, dbName string) (*ai.SchemaContext, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, err
	}

	tables, err := database.GetTables(db, cfg.Type, dbName)
	if err != nil {
		return nil, err
	}

	schema := &ai.SchemaContext{
		DatabaseType: cfg.Type,
		DatabaseName: dbName,
	}

	for _, t := range tables {
		cols, err := database.GetColumns(db, cfg.Type, dbName, t.Name)
		if err != nil {
			continue
		}

		tableSchema := ai.TableSchema{Name: t.Name}
		for _, c := range cols {
			tableSchema.Columns = append(tableSchema.Columns, ai.ColumnSchema{
				Name:     c.Name,
				Type:     c.Type,
				Nullable: c.Nullable,
				Comment:  c.Comment,
			})
		}
		schema.Tables = append(schema.Tables, tableSchema)
	}

	return schema, nil
}
