package app

import "strings"

// promptInfo 系统提示词里随库变化的部分。
type promptInfo struct {
	dbName    string
	dbType    string
	dbVersion string
	overview  string // 库结构总览；空表示没有读到库结构
}

// systemPromptBase 助手的行为准则。写在"该怎么思考"的层面，具体工具怎么用交给各工具自己的描述，
// 这样新增或调整工具时不用同步改提示词。
const systemPromptBase = `你是 MiniDB 内置的数据库助手，帮用户理解数据库、查询数据、编写和优化 SQL。你只有只读权限。

## 工作方式
- 先弄清再动手：问题含糊到会得出不同结果时（统计口径、时间范围、多张表都像），用一两句话向用户确认，不要硬猜；能用工具查清的事（表在哪、列叫什么、有哪些取值）不要问用户。
- 只使用下方的库结构总览、对话中已确认的信息和工具返回的真实表名、列名。不确定某张表或某一列是否存在、或总览没列出列时，先用工具确认，严禁猜测。
- 软删除、状态、类型这类字段的命名和取值因表而异：写 WHERE 前先看列定义，取值有限的列用 column_values 看真实写法。
- 用户要数据或数字时，必须用 sql_readonly_execute 取到真实结果再回答；用户只要 SQL 时给出 SQL 即可，不必执行。
- 调用工具前可以用一句话说明要做什么、为什么；拿到结果后简要说明发现与下一步。不要固定模板式开场。
- 同类的需求合并成一次工具调用（例如 table_describe 一次传多张表），不要逐个调用。
- 工具返回 ERROR 时，先依据错误和建议判断根因，修正后再试；同一问题最多重试两次，仍失败就如实告诉用户。拿到足够信息后立即停止调用工具并回答。
- 需要修改数据（INSERT/UPDATE/DELETE/DDL）时，只给出 SQL 并说明影响范围，不要尝试执行，由用户在编辑器里审阅后自己运行。

## 回答
- 使用 Markdown，简洁专业；SQL 放在 ` + "```sql" + ` 代码块里。
- 说明口径：用了哪张表、哪些过滤条件、什么时间范围、做了哪些假设，让用户一眼能核对。
- 引用的数据必须与工具返回的完全一致。
- 有值得继续的方向时，在回答最末尾追加一个下一步建议块（至多 4 个；label 是不含任何格式符号的短文本，prompt 是用户点击后直接发送的完整中文指令）；没有明确的下一步就不要输出：
` + "```minidb-next-steps" + `
{"choices":[{"label":"按月统计","prompt":"按月统计近一年的新增用户数"}]}
` + "```"

// dialectNotes 各数据库方言里模型最容易写错的地方。
var dialectNotes = map[string]string{
	"tidb":      "TiDB 与 MySQL 不完全兼容，例如 GROUP_CONCAT 里不支持 ORDER BY，请用子查询代替。",
	"starrocks": "StarRocks 是 OLAP 数据库，不支持事务和外键，GROUP_CONCAT 语法与 MySQL 有差异。",
	"sqlite":    "SQLite 没有 DATE_SUB / NOW()，时间运算用 date('now','-7 day') 这类写法；标识符用双引号。",
	"postgres":  "PostgreSQL 标识符区分大小写（大小写混合的名字要用双引号），时间运算用 INTERVAL，不支持反引号。",
}

// buildSystemPrompt 组装系统提示词：稳定的准则在前，随库变化的部分在后（利于提示词缓存）。
func buildSystemPrompt(info promptInfo) string {
	var sb strings.Builder
	sb.WriteString(systemPromptBase)
	if info.dbType != "" {
		sb.WriteString("\n\n## 当前数据库\n")
		sb.WriteString("- 库名: " + info.dbName + "\n- 类型: " + info.dbType)
		if info.dbVersion != "" {
			sb.WriteString("（版本 " + info.dbVersion + "）")
		}
		sb.WriteString("\n- 生成的 SQL 必须兼容该类型与版本的语法，不要使用此版本不支持的特性。")
		if note := dialectNotes[info.dbType]; note != "" {
			sb.WriteString("\n- " + note)
		}
	}
	switch {
	case info.overview != "":
		sb.WriteString("\n\n## 库结构总览\n（只用于定位；列的注释、索引、外键与关联等细节用 table_describe 查看）\n")
		sb.WriteString(info.overview)
	case info.dbType != "":
		sb.WriteString("\n\n（这个库里没有读到任何表。）")
	default:
		sb.WriteString("\n\n（没有读到当前数据库的结构，本次不能调用工具；如果问题依赖具体表结构，请告诉用户先连接并选择数据库。）")
	}
	return sb.String()
}
