package sqltext

import (
	"strings"
	"unicode"
)

type tokKind int

const (
	tkSpace tokKind = iota
	tkWord
	tkString // '…' "…" `…`
	tkComment
	tkSymbol
	tkNumber
	tkParam // :name {name} {{name}} ${name} 这类参数占位符，整体不可拆开
)

type token struct {
	kind tokKind
	text string
	// ownLine 仅对注释有意义：注释前面（同一行内）没有任何代码，即源码里它独占一行
	ownLine bool
}

func isWordStart(c rune) bool { return unicode.IsLetter(c) || c == '_' || c == '@' || c == '$' }
func isWordPart(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '$' || c == '@'
}

// tokenize 把 SQL 切成词法单元；空白也保留为单元，供格式化器判断注释是否独占一行。
func tokenize(sql string) []token {
	var out []token
	rs := []rune(sql)
	atLineStart := true // 当前行到目前为止只有空白
	for i := 0; i < len(rs); {
		c := rs[i]
		start := i
		switch {
		case unicode.IsSpace(c):
			for i < len(rs) && unicode.IsSpace(rs[i]) {
				if rs[i] == '\n' {
					atLineStart = true
				}
				i++
			}
			out = append(out, token{kind: tkSpace, text: string(rs[start:i])})
			continue
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-', c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			out = append(out, token{kind: tkComment, text: string(rs[start:i]), ownLine: atLineStart})
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i += 2
			for i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/') {
				i++
			}
			i = min(i+2, len(rs))
			out = append(out, token{kind: tkComment, text: string(rs[start:i]), ownLine: atLineStart})
		case c == '\'' || c == '"' || c == '`':
			i++
			for i < len(rs) {
				if c != '`' && rs[i] == '\\' && i+1 < len(rs) {
					i += 2
					continue
				}
				if rs[i] == c {
					if i+1 < len(rs) && rs[i+1] == c {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			out = append(out, token{kind: tkString, text: string(rs[start:min(i, len(rs))])})
		case unicode.IsDigit(c) || (c == '.' && i+1 < len(rs) && unicode.IsDigit(rs[i+1]) && !prevIsWordLike(out)):
			// 数字：整数、小数、科学计数法、0x1F、1_000，整体算一个词
			for i < len(rs) && (isWordPart(rs[i]) || rs[i] == '.') {
				if (rs[i] == 'e' || rs[i] == 'E') && i+1 < len(rs) && (rs[i+1] == '+' || rs[i+1] == '-') && i > start {
					i++ // 1e+5 里的符号属于数字
				}
				i++
			}
			out = append(out, token{kind: tkNumber, text: string(rs[start:i])})
		case c == '{' || (c == '$' && i+1 < len(rs) && rs[i+1] == '{'):
			// {name} {{name}} ${name}：到配对的右花括号为止，不跨行
			j := i
			if rs[j] == '$' {
				j++
			}
			depth := 0
			end := -1
			for k := j; k < len(rs) && rs[k] != '\n'; k++ {
				if rs[k] == '{' {
					depth++
				} else if rs[k] == '}' {
					depth--
					if depth == 0 {
						end = k + 1
						break
					}
				}
			}
			if end < 0 {
				i++
				out = append(out, token{kind: tkSymbol, text: string(rs[start:i])})
			} else {
				i = end
				out = append(out, token{kind: tkParam, text: string(rs[start:i])})
			}
		case c == ':' && i+1 < len(rs) && isWordStart(rs[i+1]) && rs[i+1] != '@' && rs[i+1] != '$' && !(i > 0 && rs[i-1] == ':'):
			// :name 参数（不是 :: 类型转换）
			i++
			for i < len(rs) && isWordPart(rs[i]) {
				i++
			}
			out = append(out, token{kind: tkParam, text: string(rs[start:i])})
		case isWordStart(c):
			for i < len(rs) && isWordPart(rs[i]) {
				i++
			}
			out = append(out, token{kind: tkWord, text: string(rs[start:i])})
		default:
			i++
			if i+1 < len(rs) {
				switch three := string(rs[start : i+2]); three {
				case "->>", "<=>", "#>>", "!~*":
					i += 2
					out = append(out, token{kind: tkSymbol, text: three})
					atLineStart = false
					continue
				}
			}
			if i < len(rs) {
				switch two := string(rs[start : i+1]); two {
				case "<=", ">=", "<>", "!=", "||", "::", "->", ":=", "<<", ">>", "&&", "@>", "<@", "!~", "~*", "#>":
					i++
				}
			}
			out = append(out, token{kind: tkSymbol, text: string(rs[start:i])})
		}
		atLineStart = false
	}
	return out
}

// prevIsWordLike 判断前一个有效单元是不是词/右括号：此时 ".5" 里的点是成员访问而不是小数点。
func prevIsWordLike(toks []token) bool {
	if len(toks) == 0 || toks[len(toks)-1].kind == tkSpace { // 点前有空白：当作小数
		return false
	}
	last := toks[len(toks)-1]
	return last.kind == tkWord || last.kind == tkString || last.text == ")"
}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Fields(s) {
		m[k] = true
	}
	return m
}

// keywords 在任何位置都会被转成大写。
var keywords = words(`SELECT FROM WHERE GROUP BY ORDER HAVING LIMIT OFFSET JOIN INNER LEFT RIGHT FULL OUTER CROSS NATURAL ON USING
	AS AND OR NOT IN IS NULL LIKE BETWEEN EXISTS CASE WHEN THEN ELSE END DISTINCT UNION ALL INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE
	ALTER DROP INDEX PRIMARY KEY FOREIGN REFERENCES DEFAULT ASC DESC WITH RECURSIVE OVER PARTITION IF TRUE FALSE EXPLAIN ANALYZE SHOW DESCRIBE
	USE BEGIN COMMIT ROLLBACK TRUNCATE VIEW UNIQUE CONSTRAINT CHECK COLLATE INTERVAL CAST ILIKE RETURNING EXCEPT INTERSECT
	ADD COLUMN MODIFY CHANGE RENAME TO TEMPORARY TEMP REPLACE CURRENT_TIMESTAMP CURRENT_DATE CURRENT_TIME CASCADE RESTRICT
	DUPLICATE CONFLICT DO NOTHING WINDOW FETCH NEXT ROWS ONLY FIRST LAST NULLS ANY SOME ESCAPE SIMILAR`)

// typeWords 是列定义里"列名之后"的类型词，只在 CREATE/ALTER 的列定义里转成大写，
// 以免把名叫 date、text 的普通列改了大小写。
var typeWords = words(`INT INTEGER TINYINT SMALLINT MEDIUMINT BIGINT DECIMAL NUMERIC FLOAT DOUBLE REAL BIT BOOL BOOLEAN CHAR VARCHAR NCHAR
	NVARCHAR TEXT TINYTEXT MEDIUMTEXT LONGTEXT BLOB TINYBLOB MEDIUMBLOB LONGBLOB BINARY VARBINARY DATE TIME DATETIME TIMESTAMP YEAR JSON
	JSONB UUID SERIAL BIGSERIAL SMALLSERIAL ENUM BYTEA MONEY CITEXT GEOMETRY POINT CHARACTER VARYING PRECISION INT2 INT4 INT8 FLOAT4 FLOAT8
	TIMESTAMPTZ TIMETZ`)

// ddlWords 是列定义与表选项里的修饰词（列名与类型之后），同样只在 DDL 里转大写。
var ddlWords = words(`UNSIGNED ZEROFILL AUTO_INCREMENT AUTOINCREMENT COMMENT CHARSET CHARACTER COLLATE GENERATED ALWAYS STORED VIRTUAL IDENTITY
	WITHOUT TIME ZONE ACTION NO ENGINE ROW_FORMAT TABLESPACE INHERITS USING BTREE HASH VISIBLE INVISIBLE FIRST AFTER`)

// constraintStarts 在列定义列表里出现在第一个位置时，说明这一项是约束而不是列。
var constraintStarts = words(`PRIMARY KEY UNIQUE INDEX CONSTRAINT FOREIGN CHECK FULLTEXT SPATIAL EXCLUDE LIKE PERIOD`)

// alterActions 是 ALTER TABLE 表名之后的动作。
var alterActions = words(`ADD DROP MODIFY CHANGE RENAME ALTER ENABLE DISABLE OWNER CONVERT ENGINE AUTO_INCREMENT COMMENT`)

// joinWords 开启一个 JOIN 子句。
var joinWords = words(`JOIN INNER LEFT RIGHT FULL CROSS NATURAL`)

// clauseStarts 在语句层独占一行。
var clauseStarts = words(`FROM WHERE HAVING LIMIT OFFSET UNION EXCEPT INTERSECT RETURNING WINDOW QUALIFY`)

// spaceBeforeParen 后面跟 "(" 时要留空格的关键字；其他词（函数名、类型名）则紧贴括号。
var spaceBeforeParen = words(`IN NOT AND OR ON USING VALUES WHERE AS EXISTS SELECT FROM JOIN UNION ALL ANY SOME OVER WHEN THEN ELSE CASE HAVING BY SET
	DEFAULT KEY INDEX UNIQUE PRIMARY FOREIGN CHECK REFERENCES CONSTRAINT INTO TABLE LIKE ILIKE BETWEEN IS DO CONFLICT PARTITION EXCEPT INTERSECT RETURNING
	WITH RECURSIVE INSERT UPDATE DELETE REPLACE LATERAL ADD`)

type frameKind int

const (
	fRoot   frameKind = iota // 语句层
	fSub                     // 子查询 / CTE 主体：括号内另起行、缩进一级
	fInline                  // 函数参数、IN 列表、条件分组等：保持在一行
	fCreate                  // CREATE TABLE 的列定义列表：一项一行
)

// frame 是一层括号的上下文。
type frame struct {
	kind frameKind
	// indent 是本层语句内各子句所在的缩进级别；openIndent 是左括号所在行的缩进级别，右括号回到这一级。
	indent, openIndent int

	selList  bool // 正处在 SELECT 列表
	multiSel bool // SELECT 列表多项：逐项换行
	setList  bool // 正处在 UPDATE 的 SET 列表
	multiSet bool
	multiVal bool // INSERT … VALUES 多组：逐组换行
	cond     bool // 正处在 WHERE / HAVING / ON 条件里，AND / OR 才换行
	between  bool // BETWEEN 的 AND 不换行
	inWith   bool // 正在 WITH 的 CTE 列表里，逗号换行

	// CREATE TABLE 列定义列表里当前这一项的状态
	item       int
	constraint bool
}

type stmtKind int

const (
	sOther stmtKind = iota
	sSelect
	sInsert
	sUpdate
	sDelete
	sCreateTable
	sAlter
)

type caseFrame struct {
	indent int
	multi  bool
}

// formatter 逐词输出格式化结果。
type formatter struct {
	toks []token // 已去掉空白
	idx  int

	unit    string // 一级缩进
	lower   bool   // 关键字小写
	lines   []string
	cur     strings.Builder
	curInd  int  // 当前行的缩进级别
	empty   bool // 当前行还没有内容（只有缩进）
	glue    bool // 下一个词紧贴上一个词（. :: 一元负号之后）
	frames  []*frame
	cases   []caseFrame
	kind    stmtKind
	first   bool // 本语句还没有输出过词
	prev    string
	tailOpt bool // CREATE TABLE 的列表已结束，正在输出表选项
	alter   bool // ALTER 已经换过行
}

// defaultIndent 是缺省的缩进：两个空格。
const defaultIndent = "  "

// FormatOptions 是格式化的可选项，零值就是缺省写法（两个空格缩进、关键字大写）。
type FormatOptions struct {
	Indent        string // 一级缩进用的字符，如 "\t"、"    "；空则两个空格
	LowerKeywords bool   // 关键字写成小写
}

// operandMark 记在 prev 里，表示上一个单元是字符串、数字、参数这类操作数。
const operandMark = "#"

// countItem 在 CREATE TABLE 的列定义里给当前项记一个单元（带引号的列名也算一个）。
func (f *formatter) countItem() {
	if fr := f.top(); fr.kind == fCreate {
		fr.item++
	}
}

// Format 把语句排成通行的多行写法：
//   - 子句各占一行，关键字大写，标识符、字符串、注释原样保留；
//   - SELECT / SET 列表有多项时逐项换行，WHERE 里的 AND / OR 缩进一级；
//   - 子查询、CTE 在括号内另起行缩进；CASE 过长时 WHEN / ELSE 分行；
//   - INSERT … VALUES 多组时逐组换行；
//   - CREATE TABLE 一列一行，类型与修饰词大写，表选项紧跟右括号；
//   - 多条语句之间空一行。
//
// 格式化只改变空白与关键字大小写，不改变语句含义。
func Format(sql string) string { return FormatWith(sql, FormatOptions{}) }

// FormatWith 同 Format，但缩进字符和关键字大小写按 o 来。
func FormatWith(sql string, o FormatOptions) string {
	var toks []token
	for _, t := range tokenize(sql) {
		if t.kind != tkSpace {
			toks = append(toks, t)
		}
	}
	f := &formatter{toks: toks, empty: true, first: true, unit: o.Indent, lower: o.LowerKeywords}
	if f.unit == "" {
		f.unit = defaultIndent
	}
	f.frames = []*frame{{kind: fRoot}}
	f.run()
	return strings.TrimSpace(f.result())
}

func (f *formatter) top() *frame { return f.frames[len(f.frames)-1] }

// stmtLevel 报告当前是否在语句层（根或子查询），子句换行只发生在这里。
func (f *formatter) stmtLevel() bool {
	k := f.top().kind
	return k == fRoot || k == fSub
}

func (f *formatter) peek(n int) string {
	if f.idx+n < len(f.toks) {
		return f.toks[f.idx+n].text
	}
	return ""
}

func (f *formatter) peekUp(n int) string { return strings.ToUpper(f.peek(n)) }

// newline 结束当前行并以 level 级缩进开始新行；当前行为空时只调整缩进，不产生空行。
func (f *formatter) newline(level int) {
	if !f.empty {
		f.lines = append(f.lines, strings.TrimRight(f.cur.String(), " "))
		f.cur.Reset()
	}
	f.cur.Reset()
	f.cur.WriteString(strings.Repeat(f.unit, max(level, 0)))
	f.curInd = max(level, 0)
	f.empty = true
	f.glue = false
}

// blankLine 在语句之间留一个空行。
func (f *formatter) blankLine() {
	f.newline(0)
	if n := len(f.lines); n > 0 && f.lines[n-1] != "" {
		f.lines = append(f.lines, "")
	}
}

func (f *formatter) result() string {
	if !f.empty {
		f.lines = append(f.lines, strings.TrimRight(f.cur.String(), " "))
	}
	return strings.Join(f.lines, "\n")
}

// emit 输出一个词，必要时在前面补空格。
func (f *formatter) emit(s string) {
	if !f.empty && !f.glue {
		last := f.cur.String()
		if c := last[len(last)-1]; c != ' ' && c != '(' && c != '[' {
			switch s {
			case ")", "]", ",", ";", ".", "::":
			default:
				f.cur.WriteByte(' ')
			}
		}
	}
	f.cur.WriteString(s)
	f.empty = false
	f.glue = false
}

// emitGlued 输出一个紧贴前一个词、也紧贴后一个词的符号（. ::）。
func (f *formatter) emitGlued(s string) {
	f.cur.WriteString(s)
	f.empty = false
	f.glue = true
}

func (f *formatter) push(k frameKind, indent int) *frame {
	fr := &frame{kind: k, indent: indent, openIndent: f.curInd}
	f.frames = append(f.frames, fr)
	return fr
}

func (f *formatter) pop() *frame {
	if len(f.frames) == 1 {
		return nil
	}
	fr := f.top()
	f.frames = f.frames[:len(f.frames)-1]
	return fr
}

func (f *formatter) resetStatement() {
	f.frames = f.frames[:1]
	*f.frames[0] = frame{kind: fRoot}
	f.cases = nil
	f.kind = sOther
	f.first, f.prev = true, ""
	f.tailOpt, f.alter = false, false
}

func (f *formatter) run() {
	for f.idx = 0; f.idx < len(f.toks); f.idx++ {
		tk := f.toks[f.idx]
		switch tk.kind {
		case tkComment:
			f.comment(tk)
		case tkWord:
			f.word(tk)
		case tkSymbol:
			f.symbol(tk)
		default:
			f.countItem()
			f.emit(tk.text)
			f.prev = operandMark
			f.first = false
		}
	}
}

func (f *formatter) comment(tk token) {
	fr := f.top()
	level := fr.indent
	if !f.stmtLevel() {
		level = f.curInd
	}
	// 行内跟在代码后面的注释留在原行：逗号后立刻换了行时，把它贴回上一行末尾
	if !tk.ownLine && f.empty && len(f.lines) > 0 && f.lines[len(f.lines)-1] != "" {
		ind := f.curInd
		last := f.lines[len(f.lines)-1]
		f.lines = f.lines[:len(f.lines)-1]
		f.cur.Reset()
		f.cur.WriteString(last + " " + tk.text)
		f.empty = false
		f.newline(ind)
		return
	}
	// 独占一行的注释保持独占一行
	if tk.ownLine && !f.empty {
		f.newline(level)
	}
	f.emit(tk.text)
	if strings.HasPrefix(tk.text, "--") || strings.HasPrefix(tk.text, "#") {
		f.newline(level)
	} else if tk.ownLine {
		f.newline(level)
	}
}

// detectKind 根据语句开头的几个词判断语句类型。
func (f *formatter) detectKind(up string) {
	switch up {
	case "SELECT", "WITH":
		f.kind = sSelect
	case "INSERT", "REPLACE":
		f.kind = sInsert
	case "UPDATE":
		f.kind = sUpdate
	case "DELETE":
		f.kind = sDelete
	case "ALTER":
		f.kind = sAlter
	case "CREATE":
		for j := 1; j <= 6; j++ {
			switch f.peekUp(j) {
			case "TABLE":
				f.kind = sCreateTable
				return
			case "VIEW", "INDEX", "TRIGGER", "FUNCTION", "PROCEDURE", "DATABASE", "SCHEMA":
				return
			}
		}
	}
}

func (f *formatter) word(tk token) {
	up := strings.ToUpper(tk.text)
	fr := f.top()
	if f.first {
		f.detectKind(up)
		f.first = false
	}
	word := tk.text
	upper := false

	switch {
	case fr.kind == fCreate:
		upper = f.createItemWord(fr, up)
	case f.tailOpt && fr.kind == fRoot:
		// 表选项：ENGINE=InnoDB 里等号前的选项名大写，等号后的值原样
		upper = keywords[up] || ddlWords[up] || f.peek(1) == "="
	case f.kind == sAlter:
		upper = keywords[up] || f.inDDLColumn(up)
	default:
		upper = keywords[up]
	}
	if upper {
		word = up
		if f.lower {
			word = strings.ToLower(up)
		}
	}

	if f.stmtLevel() {
		f.clauseBreak(fr, up)
	}
	switch up {
	case "CASE":
		f.beginCase()
	case "WHEN", "ELSE":
		if n := len(f.cases); n > 0 && f.cases[n-1].multi {
			f.newline(f.cases[n-1].indent + 1)
		}
	case "END":
		if n := len(f.cases); n > 0 {
			c := f.cases[n-1]
			f.cases = f.cases[:n-1]
			if c.multi {
				f.newline(c.indent)
			}
		}
	case "BETWEEN":
		fr.between = true
	}
	f.emit(word)
	f.afterWord(fr, up)
	f.prev = up
}

// inDDLColumn 在 ALTER 的 ADD/MODIFY COLUMN 之类的位置，判断 up 是不是类型或修饰词：
// 类型词必须紧跟在列名之后（或带长度括号），否则可能只是个叫 date、text 的普通标识符。
func (f *formatter) inDDLColumn(up string) bool {
	if ddlWords[up] {
		return true
	}
	return typeWords[up] && (f.peek(1) == "(" || f.prevIsColumnName())
}

// prevIsColumnName 判断上一个单元是不是标识符（普通词或带引号的名字）。
func (f *formatter) prevIsColumnName() bool {
	if f.idx == 0 {
		return false
	}
	t := f.toks[f.idx-1]
	return t.kind == tkString || (t.kind == tkWord && !keywords[strings.ToUpper(t.text)])
}

// createItemWord 处理 CREATE TABLE 列定义列表里的一个词，返回是否要转大写。
// 每一项的第一个词是列名（原样），第二个是类型，其后是修饰词；以 PRIMARY / KEY / UNIQUE 等开头的是约束项。
func (f *formatter) createItemWord(fr *frame, up string) bool {
	defer func() { fr.item++ }()
	switch {
	case fr.item == 0:
		fr.constraint = constraintStarts[up]
		return fr.constraint
	case fr.constraint:
		return keywords[up] || ddlWords[up]
	case fr.item == 1:
		return typeWords[up] || keywords[up]
	}
	return keywords[up] || ddlWords[up] || typeWords[up]
}

// clauseBreak 处理语句层的子句换行与列表布局。
func (f *formatter) clauseBreak(fr *frame, up string) {
	switch {
	case up == "SELECT":
		if !f.empty {
			f.newline(fr.indent)
		}
		fr.inWith = false
		fr.selList, fr.cond = true, false
		fr.multiSel = f.selectHasManyItems()
		// SELECT DISTINCT / ALL 要和 SELECT 同行，逐项换行排在它们之后
	case up == "FROM" && (f.prev == "DELETE" || f.prev == "DISTINCT"):
		// DELETE FROM、IS DISTINCT FROM：不换行
	case up == "GROUP" || up == "ORDER":
		if f.peekUp(1) == "BY" {
			f.newline(fr.indent)
			fr.selList, fr.cond, fr.setList = false, false, false
		}
	case up == "BY" && (f.prev == "GROUP" || f.prev == "ORDER"):
		fr.selList = false
	case up == "VALUES" && f.kind == sInsert:
		f.newline(fr.indent)
		fr.multiVal = f.valuesHasManyTuples()
	case up == "SET" && f.kind == sUpdate:
		f.newline(fr.indent)
		fr.setList, fr.selList, fr.cond = true, false, false
		fr.multiSet = f.setHasManyItems()
	case up == "ON" && (f.peekUp(1) == "DUPLICATE" || f.peekUp(1) == "CONFLICT"):
		f.newline(fr.indent)
		fr.cond = false
	case clauseStarts[up]:
		// UNION ALL 的 ALL、FETCH 之类不是子句；LIMIT / OFFSET 等换行
		f.newline(fr.indent)
		fr.selList, fr.setList = false, false
		fr.cond = up == "WHERE" || up == "HAVING"
	case joinWords[up] && !joinWords[f.prev] && f.prev != "OUTER" && (up == "JOIN" || f.peek(1) != "("):
		f.newline(fr.indent)
		fr.selList, fr.cond = false, false
	case up == "ON" && fr.kind != fInline:
		fr.cond = true
	case up == "WITH" && f.empty && f.kind == sSelect:
		fr.inWith = true
	case f.kind == sAlter && !f.alter && alterActions[up] && f.prevIsColumnName():
		// 表名之后的第一个动作另起一行，之后每个逗号再换行
		f.alter = true
		f.newline(fr.indent + 1)
	case up == "AND" && fr.between:
		fr.between = false
	case (up == "AND" || up == "OR") && fr.cond:
		f.newline(fr.indent + 1)
	}
}

// afterWord 在词输出之后处理需要"紧跟"的版面：列表逐项时，关键字之后换行。
func (f *formatter) afterWord(fr *frame, up string) {
	if !f.stmtLevel() {
		return
	}
	switch up {
	case "SELECT":
		if fr.multiSel {
			if next := f.peekUp(1); next == "DISTINCT" || next == "ALL" {
				return // 等 DISTINCT / ALL 输出后再换行
			}
			f.newline(fr.indent + 1)
		}
	case "DISTINCT", "ALL":
		if f.prev == "SELECT" && fr.multiSel {
			f.newline(fr.indent + 1)
		}
	case "SET":
		if f.kind == sUpdate && fr.multiSet && fr.setList {
			f.newline(fr.indent + 1)
		}
	case "VALUES":
		if f.kind == sInsert && fr.multiVal {
			f.newline(fr.indent + 1)
		}
	}
}

func (f *formatter) beginCase() {
	// 短的 CASE 保持一行，长的才把 WHEN / ELSE / END 分行
	length, depth := 0, 0
	multi := false
	for j := f.idx; j < len(f.toks); j++ {
		t := f.toks[j]
		length += len(t.text) + 1
		up := strings.ToUpper(t.text)
		if up == "CASE" {
			depth++
		}
		if up == "END" {
			depth--
			if depth == 0 {
				break
			}
		}
		if up == "SELECT" || t.kind == tkComment {
			multi = true
		}
	}
	f.cases = append(f.cases, caseFrame{indent: f.curInd, multi: multi || length > 72})
}

// 向前扫描：SELECT 列表（到 FROM 或语句结束为止）在括号外有没有逗号。
func (f *formatter) selectHasManyItems() bool {
	depth := 0
	for j := f.idx + 1; j < len(f.toks); j++ {
		t := f.toks[j]
		switch {
		case t.text == "(":
			depth++
		case t.text == ")":
			if depth == 0 {
				return false
			}
			depth--
		case depth == 0 && t.text == ",":
			return true
		case depth == 0 && t.text == ";":
			return false
		case depth == 0 && t.kind == tkWord:
			switch strings.ToUpper(t.text) {
			case "FROM", "WHERE", "UNION", "INTO", "EXCEPT", "INTERSECT", "LIMIT", "GROUP", "ORDER":
				return false
			}
		}
	}
	return false
}

func (f *formatter) setHasManyItems() bool {
	depth := 0
	for j := f.idx + 1; j < len(f.toks); j++ {
		t := f.toks[j]
		switch {
		case t.text == "(":
			depth++
		case t.text == ")":
			if depth == 0 {
				return false
			}
			depth--
		case depth == 0 && t.text == ",":
			return true
		case depth == 0 && t.text == ";":
			return false
		case depth == 0 && t.kind == tkWord:
			switch strings.ToUpper(t.text) {
			case "WHERE", "RETURNING", "FROM", "ORDER", "LIMIT":
				return false
			}
		}
	}
	return false
}

func (f *formatter) valuesHasManyTuples() bool {
	depth := 0
	for j := f.idx + 1; j < len(f.toks); j++ {
		t := f.toks[j]
		switch {
		case t.text == "(":
			depth++
		case t.text == ")":
			if depth == 0 {
				return false
			}
			depth--
		case depth == 0 && t.text == ",":
			return true
		case depth == 0 && (t.text == ";" || t.kind == tkWord):
			return false
		}
	}
	return false
}

func (f *formatter) symbol(tk token) {
	fr := f.top()
	switch tk.text {
	case "(":
		f.openParen(fr)
	case ")":
		f.closeParen()
	case ",":
		f.emit(",")
		f.afterComma(fr)
	case ";":
		f.emit(";")
		f.resetStatement()
		f.blankLine()
		return
	case ".":
		// 成员访问：前后都不留空格（t.col、db.t、t.*）
		f.emitGlued(".")
	case "::":
		f.emitGlued("::")
	case "=":
		if f.tailOpt && f.stmtLevel() && f.kind == sCreateTable {
			f.emitGlued("=") // ENGINE=InnoDB，与 SHOW CREATE TABLE 的写法一致
		} else {
			f.emit("=")
		}
	case "-", "+":
		unary := f.isUnary()
		f.emit(tk.text)
		f.glue = unary
	case "[":
		// 数组下标或类型后缀（TEXT[]、a[1]）：紧贴前一个词
		if f.lastWord() != "" || (f.idx > 0 && f.toks[f.idx-1].text == ")") {
			f.emitGlued("[")
		} else {
			f.emit("[")
			f.glue = true
		}
	case "]":
		f.emit("]")
	default:
		f.emit(tk.text)
	}
	f.prev = tk.text
	f.first = false
}

// isUnary 判断刚输出的 + / - 是不是一元符号：它前面是运算符、逗号、左括号或某些关键字。
func (f *formatter) isUnary() bool {
	switch f.prev {
	case "(", ",", "=", "<", ">", "<=", ">=", "<>", "!=", "+", "-", "*", "/", "%", "||", "[":
		return true
	case "AND", "OR", "NOT", "WHEN", "THEN", "ELSE", "SELECT", "WHERE", "BY", "IN", "BETWEEN", "LIMIT", "OFFSET", "HAVING", "ON", "SET", "VALUES", "DEFAULT", "RETURN", "AS":
		return true
	}
	return false
}

func (f *formatter) openParen(fr *frame) {
	next := f.peekUp(1)
	switch {
	case next == "SELECT" || next == "WITH":
		// 子查询：括号内另起行，右括号回到这一行的缩进
		f.emitParen(fr)
		sub := f.push(fSub, f.curInd+1)
		sub.openIndent = f.curInd
		f.newline(sub.indent)
	case f.kind == sCreateTable && !f.tailOpt && len(f.frames) == 1 && f.prev != "AS" && f.prev != "LIKE" && next != "LIKE":
		// 列定义列表：一项一行
		f.emitParen(fr)
		cr := f.push(fCreate, f.curInd+1)
		cr.openIndent = f.curInd
		f.newline(cr.indent)
	default:
		f.emitParen(fr)
		f.push(fInline, f.curInd)
	}
}

// emitParen 输出左括号：函数调用、类型名紧贴前一个词，其余留一个空格。
func (f *formatter) emitParen(fr *frame) {
	if f.empty {
		f.cur.WriteString("(")
		f.empty = false
		return
	}
	glue := false
	if prev := f.lastWord(); prev != "" && !spaceBeforeParen[prev] {
		switch {
		case f.kind == sCreateTable || f.kind == sInsert || f.kind == sAlter:
			// DDL / INSERT：表名之后的列表要空一格；括号里面的词（类型名、函数）才紧贴，
			// 约束项里的索引名（KEY idx (a)）也要空一格
			glue = (typeWords[prev] && f.kind != sInsert) || (len(f.frames) > 1 && !(fr.kind == fCreate && fr.constraint))
		default:
			glue = true
		}
	}
	if glue {
		f.cur.WriteString("(")
		f.empty = false
		f.glue = false
		return
	}
	f.emit("(")
}

// lastWord 返回上一个输出的词（大写）；上一个是符号则返回空。
func (f *formatter) lastWord() string {
	if f.idx == 0 {
		return ""
	}
	t := f.toks[f.idx-1]
	if t.kind != tkWord {
		return ""
	}
	return strings.ToUpper(t.text)
}

func (f *formatter) closeParen() {
	fr := f.pop()
	if fr == nil {
		f.emit(")")
		return
	}
	switch fr.kind {
	case fSub:
		f.newline(fr.openIndent)
		f.emit(")")
	case fCreate:
		f.newline(fr.openIndent)
		f.emit(")")
		f.tailOpt = true
	default:
		f.emit(")")
	}
}

func (f *formatter) afterComma(fr *frame) {
	switch fr.kind {
	case fCreate:
		fr.item, fr.constraint = 0, false
		f.newline(fr.indent)
	case fRoot, fSub:
		switch {
		case fr.selList && fr.multiSel:
			f.newline(fr.indent + 1)
		case fr.setList && fr.multiSet:
			f.newline(fr.indent + 1)
		case f.kind == sInsert && fr.multiVal && fr.kind == fRoot && f.prevParenClosed():
			f.newline(fr.indent + 1)
		case fr.inWith:
			f.newline(fr.indent)
		case f.kind == sAlter && f.alter:
			f.newline(fr.indent + 1)
		}
	}
}

// prevParenClosed 判断逗号前是不是刚闭合的一组括号（VALUES 的元组之间的逗号）。
func (f *formatter) prevParenClosed() bool {
	return f.idx > 0 && f.toks[f.idx-1].text == ")"
}
