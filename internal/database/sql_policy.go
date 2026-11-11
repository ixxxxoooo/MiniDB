package database

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractAutoExecuteMetaBlock 从回复中提取 AI 旧版结构化元数据块（minidb-meta），并返回剥离后的正文。
// 兼容旧链路残留；新 Agent 协议不再要求模型输出该块。
func ExtractAutoExecuteMetaBlock(text string) (meta struct {
	AutoExecute struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode,omitempty"`
		Reason  string `json:"reason,omitempty"`
	} `json:"autoExecute"`
}, cleaned string, ok bool) {
	re := regexp.MustCompile("(?is)```minidb-meta\\s*(\\{[\\s\\S]*?\\})\\s*```")
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return meta, strings.TrimSpace(text), false
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &meta); err != nil {
		return struct {
			AutoExecute struct {
				Enabled bool   `json:"enabled"`
				Mode    string `json:"mode,omitempty"`
				Reason  string `json:"reason,omitempty"`
			} `json:"autoExecute"`
		}{}, strings.TrimSpace(text), false
	}
	cleaned = re.ReplaceAllString(text, "")
	cleaned = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleaned, "\n\n")
	return meta, strings.TrimSpace(cleaned), true
}

// AutoExecutableCheckResult 自动执行 SQL 的安全校验结果（reasonCode 供前端 i18n）
type AutoExecutableCheckResult struct {
	Allowed    bool   `json:"allowed"`
	ReasonCode string `json:"reasonCode,omitempty"` // empty_sql, multi_sql, risky_sql, unknown_sql
	Verb       string `json:"verb,omitempty"`
}

// CheckAutoExecutableSelectSQL 判断是否允许 AI 自动执行（仅允许只读类语句）
func CheckAutoExecutableSelectSQL(sql string) AutoExecutableCheckResult {
	verb := SQLLeadingVerb(sql)
	if verb == "" {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "empty_sql"}
	}
	allow := map[string]struct{}{
		"select": {}, "show": {}, "desc": {}, "describe": {}, "explain": {}, "with": {},
	}
	if _, ok := allow[verb]; ok {
		return AutoExecutableCheckResult{Allowed: true}
	}
	risky := map[string]struct{}{
		"insert": {}, "update": {}, "delete": {}, "replace": {}, "create": {}, "alter": {}, "drop": {}, "truncate": {}, "rename": {},
		"grant": {}, "revoke": {}, "call": {}, "set": {}, "use": {}, "begin": {}, "start": {}, "commit": {}, "rollback": {}, "lock": {}, "unlock": {},
	}
	if _, ok := risky[verb]; ok {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "risky_sql", Verb: strings.ToUpper(verb)}
	}
	return AutoExecutableCheckResult{Allowed: false, ReasonCode: "unknown_sql", Verb: strings.ToUpper(verb)}
}

// CheckAutoExecutableReadOnlySingleSQL 判断是否允许自动执行单条只读 SQL。
// 允许一个结尾分号，但拒绝用分号串联的多语句。等价于 CheckReadOnlySQL。
func CheckAutoExecutableReadOnlySingleSQL(sql string) AutoExecutableCheckResult {
	return CheckReadOnlySQL(sql)
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

var readOnlyVerbs = map[string]struct{}{
	"select": {}, "show": {}, "desc": {}, "describe": {}, "explain": {}, "with": {},
}

var riskyVerbs = map[string]struct{}{
	"insert": {}, "update": {}, "delete": {}, "replace": {}, "create": {}, "alter": {}, "drop": {}, "truncate": {}, "rename": {},
	"grant": {}, "revoke": {}, "call": {}, "set": {}, "use": {}, "begin": {}, "start": {}, "commit": {}, "rollback": {}, "lock": {}, "unlock": {},
	"merge": {}, "copy": {}, "do": {}, "load": {}, "analyze": {}, "optimize": {}, "vacuum": {}, "kill": {}, "handler": {},
}

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
	// 两种反斜杠解释都必须通过
	var last AutoExecutableCheckResult
	for _, backslashEscapes := range []bool{true, false} {
		skeleton, executableComment := sqlSkeleton(sql, backslashEscapes)
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
	if _, ok := readOnlyVerbs[verb]; !ok {
		if _, risky := riskyVerbs[verb]; risky {
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
// backslashEscapes 决定引号内的反斜杠是否转义下一个字符。
// executableComment 表示出现了 MySQL 可执行注释 /*! ... */。
func sqlSkeleton(sql string, backslashEscapes bool) (skeleton string, executableComment bool) {
	var b strings.Builder
	n := len(sql)
	for i := 0; i < n; {
		c := sql[i]
		switch {
		case c == '-' && i+1 < n && sql[i+1] == '-' && (i+2 >= n || isSQLSpace(sql[i+2])):
			// 行注释：-- 后必须跟空白才在所有方言里都是注释
			for i < n && sql[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
		case c == '/' && i+1 < n && sql[i+1] == '*':
			if i+2 < n && sql[i+2] == '!' {
				executableComment = true
			}
			end := strings.Index(sql[i+2:], "*/")
			if end < 0 {
				// 未闭合的块注释：其后内容是否执行取决于方言，按代码保守处理
				b.WriteString(sql[i:])
				i = n
				break
			}
			i += 2 + end + 2
			b.WriteByte(' ')
		case c == '\'' || c == '"' || c == '`':
			quote := c
			i++
			for i < n {
				ch := sql[i]
				if backslashEscapes && quote != '`' && ch == '\\' && i+1 < n {
					i += 2
					continue
				}
				if ch == quote {
					if i+1 < n && sql[i+1] == quote { // 双写引号表示转义
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			b.WriteString(" '' ")
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), executableComment
}

func isSQLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
