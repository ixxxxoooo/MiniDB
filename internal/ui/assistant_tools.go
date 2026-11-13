package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// toolLabel 是工具在界面上的名字。
func toolLabel(name string) string {
	switch name {
	case "table_list":
		return locales.AiToolTableList()
	case "schema_search":
		return locales.AiToolSchemaSearch()
	case "table_describe":
		return locales.AiToolTableDescribe()
	case "table_stats":
		return locales.AiToolTableStats()
	case "table_sample":
		return locales.AiToolTableSample()
	case "column_values":
		return locales.AiToolColumnValues()
	case "table_profile":
		return locales.AiToolTableProfile()
	case "sql_readonly_execute":
		return locales.AiToolSqlReadonlyExecute()
	case "sql_explain_plan":
		return locales.AiToolSqlExplainPlan()
	}
	return name
}

// toolIcon 给每类工具一个一眼能认的图标：浏览、结构、数据、执行。
func toolIcon(name string) *ui.SVG {
	switch name {
	case "table_list":
		return icTable
	case "schema_search":
		return icSearch
	case "table_describe":
		return icColumns
	case "table_sample":
		return icTable
	case "table_stats", "table_profile":
		return icBars
	case "column_values":
		return icFilter
	case "sql_explain_plan":
		return icActivity
	case "sql_readonly_execute":
		return icPlay
	}
	return icBot
}

// toolTarget 把工具参数里最能说明"对什么做"的部分挑出来：表名、列名、关键词。
func toolTarget(args map[string]any) string {
	var parts []string
	add := func(v any) {
		switch x := v.(type) {
		case string:
			if x != "" {
				parts = append(parts, x)
			}
		case []any:
			for _, e := range x {
				if s, ok := e.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	add(args["table_name"])
	add(args["table_names"])
	if col, ok := args["column"].(string); ok && col != "" && len(parts) > 0 {
		parts[0] += "." + col
	}
	add(args["keyword"])
	add(args["keywords"])
	return strings.Join(parts, ", ")
}

// argLines 把工具参数写成 "名称: 值" 的几行，展开步骤时显示。
// SQL 在步骤里单独展示，这里略过；没传的可选参数（null）也略过。
func argLines(args map[string]any) []string {
	keys := make([]string, 0, len(args))
	for k := range args {
		if k != "sql" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		var val string
		switch x := args[k].(type) {
		case nil:
			continue
		case []any:
			items := make([]string, 0, len(x))
			for _, e := range x {
				items = append(items, fmt.Sprint(e))
			}
			val = strings.Join(items, ", ")
		default:
			val = fmt.Sprint(x)
		}
		if val != "" {
			lines = append(lines, k+": "+val)
		}
	}
	return lines
}
