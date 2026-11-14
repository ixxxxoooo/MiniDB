package sqltext

import (
	"strings"
)

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
