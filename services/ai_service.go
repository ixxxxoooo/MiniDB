package services

import (
	"context"
	"fmt"
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
	query   *QueryService // 供 sql_readonly_execute 工具执行只读查询
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

// ChatStreamEvent 流式事件结构（ReAct 模式：AI 边思考边调工具边分析）
type ChatStreamEvent struct {
	RequestID  string `json:"requestId"`
	Type       string `json:"type"` // delta/done/error/status/tool_start/tool_sql/tool_result/tool_error/thinking/final_answer
	Delta      string `json:"delta,omitempty"`
	Content    string `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolState  string `json:"toolState,omitempty"`
	ToolInput  string `json:"toolInput,omitempty"`
	ToolSQL    string `json:"toolSql,omitempty"`
	ToolOutput string `json:"toolOutput,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	// AI 在工具调用间输出的真实推理/分析内容
	ThinkingContent string `json:"thinkingContent,omitempty"`
}

// ChatAutoExecuteDirective 保留兼容历史结构，当前主会话链路已不再使用自动执行。
type ChatAutoExecuteDirective struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

const (
	schemaCacheTTL    = 5 * time.Minute
	maxChatContextMsg = 12
)

// NewAIService 创建 AI 服务（query 供 sql_readonly_execute 工具执行只读查询）
func NewAIService(manager *database.Manager, store *storage.Store, query *QueryService) *AIService {
	// 从存储中加载 AI 配置
	var cfg ai.Config
	err := store.Get("settings", "ai_config", &cfg)
	if err != nil {
		cfg = ai.Config{
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o",
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

// ChatAI 会话式 AI 助手（非流式），支持 ReAct 多轮工具调用
func (s *AIService) ChatAI(connID, dbName string, messages []ai.ChatMessage) (map[string]interface{}, error) {
	s.ReloadConfig()
	logger.Info("[AIService] ChatAI 开始: connID=%s dbName=%s messages_count=%d", connID, dbName, len(messages))

	userQuestion := extractUserQuestion(messages)
	schemaStr, dbType, dbVersion, schema := s.loadSchemaContext(connID, dbName, userQuestion)
	systemPrompt := s.buildChatSystemPrompt(schemaStr, dbType, dbVersion)

	var tools []ai.FunctionToolDefinition
	if schema != nil {
		tools = BuildAllToolDefinitions()
	}

	contextMessages := trimContextMessages(messages)
	executor := s.buildToolExecutor(connID, dbName, userQuestion, schema)

	resp, err := s.client.ChatWithToolsStream(
		context.Background(),
		systemPrompt,
		contextMessages,
		tools,
		maxToolCallRounds,
		executor,
		ai.ToolStreamCallbacks{},
	)
	if err != nil {
		logger.Error("[AIService] ChatAI 失败: %v", err)
		return nil, err
	}

	_, cleanResp, _ := database.ExtractAutoExecuteMetaBlock(resp)
	logger.Info("[AIService] ChatAI 成功: response_len=%d", len(cleanResp))
	return map[string]interface{}{
		"content": cleanResp,
	}, nil
}

// ChatAIStream 会话式 AI 助手（流式输出），ReAct 模式：AI 边思考边调工具边分析
func (s *AIService) ChatAIStream(connID, dbName string, messages []ai.ChatMessage, requestID string) (map[string]interface{}, error) {
	s.ReloadConfig()
	logger.Info("[AIService] ChatAIStream 开始: connID=%s dbName=%s requestID=%s messages_count=%d", connID, dbName, requestID, len(messages))

	userQuestion := extractUserQuestion(messages)

	// 推送进度：正在加载表结构
	s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "loading_schema"})
	schemaStr, dbType, dbVersion, schema := s.loadSchemaContext(connID, dbName, userQuestion)
	systemPrompt := s.buildChatSystemPrompt(schemaStr, dbType, dbVersion)

	var tools []ai.FunctionToolDefinition
	if schema != nil {
		tools = BuildAllToolDefinitions()
	}

	contextMessages := trimContextMessages(messages)
	logger.Info("[AIService] ChatAIStream 上下文裁剪: 原始=%d 裁剪后=%d", len(messages), len(contextMessages))

	// 推送进度：AI 开始推理
	s.emitStreamEvent(ChatStreamEvent{RequestID: requestID, Type: "status", Delta: "calling_ai"})
	simulateThinking := !s.client.IsReasoningModel()
	logger.Info("[AIService] ChatAIStream thinking_mode: model=%s simulate=%v", s.client.ModelName(), simulateThinking)
	if simulateThinking {
		s.emitThinkingEvent(requestID, "正在理解你的问题并规划查询步骤...")
	}

	// 构建工具执行器
	executor := s.buildToolExecutor(connID, dbName, userQuestion, schema)

	// 构建 ReAct 流式回调：将 AI 的思考、工具调用、最终回答事件推送给前端
	callbacks := ai.ToolStreamCallbacks{
		OnThinking: func(content string) {
			logger.Debug("[AIService] ReAct 思考内容: %s", content)
			s.emitThinkingEvent(requestID, content)
		},
		OnToolCall: func(call ai.FunctionToolCall) {
			logger.Info("[AIService] ReAct 工具调用开始: tool=%s callID=%s", call.Name, call.ID)
			if simulateThinking {
				tool := strings.TrimSpace(call.Name)
				if tool == "" {
					tool = "unknown_tool"
				}
				s.emitThinkingEvent(requestID, fmt.Sprintf("准备调用工具 `%s` 获取关键信息...", tool))
			}
			s.emitStreamEvent(ChatStreamEvent{
				RequestID:  requestID,
				Type:       "tool_start",
				ToolName:   call.Name,
				ToolCallID: call.ID,
				ToolState:  "running",
				ToolInput:  call.Arguments,
			})
		},
		OnToolResult: func(callID, toolName, result string, durationMs int64) {
			logger.Info("[AIService] ReAct 工具执行完成: tool=%s callID=%s duration=%dms", toolName, callID, durationMs)
			displayTool := strings.TrimSpace(toolName)
			if displayTool == "" {
				displayTool = "unknown_tool"
			}
			if strings.HasPrefix(result, "ERROR:") {
				if simulateThinking {
					errMsg := strings.TrimSpace(strings.TrimPrefix(result, "ERROR:"))
					if len(errMsg) > 120 {
						errMsg = errMsg[:120] + "..."
					}
					if errMsg == "" {
						errMsg = "未知错误"
					}
					s.emitThinkingEvent(requestID, fmt.Sprintf("工具 `%s` 调用失败（%s），正在分析原因并修正查询...", displayTool, errMsg))
				}
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:  requestID,
					Type:       "tool_error",
					ToolName:   toolName,
					ToolCallID: callID,
					ToolState:  "error",
					ToolOutput: result,
					DurationMs: durationMs,
				})
			} else {
				if simulateThinking {
					s.emitThinkingEvent(requestID, fmt.Sprintf("已拿到工具 `%s` 的结果，继续推理...", displayTool))
				}
				s.emitStreamEvent(ChatStreamEvent{
					RequestID:  requestID,
					Type:       "tool_result",
					ToolName:   toolName,
					ToolCallID: callID,
					ToolState:  "success",
					ToolOutput: result,
					DurationMs: durationMs,
				})
			}
		},
		OnFinalAnswer: func() {
			logger.Info("[AIService] ReAct 最终回答阶段开始")
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "final_answer",
			})
		},
		OnDelta: func(delta string) {
			s.emitStreamEvent(ChatStreamEvent{
				RequestID: requestID,
				Type:      "delta",
				Delta:     delta,
			})
		},
	}

	resp, err := s.client.ChatWithToolsStreamRealtime(
		context.Background(),
		systemPrompt,
		contextMessages,
		tools,
		maxToolCallRounds,
		executor,
		callbacks,
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

	_, cleanResp, _ := database.ExtractAutoExecuteMetaBlock(resp)
	s.emitStreamEvent(ChatStreamEvent{
		RequestID: requestID,
		Type:      "done",
		Content:   cleanResp,
	})
	logger.Info("[AIService] ChatAIStream 成功: response_len=%d", len(cleanResp))
	return map[string]interface{}{
		"content": cleanResp,
	}, nil
}

// loadSchemaContext 加载数据库 Schema 上下文（提取公共逻辑）
func (s *AIService) loadSchemaContext(connID, dbName, userQuestion string) (schemaStr, dbType, dbVersion string, schema *ai.SchemaContext) {
	if connID == "" || dbName == "" {
		return "", "", "", nil
	}
	var err error
	schema, err = s.getSchemaWithCache(connID, dbName)
	if err != nil {
		logger.Warn("[AIService] 加载数据库 schema 失败: %v", err)
		return "", "", "", nil
	}
	schemaStr = buildSchemaForChat(schema, userQuestion)
	dbType = schema.DatabaseType
	dbVersion = schema.DatabaseVersion
	logger.Debug("[AIService] 数据库 schema 已加载: tables_count=%d schema_len=%d dbType=%s", len(schema.Tables), len(schemaStr), dbType)
	return
}

// buildToolExecutor 构建 ReAct 工具执行器闭包，捕获连接上下文
func (s *AIService) buildToolExecutor(connID, dbName, userQuestion string, schema *ai.SchemaContext) ai.ToolExecutor {
	if schema == nil {
		return nil
	}
	return func(call ai.FunctionToolCall) string {
		result := s.ExecuteToolFromAICall(call, connID, dbName, userQuestion, schema)
		if result.Err != nil {
			logger.Warn("[AIService] ReAct 工具执行失败: tool=%s err=%v", call.Name, result.Err)
			return "ERROR: " + result.Err.Error()
		}
		return result.ToolOutput
	}
}

func (s *AIService) emitThinkingEvent(requestID, content string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}
	s.emitStreamEvent(ChatStreamEvent{
		RequestID:       requestID,
		Type:            "thinking",
		ThinkingContent: trimmed,
	})
}

// buildChatAutoExecuteDirective 保留兼容历史逻辑（测试与旧接口依赖），主流程已不再调用。
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
func buildSchemaForChat(schema *ai.SchemaContext, userQuestion string) string {
	tableCount := len(schema.Tables)
	logger.Info("[AIService] 分层 Schema 策略: tables=%d", tableCount)

	if tableCount <= schemaSmallThreshold {
		logger.Info("[AIService] 小库策略: 全量 DDL (%d 张表)", tableCount)
		return ai.BuildSchemaDDL(schema)
	}

	if tableCount <= schemaMediumThreshold {
		relevant := filterRelevantTables(schema.Tables, userQuestion)
		logger.Info("[AIService] 中库策略: 关键词匹配 %d/%d 张相关表", len(relevant), tableCount)

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
func filterRelevantTables(tables []ai.TableSchema, userQuestion string) []ai.TableSchema {
	if userQuestion == "" {
		return nil
	}

	questionLower := strings.ToLower(userQuestion)
	matched := make(map[string]bool)
	var result []ai.TableSchema

	for _, t := range tables {
		nameLower := strings.ToLower(t.Name)
		if strings.Contains(questionLower, nameLower) {
			matched[t.Name] = true
			result = append(result, t)
			continue
		}
		shortName := stripTablePrefix(nameLower)
		if shortName != nameLower && len(shortName) >= 3 && strings.Contains(questionLower, shortName) {
			matched[t.Name] = true
			result = append(result, t)
		}
	}

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
			if commentLower != "" && strings.Contains(commentLower, kw) {
				matched[t.Name] = true
				result = append(result, t)
				break
			}
			if len(kw) >= 3 && strings.Contains(shortName, kw) {
				matched[t.Name] = true
				result = append(result, t)
				break
			}
		}
	}

	fkTables := make(map[string]bool)
	for _, t := range result {
		for _, c := range t.Columns {
			if c.ForeignKey != "" {
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

	if len(result) > maxRelevantTables {
		result = result[:maxRelevantTables]
	}

	return result
}

func stripTablePrefix(name string) string {
	prefixes := []string{"tbl_", "tbl", "tb_", "t_", "sys_"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}

func extractKeywords(question string) []string {
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

func extractUserQuestion(messages []ai.ChatMessage) string {
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
	basePrompt := `你是一个智能数据库助手。你可以帮助用户查询数据、生成 SQL、解释 SQL、分析数据等。

	你拥有以下工具能力，可以在需要时自主调用：
	- table_fuzzy_match: 按关键词模糊匹配数据库中的表名
	- table_describe: 查看指定表的字段定义和注释
	- table_ddl: 查看指定表的建表语句
	- table_stats: 查看指定表的行数与统计信息
	- sql_readonly_execute: 执行只读 SQL 查询并返回结果
	
	【新增：ReAct 执行流程 - 这是实现循环调用的关键】
	你的工作流程必须遵循以下循环（Think → Act → Observe → Repeat）：
	
	1. **思考 (Thought)**：分析用户问题，判断当前已掌握的信息是否足够回答。如果缺少表结构、数据内容或统计信息，明确规划需要调用哪个工具。
	2. **行动 (Action)**：如果信息不足，每次只调用**一个**最必要的工具。输出工具调用后必须立即停止，等待返回结果。
	3. **观察 (Observe)**：工具返回结果后（系统会主动提供），基于返回内容再次进入思考阶段，判断：
	   - 信息已充足 → 给出最终回答
	   - 仍需更多信息 → 再次执行步骤 1（新一轮 Thought → Action）
	4. **重复**：直到获取足够信息或确认无法获取为止。
	
	⚠️ 关键约束：每次回复你只能选择以下之一：
	   - 调用工具（此时不要输出最终答案）
	   - 给出最终回答（此时不要再调用工具）
	
	工具使用格式（让系统能识别你的调用意图）：
	当你需要调用工具时，使用以下格式输出，然后立即停止：
	` + "```tool" + `
	{"tool": "tool_name", "params": {"key": "value"}}
	` + "```" + `
	
	规则：
	1. 如果用户明确要"直接查数 / 直接给结果"，你必须在最终回答前主动调用 sql_readonly_execute 获取真实结果，再基于工具结果回答。
	2. 如果用户只要 SQL（不要求执行），则给出 SQL 并用 ` + "```sql" + ` 代码块包裹，不要调用查询工具。
	3. 使用 Markdown 格式输出，支持表格、列表、代码块等。
	4. 回答要简洁专业。
	5. 直接进入解答，不要输出固定模板开场（例如"问题复述：""我的理解：""分析："等）。
	6. 根据提供的表结构信息生成准确的 SQL。
	7. 当收到 SQL 执行错误反馈时（以 [SQL_ERROR] 开头的消息），你必须：
	   a. 分析错误原因，简要说明问题所在
	   b. 生成修复后的 SQL，同样用 ` + "```sql" + ` 代码块包裹
	   c. 不要重复之前的错误，确保新 SQL 语法和逻辑正确
	8. 在多轮工具调用时，严格按以下循环执行（与上方 ReAct 流程一致）：
	   a. **思考**：当前还缺哪些信息、是否需要调用工具、调用哪个工具最优先
	   b. **行动**：只调用本轮最必要的一个工具，使用上述 ` + "```tool" + ` 格式，然后停止输出等待返回
	   c. **观察**：收到工具返回后，分析结果是否充分
	   d. **判断**：信息充分后再输出最终结论；若用户要"直接结果/结果分析"，必须确保已通过 sql_readonly_execute 获取真实数据后再回答
	   e. **循环**：若信息不足，回到步骤 a 继续下一轮
	9. 若任一工具返回 ` + "`ERROR:`" + `：
	   a. 必须先基于该错误做根因判断，再给出修正后的 SQL 并优先再次调用 sql_readonly_execute 验证
	   b. 不要在修复前插入无关的新查询
	   c. 一旦核心问题所需结果已获取，立即停止继续调用工具并输出最终答案
	10. 如果你会给出下一步建议，必须在回答末尾追加如下唯一结构化块（仅此一种格式）：
	` + "```tableplus-ai-next-steps" + `
	{"choices":[{"label":"选项文案","prompt":"用户点击后应发送的完整下一句"}]}
	` + "```" + `
		- choices 最多 4 个
		- label 用于按钮展示，简短明确
		- prompt 必须是和你给出的建议的选项一致，内容不能为空，且必须是可直接执行的中文指令
		- label 必须是纯文本，禁止使用反引号、引号包裹、Markdown 强调等格式符号
		- 若不需要下一步选择，则不要输出该块
	
	⚠️ 极其重要 — Schema 使用约束：
	- 你只能使用下方 CREATE TABLE 语句中明确定义的表名和列名
	- 严禁猜测、推测或使用未在 Schema 中出现的表名或列名
	- 如果你不确定某个字段是否存在，不要猜测，应明确告知用户
	- 不同的表可能使用不同的软删除字段命名（如 delete_time、deleted_at、is_deleted 等），必须查看具体表的 Schema 确认
	- 如果 Schema 中某张表的列信息未列出，请向用户说明你无法获取该表的结构信息
	
	工具使用指导：
	- 当用户问题涉及具体表但你对表结构不完全确定时，先用 table_describe 查看表结构
	- 当用户需要查看实际数据或统计结果时，用 sql_readonly_execute 执行查询
	- 当需要搜索可能相关的表时，用 table_fuzzy_match 进行模糊匹配
	- 工具返回的数据是真实的数据库查询结果，在回答中引用数据时必须与工具返回内容一致
	- 【新增】永远不要假设工具返回的内容，每次调用后必须等待真实返回结果再决定下一步`

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
