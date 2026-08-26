package services

import (
	"context"
	"encoding/json"
	"fmt"

	"minidb/internal/agent"
	"minidb/internal/ai"
)

// toolSpec 是工具的唯一事实来源：name/description/参数 schema/只读属性。
// 由它派生：LLM 的 FunctionToolDefinition、前端 @tool 列表、Agent 注册表。
type toolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
	ReadOnly    bool
	ResultKind  string // 前端提示：rows/text
}

// toolSpecs AI 工具定义清单（增删工具只改这里）。
var toolSpecs = []toolSpec{
	{
		Name: "table_fuzzy_match",
		Description: "按关键词匹配潜在相关表名.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keywords": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
			},
			"required":             []string{"keywords", "limit"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "column_fuzzy_match",
		Description: "按关键词匹配潜在相关字段名、字段注释和字段类型。table_names 可传空数组表示全库搜索。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keywords":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": columnFuzzyMaxMatches},
			},
			"required":             []string{"keywords", "table_names", "limit"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "table_describe",
		Description: "查看指定表字段定义",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required":             []string{"table_names"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "table_relationships",
		Description: "查看指定表的显式外键、反向外键，以及基于 *_id 字段命名推断的疑似关联。推断关系必须二次确认。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_names": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "string"},
					"maxItems": tableRelationshipsMaxTables,
				},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": tableRelationshipsMaxItems},
			},
			"required":             []string{"table_names", "limit"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "sql_readonly_execute",
		Description: "执行只读 SQL 查询并返回结果摘要",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql":          map[string]any{"type": "string"},
				"table_names":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"limit":        map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
			},
			"required":             []string{"sql", "table_names", "limit"},
			"additionalProperties": false,
		},
		ReadOnly:   true,
		ResultKind: "rows",
	},
	{
		Name: "table_ddl",
		Description: "查看指定表建表语句",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required":             []string{"table_names"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "table_stats",
		Description: "查看指定表统计信息（行数、大小等）。单次最多传 20 张表；后端最多 6 并发获取统计；如果要看多张表，请尽量一次性放入 table_names。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_names": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "string"},
					"maxItems": tableStatsMaxTables,
				},
			},
			"required":             []string{"table_names"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "table_sample",
		Description: "安全抽样查看指定表前 N 行。只接受 table_name 和 limit，后端只生成 SELECT * FROM <table> LIMIT n，不接受 WHERE。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_name": map[string]any{"type": "string"},
				"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": tableSampleMaxRows},
			},
			"required":             []string{"table_name", "limit"},
			"additionalProperties": false,
		},
		ReadOnly:   true,
		ResultKind: "rows",
	},
	{
		Name: "table_profile",
		Description: "查看表字段画像：row count、null count、distinct count，数字列补 min/max。单次最多 3 张表、每表最多 8 列；后端字段统计最多 5 并发。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_names": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "string"},
					"maxItems": tableProfileMaxTables,
				},
				"columns": map[string]any{
					"type":     "array",
					"items":    map[string]any{"type": "string"},
					"maxItems": tableProfileMaxColumns,
				},
			},
			"required":             []string{"table_names", "columns"},
			"additionalProperties": false,
		},
		ReadOnly: true,
	},
	{
		Name: "sql_explain_plan",
		Description: "为单条只读 SQL 生成 EXPLAIN 执行计划。拒绝写操作、多语句和 EXPLAIN ANALYZE。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string"},
			},
			"required":             []string{"sql"},
			"additionalProperties": false,
		},
		ReadOnly:   true,
		ResultKind: "rows",
	},
}

// orderedToolNames 保持工具注册/展示顺序（与前端 @tool 联想一致）。
// 定义在 ai_tools.go（兼容旧 buildFunctionToolDefinitions 使用）。

// buildAgentToolRegistry 构建 Agent 工具注册表（handler 复用 execToolXX 执行逻辑，
// 但以结构化 *agent.ToolResult 输出，供 v2 事件与前端渲染）。
// connID/dbName/userQuestion/schema 是该 run 的上下文，通过闭包绑定。
func (s *AIService) buildAgentToolRegistry(connID, dbName, userQuestion string, schema *ai.SchemaContext) *agent.ToolRegistry {
	reg := agent.NewToolRegistry()
	mentions := parseMentions(userQuestion)
	for _, spec := range toolSpecs {
		spec := spec
		toolName := spec.Name
		reg.MustRegister(agent.Tool{
			Name:        toolName,
			Description: spec.Description,
			Parameters:  spec.Parameters,
			ReadOnly:    spec.ReadOnly,
			ResultKind:  spec.ResultKind,
			Handler: func(ctx context.Context, input agent.ToolInput) *agent.ToolResult {
				args := aiToolCallArgsFromInput(input)
				exec := s.executeTool(ctx, toolName, connID, dbName, userQuestion, schema, mentions, args)
				return toolResultFromExec(exec)
			},
		})
	}
	return reg
}

// aiToolCallArgsFromInput 从注册表参数 map 构造旧 execToolXX 使用的参数。
func aiToolCallArgsFromInput(input agent.ToolInput) aiToolCallArgs {
	return aiToolCallArgs{
		TableName:  input.String("table_name"),
		TableNames: input.Strings("table_names"),
		Columns:    input.Strings("columns"),
		Keywords:   input.Strings("keywords"),
		Limit:      input.Int("limit"),
		SQL:        input.String("sql"),
	}
}

// agentToolDefinitions 从注册表派生 LLM 可见的 FunctionToolDefinition。
func agentToolDefinitions(reg *agent.ToolRegistry) []ai.FunctionToolDefinition {
	tools := reg.Visible()
	defs := make([]ai.FunctionToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, ai.FunctionToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return defs
}

// buildFunctionToolDefinitionsFromSpecs 由 toolSpecs 派生 LLM FunctionToolDefinition（兼容旧入口）。
func buildFunctionToolDefinitionsFromSpecs() []ai.FunctionToolDefinition {
	defs := make([]ai.FunctionToolDefinition, 0, len(toolSpecs))
	for _, spec := range toolSpecs {
		defs = append(defs, ai.FunctionToolDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return defs
}

// agentToolList 从注册表派生前端 @tool 联想列表。
func agentToolList(reg *agent.ToolRegistry) []AIToolDefinition {
	tools := reg.List()
	out := make([]AIToolDefinition, 0, len(tools))
	for _, t := range tools {
		out = append(out, AIToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			ReadOnly:    t.ReadOnly,
			ResultKind:  t.ResultKind,
		})
	}
	return out
}

// validateToolArgumentJSON 校验模型返回的工具参数 JSON（供注册表 Validate 使用）。
func toolArgumentsFromJSON(arguments string) (agent.ToolInput, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(arguments), &raw); err != nil {
		return nil, fmt.Errorf("工具参数不是合法 JSON: %v", err)
	}
	return agent.NormalizeArguments(raw), nil
}