package sqltext

import (
	"strings"
)

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
