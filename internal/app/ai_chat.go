package app

import (
	"context"
	"errors"
	"fmt"
	"minidb/internal/database"
	"strings"
	"time"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/logger"
)

// aiChatTimeout 一次助手对话（含全部工具轮次）的总时限。
const aiChatTimeout = 15 * time.Minute

// ChatAIStream 助手对话：带工具的多轮循环，过程以事件流推送（TopicAgent），结束时返回最终回答。
//
// 参数：connID/dbName 当前连接与库；messages 完整会话历史（最后一条是用户的新问题）；
// requestID 本次请求的标识，事件与取消都用它；sessionID 会话标识，非空时对话会被持久化。
// 返回：{"content": 最终回答}；失败时返回 error（同时已发出 run.error 事件）。
func (s *AIService) ChatAIStream(connID, dbName string, messages []ai.ChatMessage, requestID, sessionID string) (map[string]interface{}, error) {
	defer logger.Perf("app.AI.ChatAIStream", "conn", connID, "db", dbName)()
	if len(messages) == 0 || strings.TrimSpace(messages[len(messages)-1].Content) == "" {
		return nil, errors.New("消息不能为空")
	}
	s.ReloadConfig()
	// 整段对话的总时限：单次请求有 SDK 的超时，但多轮工具循环加起来没有上限，网关卡住时会一直占着。
	ctx, cancel := context.WithTimeout(context.Background(), aiChatTimeout)
	defer cancel()
	defer s.registerCancel(requestID, cancel)()

	run := newChatRun(s, requestID, strings.TrimSpace(sessionID))
	run.saveUserMessage(connID, dbName, messages[len(messages)-1])
	logger.Info("[AIService] 对话开始: connID=%s db=%s requestID=%s session=%s messages=%d", connID, dbName, requestID, sessionID, len(messages))

	var (
		tools    []ai.FunctionToolDefinition
		executor ai.ToolExecutor
		info     = promptInfo{dbName: dbName}
	)
	schema, search, err := s.schema.Index(ctx, connID, dbName)
	if err != nil {
		logger.Warn("[AIService] 加载库结构失败，本次对话不带工具: %v", err)
	} else {
		metadataOnly := s.metadataOnly(connID)
		env := &toolEnv{s: s, connID: connID, dbName: dbName, schema: schema, search: search, metadataOnly: metadataOnly}
		info.metadataOnly = metadataOnly
		reg := env.registry()
		tools, executor = toolDefinitions(reg), run.executor(ctx, reg)
		info.dbType, info.dbVersion, info.overview = database.DBType(schema.DatabaseType), schema.DatabaseVersion, buildSchemaOverview(schema)
		tables, missing := resolveMentions(schema, messages)
		info.mentioned, info.missing = ai.BuildTablesDDL(tables), missing
	}
	run.emit(agent.EventRunStarted, agent.RunStarted{Model: s.client.ModelName(), DBType: string(info.dbType)})

	answer, err := s.client.ChatWithToolsStream(ctx, buildSystemPrompt(info), trimHistory(messages), tools, maxToolRounds, executor, run.callbacks())
	return run.finish(ctx, answer, err)
}

// finish 收尾：发 run.done 或 run.error，保存助手回复，返回给调用方。
func (r *chatRun) finish(ctx context.Context, answer string, err error) (map[string]interface{}, error) {
	switch {
	case ctx.Err() != nil:
		err = fmt.Errorf("已停止: %w", ctx.Err())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("对话超过 %s 仍未完成，已中止；可以缩小问题范围后重试", aiChatTimeout)
		}
		r.emit(agent.EventRunError, agent.RunError{Code: agent.ErrCodeCancelled, Message: err.Error()})
	case err != nil:
		logger.Error("[AIService] 对话失败: %v", err)
		code, friendly := agent.ErrCodeFailed, err
		if ai.IsRateLimit(err) {
			code, friendly = agent.ErrCodeRateLimited, friendlyAIError(err)
		}
		r.emit(agent.EventRunError, agent.RunError{Code: code, Message: friendly.Error()})
		err = friendly
	default:
		body, suggestions := agent.SplitSuggestions(answer)
		if body == "" {
			// 模型自始至终没写出回答：明确报错，让界面给出"重试"，而不是留一条空消息
			err = errors.New("模型没有返回回答（工具结果已在步骤里），请重试或换一种问法")
			r.emit(agent.EventRunError, agent.RunError{Code: agent.ErrCodeEmptyAnswer, Message: err.Error()})
			break
		}
		usage, rounds := r.usageSnapshot()
		r.emit(agent.EventRunDone, agent.RunDone{Content: body, Rounds: rounds, Usage: usage, Suggestions: suggestions})
		r.saveAssistantMessage()
		logger.Info("[AIService] 对话完成: rounds=%d tokens=%d answer_len=%d", rounds, usage.TotalTokens, len(body))
		return map[string]interface{}{"content": body}, nil
	}
	r.saveAssistantMessage()
	return nil, err
}

// metadataOnly 这个连接是否设置了"只共享库结构"。连接不存在（已断开、被删）时按最严的处理。
func (s *AIService) metadataOnly(connID string) bool {
	if s.manager == nil {
		return false
	}
	cfg, ok := s.manager.GetConfig(connID)
	return !ok || cfg.AIMetadataOnly
}
