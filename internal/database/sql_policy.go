package database

import (
	"minidb/internal/sqltext"
	"regexp"
	"strings"
)

// AutoExecutableCheckResult 自动执行 SQL 的安全校验结果（reasonCode 供前端 i18n）
type AutoExecutableCheckResult struct {
	Allowed    bool   `json:"allowed"`
	ReasonCode string `json:"reasonCode,omitempty"` // empty_sql, multi_sql, risky_sql, unknown_sql
	Verb       string `json:"verb,omitempty"`
}

// ---------------------------------------------------------------------------
// 只读 SQL 护栏（AI 自动执行通道的唯一判定入口）
//
// 设计原则：宁可误拒，不可漏放。
//   - 先把注释、字符串、引用标识符从语句里剥掉，只在剩下的"骨架"上找关键字，
//     这样字符串里的 'delete' 不会误报，而注释里藏的动词也不会被当成首个动词。
//   - 不同方言对反斜杠转义的解释不同（MySQL 把 \' 当转义，PostgreSQL 默认不是）。
//     同一段文本按两种解释可能得到不同的语句边界，所以两种解释都要通过才放行。
//   - 方言特有、无法确定是否为注释/字符串的写法（# 注释、$$ 字符串）一律当作代码扫描。
// ---------------------------------------------------------------------------

var (
	reSkeletonLeadingWord = regexp.MustCompile(`^[a-zA-Z]+`)
	// 出现在 SELECT/WITH/EXPLAIN 语句体内即意味着会写数据或加锁的关键字。
	// 不含 replace：REPLACE(...) 是常用字符串函数，而 REPLACE INTO 已被 into 覆盖。
	reWriteKeywordInBody = regexp.MustCompile(`(?i)\b(insert|update|delete|merge|truncate|drop|alter|create|grant|revoke|call|into)\b|\bfor\s+share\b|\block\s+in\s+share\s+mode\b`)
	// 会产生副作用、读写服务器文件或长时间占用连接的函数/关键字。
	reDangerousCall = regexp.MustCompile(`(?i)\b(sleep|pg_sleep|benchmark|load_file|master_pos_wait|get_lock|release_lock|release_all_locks|pg_read_file|pg_read_binary_file|pg_ls_dir|pg_terminate_backend|pg_cancel_backend|pg_reload_conf|pg_advisory_lock|lo_import|lo_export|dblink|dblink_exec|nextval|setval|set_config)\s*\(|\b(shutdown|outfile|dumpfile|sys_exec|sys_eval|xp_cmdshell)\b`)
)

// CheckReadOnlySQL 判断一段文本是否为可以安全自动执行的单条只读 SQL。
// ReasonCode: empty_sql / multi_sql / risky_sql / unknown_sql / danger_function
func CheckReadOnlySQL(sql string) AutoExecutableCheckResult {
	if strings.TrimSpace(sql) == "" {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "empty_sql"}
	}
	// 每种词法解释都必须通过
	var last AutoExecutableCheckResult
	for _, lex := range readOnlyLexVariants {
		skeleton, executableComment := sqlSkeleton(sql, lex)
		last = checkReadOnlySkeleton(skeleton, executableComment)
		if !last.Allowed {
			return last
		}
	}
	return last
}

func checkReadOnlySkeleton(skeleton string, executableComment bool) AutoExecutableCheckResult {
	text := strings.TrimSpace(skeleton)
	text = strings.TrimSpace(strings.TrimRight(text, "; \t\r\n"))
	if text == "" {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "empty_sql"}
	}

	// 形如 (SELECT ...) UNION (SELECT ...) 的语句以括号开头
	verbSource := strings.TrimLeft(text, "( \t\r\n")
	verb := strings.ToLower(reSkeletonLeadingWord.FindString(verbSource))

	if strings.Contains(text, ";") {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "multi_sql", Verb: strings.ToUpper(verb)}
	}
	if verb == "" {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "unknown_sql"}
	}
	if !sqltext.IsReadOnlyVerb(verb) {
		if sqltext.IsRiskyVerb(verb) {
			return AutoExecutableCheckResult{Allowed: false, ReasonCode: "risky_sql", Verb: strings.ToUpper(verb)}
		}
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "unknown_sql", Verb: strings.ToUpper(verb)}
	}

	// MySQL 的 /*! ... */ 会被服务器当作代码执行，无法静态判断，直接拒绝
	if executableComment || reDangerousCall.MatchString(text) {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "danger_function", Verb: strings.ToUpper(verb)}
	}

	// SHOW 语句不会写数据；其余只读动词的语句体内不允许出现写关键字：
	//   EXPLAIN ANALYZE DELETE ...（会真实执行）、WITH x AS (...) DELETE ...、
	//   SELECT ... INTO ...、SELECT ... FOR UPDATE 等。
	if verb != "show" && reWriteKeywordInBody.MatchString(text) {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "risky_sql", Verb: strings.ToUpper(verb)}
	}
	return AutoExecutableCheckResult{Allowed: true, Verb: strings.ToUpper(verb)}
}

// sqlSkeleton 去掉注释，并把字符串字面量与引用标识符的内容替换为占位符，返回只含"代码"的骨架。
// 词法由 sqltext.Lex 统一提供：这里只负责选出"最保守"的解释方式，
// 不再维护一份与分句器口径不同的扫描器。
func sqlSkeleton(sql string, o sqltext.LexOptions) (skeleton string, executableComment bool) {
	return sqltext.Skeleton(sql, o)
}

// readOnlyLexVariants 列出护栏要逐一通过的词法解释：
// 同一段文本在不同方言下可能有不同的语句边界，所以反斜杠转义、块注释嵌套的各种组合都要通过。
// # 与 $$ 在两种解释里都按代码扫描，--、未闭合注释同样取保守解释。
var readOnlyLexVariants = func() []sqltext.LexOptions {
	var out []sqltext.LexOptions
	for _, backslash := range []bool{true, false} {
		for _, nested := range []bool{false, true} {
			out = append(out, sqltext.LexOptions{
				BackslashEscapes:      backslash,
				NestedComments:        nested,
				StrictDash:            true,
				UnclosedCommentIsCode: true,
			})
		}
	}
	return out
}()
