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
	query   *QueryService // 自动执行 SQL、查询结果格式化等由后端编排时使用
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
	RequestID  string `json:"requestId"`
	Type       string `json:"type"`
	Delta      string `json:"delta,omitempty"`
	Content    string `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	ToolInput  string `json:"toolInput,omitempty"`
	ToolSQL    string `json:"toolSql,omitempty"`
	ToolOutput string `json:"toolOutput,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// ChatAutoExecuteDirective 描述 AI 回复是否建议系统自动执行首个 SQL 代码块。
type ChatAutoExecuteDirective struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

const (
	schemaCacheTTL    = 5 * time.Minute
	maxChatContextMsg = 12
)

// NewAIService 创建 AI 服务（query 用于会话内自动执行与结果合并，可为 nil 则相关能力不可用）
func NewAIService(manager *database.Manager, store *storage.Store, query *QueryService) *AIService {
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
		query:              query,
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

	for i, m := range messages {
		contentPreview := m.Content
		if len(contentPreview) > 150 {
			contentPreview = contentPreview[:150] + "..."
		}
		logger.Debug("[AIService] 消息[%d]: role=%s content=%s", i, m.Role, contentPreview)
	}

	// 构建分层 Schema 上下文（根据表数量 + 用户问题自动选择策略）
	userQuestion := extractUserQuestion(messages)
	schemaStr := ""
	dbType, dbVersion := "", ""
	if connID != "" && dbName != "" {
		schema, err := s.getSchemaWithCache(connID, dbName)
		if err == nil {
			schemaStr = buildSchemaForChat(schema, userQuestion)
			dbType = schema.DatabaseType
			dbVersion = schema.DatabaseVersion
			// 规则路由工具执行：将可审计工具输出拼接到上下文，减少“黑盒”感
			toolContext := s.runPlannedTools(connID, dbName, userQuestion, schema, "", false)
			if toolContext != "" {
				schemaStr += toolContext
			}
			logger.Debug("[AIService] 数据库 schema 已加载: tables_count=%d schema_len=%d dbType=%s", len(schema.Tables), len(schemaStr), dbType)
		} else {
			logger.Warn("[AIService] 加载数据库 schema 失败: %v", err)
		}
	}

	systemPrompt := s.buildChatSystemPrompt(schemaStr, dbType, dbVersion)

	contextMessages := trimContextMessages(messages)
	logger.Info("[AIService] ChatAI 上下文裁剪: 原始=%d 裁剪后=%d", len(messages), len(contextMessages))

	resp, err := s.client.ChatWithMessages(context.Background(), systemPrompt, contextMessages)
	if err != nil {
		logger.Error("[AIService] ChatAI 失败: %v", err)
		return nil, err
	}

	meta, cleanResp, metaOK := database.ExtractAutoExecuteMetaBlock(resp)
	directive := buildChatAutoExecuteDirective(extractUserQuestion(contextMessages), cleanResp, meta, metaOK)
	logger.Info("[AIService] ChatAI 成功: response_len=%d auto_execute=%v meta=%v", len(cleanResp), directive.Enabled, metaOK)
	return map[string]interface{}{
		"content":     cleanResp,
		"autoExecute": directive,
	}, nil
}

// ChatAIStream 会话式 AI 助手（流式输出）
func (s *AIService) ChatAIStream(connID, dbName string, messages []ai.ChatMessage, requestID string) (map[string]interface{}, error) {
	s.ReloadConfig()
	logger.Info("[AIService] ChatAIStream 开始: connID=%s dbName=%s requestID=%s messages_count=%d", connID, dbName, requestID, len(messages))

	// 构建分层 Schema 上下文
	userQuestion := extractUserQuestion(messages)
	schemaStr := ""
	dbType, dbVersion := "", ""
	if connID != "" && dbName != "" {
		// 推送进度：正在加载表结构
		s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "loading_schema"})

		schema, err := s.getSchemaWithCache(connID, dbName)
		if err == nil {
			schemaStr = buildSchemaForChat(schema, userQuestion)
			dbType = schema.DatabaseType
			dbVersion = schema.DatabaseVersion
			// 推送进度：正在规划与执行工具
			s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "planning_tools"})
			toolContext := s.runPlannedTools(connID, dbName, userQuestion, schema, requestID, true)
			if toolContext != "" {
				schemaStr += toolContext
			}
			logger.Debug("[AIService] ChatAIStream schema 已加载: tables_count=%d schema_len=%d dbType=%s", len(schema.Tables), len(schemaStr), dbType)
		} else {
			logger.Warn("[AIService] ChatAIStream 加载 schema 失败: %v", err)
		}

		// 推送进度：正在调用 AI
		s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "calling_ai"})
	}

	systemPrompt := s.buildChatSystemPrompt(schemaStr, dbType, dbVersion)

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

	meta, cleanResp, metaOK := database.ExtractAutoExecuteMetaBlock(resp)
	s.emitStreamEvent(ChatStreamEvent{
		RequestID: requestID,
		Type:      "done",
		Content:   cleanResp,
	})
	directive := buildChatAutoExecuteDirective(extractUserQuestion(contextMessages), cleanResp, meta, metaOK)
	logger.Info("[AIService] ChatAIStream 成功: response_len=%d auto_execute=%v meta=%v", len(cleanResp), directive.Enabled, metaOK)
	return map[string]interface{}{
		"content":     cleanResp,
		"autoExecute": directive,
	}, nil
}

func buildChatAutoExecuteDirective(lastUserMessage, assistantContent string, meta database.AutoExecuteIntentMetaBlock, metaOK bool) ChatAutoExecuteDirective {
	if metaOK {
		return ChatAutoExecuteDirective{
			Enabled: meta.AutoExecute.Enabled,
			Mode:    strings.TrimSpace(meta.AutoExecute.Mode),
			Reason:  strings.TrimSpace(meta.AutoExecute.Reason),
		}
	}
	if !database.WantsAutoExecuteFromConversation(lastUserMessage, assistantContent) {
		return ChatAutoExecuteDirective{Enabled: false}
	}
	return ChatAutoExecuteDirective{
		Enabled: true,
		Mode:    "first_sql_readonly",
		Reason:  "user_requested_result",
	}
}

func (s *AIService) emitStreamEvent(event ChatStreamEvent) {
	if s.ctx == nil {
		logger.Warn("[AIService] emitStreamEvent 时上下文为空: requestID=%s type=%s", event.RequestID, event.Type)
		return
	}
	runtime.EventsEmit(s.ctx, "ai:chat_stream", event)
}

// 分层 Schema 策略阈值
const (
	schemaSmallThreshold  = 30  // 小库：<= 30 张表，全量 DDL
	schemaMediumThreshold = 200 // 中库：31~200 张表，摘要 + 关键词过滤
	maxRelevantTables     = 25  // 中库/大库中，最多传多少张表的完整 DDL
	minRelevantTables     = 8   // 中库匹配表数不足此值时退回全量 DDL，避免遗漏
)

// buildSchemaForChat 根据表数量分层构建 Schema 上下文
// userQuestion 用于中库场景下的关键词过滤
func buildSchemaForChat(schema *ai.SchemaContext, userQuestion string) string {
	tableCount := len(schema.Tables)
	logger.Info("[AIService] 分层 Schema 策略: tables=%d", tableCount)

	// 小库：全量 DDL
	if tableCount <= schemaSmallThreshold {
		logger.Info("[AIService] 小库策略: 全量 DDL (%d 张表)", tableCount)
		return ai.BuildSchemaDDL(schema)
	}

	// 中库：表名摘要 + 关键词匹配相关表的完整 DDL
	if tableCount <= schemaMediumThreshold {
		relevant := filterRelevantTables(schema.Tables, userQuestion)
		logger.Info("[AIService] 中库策略: 关键词匹配 %d/%d 张相关表", len(relevant), tableCount)

		// 匹配表数不足最小保底值时退回全量 DDL，防止因关键词不准导致遗漏关键表
		if len(relevant) < minRelevantTables {
			logger.Info("[AIService] 中库策略: 匹配表数(%d)不足最小保底(%d)，退回全量 DDL", len(relevant), minRelevantTables)
			return ai.BuildSchemaDDL(schema)
		}

		var sb strings.Builder
		sb.WriteString(ai.BuildTableSummary(schema.Tables))
		sb.WriteString("\n")
		sb.WriteString("-- 以下是与本次查询相关的表结构详情：\n\n")
		sb.WriteString(ai.BuildTablesDDL(relevant))
		return sb.String()
	}

	// 大库（> 200 张表）：只传表名摘要 + 关键词匹配的相关表 DDL
	relevant := filterRelevantTables(schema.Tables, userQuestion)
	logger.Info("[AIService] 大库策略: 关键词匹配 %d/%d 张相关表", len(relevant), tableCount)

	var sb strings.Builder
	sb.WriteString(ai.BuildTableSummary(schema.Tables))
	sb.WriteString("\n")
	if len(relevant) > 0 {
		sb.WriteString("-- 以下是与本次查询可能相关的表结构详情：\n\n")
		sb.WriteString(ai.BuildTablesDDL(relevant))
	}
	return sb.String()
}

// filterRelevantTables 从用户问题中提取关键词，匹配相关表
// 匹配策略：表名精确/模糊匹配 + 表注释关键词匹配
func filterRelevantTables(tables []ai.TableSchema, userQuestion string) []ai.TableSchema {
	if userQuestion == "" {
		return nil
	}

	questionLower := strings.ToLower(userQuestion)
	matched := make(map[string]bool)
	var result []ai.TableSchema

	// 第一轮：表名匹配（精确或包含）
	for _, t := range tables {
		nameLower := strings.ToLower(t.Name)
		// 用户问题中直接提到了表名
		if strings.Contains(questionLower, nameLower) {
			matched[t.Name] = true
			result = append(result, t)
			continue
		}
		// 去掉常见前缀后匹配（如 tbl/t_/tb_ 等）
		shortName := stripTablePrefix(nameLower)
		if shortName != nameLower && len(shortName) >= 3 && strings.Contains(questionLower, shortName) {
			matched[t.Name] = true
			result = append(result, t)
		}
	}

	// 第二轮：表注释关键词匹配 + 表名子串匹配
	keywords := extractKeywords(userQuestion)
	for _, t := range tables {
		if matched[t.Name] {
			continue
		}
		commentLower := strings.ToLower(t.Comment)
		shortName := stripTablePrefix(strings.ToLower(t.Name))
		for _, kw := range keywords {
			if len(kw) < 2 {
				continue
			}
			// 匹配表注释
			if commentLower != "" && strings.Contains(commentLower, kw) {
				matched[t.Name] = true
				result = append(result, t)
				break
			}
			// 匹配去前缀后的表名子串（如 kw="report" 匹配 shortName="bwreport"）
			if len(kw) >= 3 && strings.Contains(shortName, kw) {
				matched[t.Name] = true
				result = append(result, t)
				break
			}
		}
	}

	// 第三轮：外键关联表补充（如果已匹配的表有外键引用其他表，也加入）
	fkTables := make(map[string]bool)
	for _, t := range result {
		for _, c := range t.Columns {
			if c.ForeignKey != "" {
				// 外键格式："other_table.column"
				parts := strings.SplitN(c.ForeignKey, ".", 2)
				if len(parts) > 0 {
					fkTables[parts[0]] = true
				}
			}
		}
	}
	for _, t := range tables {
		if !matched[t.Name] && fkTables[t.Name] {
			matched[t.Name] = true
			result = append(result, t)
		}
	}

	// 限制最大数量
	if len(result) > maxRelevantTables {
		result = result[:maxRelevantTables]
	}

	return result
}

// stripTablePrefix 去除常见的表名前缀
func stripTablePrefix(name string) string {
	prefixes := []string{"tbl_", "tbl", "tb_", "t_", "sys_"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}

// extractKeywords 从用户问题中提取关键词
func extractKeywords(question string) []string {
	// 按空格、标点、常见连接词分割
	replacer := strings.NewReplacer(
		"，", " ", "。", " ", "、", " ", "？", " ", "！", " ",
		"的", " ", "了", " ", "吗", " ", "呢", " ", "和", " ",
		",", " ", ".", " ", "?", " ", "!", " ",
		"(", " ", ")", " ", "（", " ", "）", " ",
	)
	cleaned := replacer.Replace(question)
	parts := strings.Fields(cleaned)

	var keywords []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 {
			keywords = append(keywords, strings.ToLower(p))
		}
	}
	return keywords
}

// extractUserQuestion 从聊天消息中提取最新的用户问题
func extractUserQuestion(messages []ai.ChatMessage) string {
	// 从后往前找最后一条 user 消息
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func trimContextMessages(messages []ai.ChatMessage) []ai.ChatMessage {
	if len(messages) <= maxChatContextMsg {
		return messages
	}
	return messages[len(messages)-maxChatContextMsg:]
}

func (s *AIService) buildChatSystemPrompt(schemaStr, dbType, dbVersion string) string {
	// 基础系统提示词：约束数据库助手职责与输出格式
	basePrompt := `你是一个智能数据库助手。你可以帮助用户查询数据、生成 SQL、解释 SQL、分析数据等。

规则：
1. 如果用户想查询数据，生成 SQL 并用 ` + "```sql" + ` 代码块包裹
2. 回复开头必须先输出一个仅供系统解析的结构化元数据块，格式如下：
` + "```tableplus-ai-meta" + `
{"autoExecute":{"enabled":true,"mode":"first_sql_readonly","reason":"user_requested_result"}}
` + "```" + `
- 如果用户明确要“直接返回结果 / 直接执行 / 帮我查数据”，enabled=true
- 如果用户只是要 SQL、解释 SQL、分析 SQL、或者明确说不要执行，enabled=false
- 该元数据块之外，再正常输出给用户看的 Markdown 正文
3. 不要输出 [AUTO_EXECUTE] 之类旧协议标记
4. 使用 Markdown 格式输出，支持表格、列表、代码块等
5. 回答要简洁专业
6. 根据提供的表结构信息生成准确的 SQL
7. 当收到 SQL 执行错误反馈时（以 [SQL_ERROR] 开头的消息），你必须：
   a. 分析错误原因，简要说明问题所在
   b. 生成修复后的 SQL，同样用 ` + "```sql" + ` 代码块包裹
   c. 修复响应的开头也必须输出同样的 ` + "```tableplus-ai-meta" + ` 元数据块；如果需要系统继续执行，enabled=true，否则 enabled=false
   d. 不要重复之前的错误，确保新 SQL 语法和逻辑正确

⚠️ 极其重要 — Schema 使用约束：
- 你只能使用下方 CREATE TABLE 语句中明确定义的表名和列名
- 严禁猜测、推测或使用未在 Schema 中出现的表名或列名
- 如果你不确定某个字段是否存在，不要猜测，应明确告知用户
- 不同的表可能使用不同的软删除字段命名（如 delete_time、deleted_at、is_deleted 等），必须查看具体表的 Schema 确认
- 如果 Schema 中某张表的列信息未列出，请向用户说明你无法获取该表的结构信息`

	// 注入数据库类型和版本，指导 AI 生成兼容的 SQL 语法
	if dbType != "" {
		dbInfo := "\n\n当前数据库类型: " + dbType
		if dbVersion != "" {
			dbInfo += "（版本: " + dbVersion + "）"
		}
		dbInfo += "\n⚠️ 生成 SQL 时必须严格兼容此数据库类型的语法。"
		if dbVersion != "" {
			dbInfo += "\n⚠️ 同时必须考虑该版本能力边界，禁止使用此版本不支持的语法特性。"
		}
		if dbType == "tidb" {
			dbInfo += "\n- TiDB 不完全兼容 MySQL 语法，例如 GROUP_CONCAT 中不支持 ORDER BY 子句，请使用子查询替代。"
		} else if dbType == "starrocks" {
			dbInfo += "\n- StarRocks 是 OLAP 数据库，不支持事务、外键。GROUP_CONCAT 语法与 MySQL 有差异。"
		}
		basePrompt += dbInfo
	}

	if s.customSystemPrompt != "" {
		basePrompt += "\n\n用户自定义会话提示词:\n" + s.customSystemPrompt
	}
	if schemaStr != "" {
		basePrompt += "\n\n当前数据库表结构（DDL 格式）:\n" + schemaStr
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

// buildSchema 构建表结构上下文（含完整列元数据：主键、默认值、外键、注释）
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

	// 获取数据库版本号，用于系统提示词中指导 AI 生成兼容 SQL
	dbVersion := ""
	ver, verErr := database.GetServerVersion(db, cfg.Type)
	if verErr == nil {
		dbVersion = ver
	}

	schema := &ai.SchemaContext{
		DatabaseType:    cfg.Type,
		DatabaseName:    dbName,
		DatabaseVersion: dbVersion,
	}

	for _, t := range tables {
		cols, err := database.GetColumns(db, cfg.Type, dbName, t.Name)
		if err != nil {
			logger.Warn("[AIService] 获取表 %s 列信息失败: %v", t.Name, err)
			continue
		}

		tableSchema := ai.TableSchema{Name: t.Name, Comment: t.Comment}
		for _, c := range cols {
			// 提取默认值字符串
			defaultVal := ""
			if c.DefaultValue != nil {
				defaultVal = *c.DefaultValue
			}
			tableSchema.Columns = append(tableSchema.Columns, ai.ColumnSchema{
				Name:         c.Name,
				Type:         c.Type,
				Nullable:     c.Nullable,
				Comment:      c.Comment,
				IsPrimary:    c.IsPrimary,
				DefaultValue: defaultVal,
				ForeignKey:   c.ForeignKey,
			})
		}
		schema.Tables = append(schema.Tables, tableSchema)
	}

	logger.Info("[AIService] 构建 schema 完成: tables=%d", len(schema.Tables))
	return schema, nil
}
