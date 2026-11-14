// @author ixxxxoooo

package database

// PromptHinter 由方言实现，返回给 AI 的该数据库最容易写错的语法提示；没有实现的方言不提示。
type PromptHinter interface{ PromptHint() string }

// PromptHintOf 返回类型的 AI 语法提示；未知类型或没有提示时返回空串。
func PromptHintOf(t DBType) string {
	if d, ok := dialects[t]; ok {
		if h, ok := d.(PromptHinter); ok {
			return h.PromptHint()
		}
	}
	return ""
}

func (mariadbDialect) PromptHint() string {
	return "MariaDB 与 MySQL 大体兼容，但 JSON 列其实是 LONGTEXT，MySQL 8 的 ->> 操作符和部分 JSON 函数不可用；CTE 与窗口函数需要 10.2 以上；标识符用反引号。"
}

func (tidbDialect) PromptHint() string {
	return "TiDB 与 MySQL 不完全兼容，例如 GROUP_CONCAT 里不支持 ORDER BY，请用子查询代替。"
}

func (starrocksDialect) PromptHint() string {
	return "StarRocks 是 OLAP 数据库，不支持事务和外键，GROUP_CONCAT 语法与 MySQL 有差异。"
}

func (sqlserverDialect) PromptHint() string {
	return "SQL Server 用 T-SQL：标识符用方括号，取前 N 行用 TOP N 或 OFFSET ... FETCH（需要 ORDER BY），字符串连接用 + 或 CONCAT，当前时间用 GETDATE()，没有 LIMIT。"
}

func (clickhouseDialect) PromptHint() string {
	return "ClickHouse 是列式分析库：没有事务和外键，UPDATE/DELETE 要写成 ALTER TABLE ... UPDATE / DELETE WHERE，字符串用单引号，标识符用反引号，取前 N 行用 LIMIT，日期函数有 toDate、toStartOfMonth、dateDiff，聚合有 uniq、countIf。"
}

func (sqliteDialect) PromptHint() string {
	return "SQLite 没有 DATE_SUB / NOW()，时间运算用 date('now','-7 day') 这类写法；标识符用双引号。"
}

func (postgresDialect) PromptHint() string {
	return "PostgreSQL 标识符区分大小写（大小写混合的名字要用双引号），时间运算用 INTERVAL，不支持反引号。"
}
