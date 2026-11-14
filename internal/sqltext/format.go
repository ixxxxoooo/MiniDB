package sqltext

import (
	"strings"
)

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
