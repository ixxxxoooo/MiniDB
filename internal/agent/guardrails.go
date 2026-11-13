package agent

import (
	"strings"

	"minidb/internal/database"
)

// GuardReadOnlySQL 校验 SQL 是否为可由助手自动执行的单条只读语句。
// 放行返回 nil；拒绝返回带原因和改法的失败结果，工具处理函数直接把它当结果返回。
// 判定本身委托给 database.CheckReadOnlySQL，全项目只有这一套只读规则。
func GuardReadOnlySQL(sql string) *ToolResult {
	check := database.CheckReadOnlySQL(sql)
	if check.Allowed {
		return nil
	}
	verb := strings.ToUpper(check.Verb)
	switch check.ReasonCode {
	case "empty_sql":
		return Fail("guarded", "SQL 为空", "提供要执行的 SELECT 语句")
	case "multi_sql":
		return Fail("guarded", "一次只能执行一条 SQL", "拆成多次调用，每次一条")
	case "danger_function":
		return Fail("guarded", "SQL 含有有副作用或会读写服务器文件的函数/关键字（如 SLEEP、LOAD_FILE、INTO OUTFILE）", "去掉它们，只做查询")
	case "unknown_sql":
		return Fail("guarded", "只支持 SELECT / WITH / SHOW / DESCRIBE / EXPLAIN，收到 "+verb, "改写成这几类语句之一")
	default:
		return Fail("guarded", "助手只有只读权限，不能执行 "+verb+" 或含写入/加锁的语句", "把 SQL 写在回答里交给用户审阅，由用户在编辑器里执行；不要再调用工具执行它")
	}
}
