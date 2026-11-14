// Package sqltext holds editor-side SQL text utilities: splitting a script
// into statements, finding the statement at a caret, and formatting.
package sqltext

import "strings"

// Statement is one statement of a script, with its byte range in the source.
type Statement struct {
	Text       string // trimmed
	Start, End int    // byte offsets of Text in the source
}

// Split cuts a script at semicolons that are not inside strings, comments,
// quoted identifiers or dollar-quoted bodies. A fragment holding only
// comments is dropped; comments inside a statement stay in it.
func Split(sql string) []Statement { return SplitWith(sql, EditorLexOptions) }

// SplitWith 与 Split 相同，但词法解释方式由调用方决定（例如 PostgreSQL 脚本可开启嵌套注释）。
func SplitWith(sql string, o LexOptions) []Statement {
	var out []Statement
	start := 0
	hasCode := false
	flush := func(end int) {
		seg := sql[start:end]
		trimmed := strings.TrimSpace(seg)
		if trimmed != "" && hasCode {
			lead := len(seg) - len(strings.TrimLeft(seg, " \t\r\n"))
			out = append(out, Statement{Text: trimmed, Start: start + lead, End: start + lead + len(trimmed)})
		}
		hasCode = false
	}
	for _, t := range Lex(sql, o) {
		switch t.Kind {
		case TokQuoted, TokDollar:
			hasCode = true
		case TokCode:
			for i := t.Start; i < t.End; i++ {
				switch c := sql[i]; {
				case c == ';':
					flush(i)
					start = i + 1
				case !isSpaceByte(c):
					hasCode = true
				}
			}
		}
	}
	flush(len(sql))
	return out
}

// Texts returns just the statements' text.
func Texts(sql string) []string {
	st := Split(sql)
	out := make([]string, len(st))
	for i, s := range st {
		out[i] = s.Text
	}
	return out
}

// At returns the statement the caret (byte offset) is in or, when it sits
// between statements, the one before it, else the first.
func At(sql string, caret int) (Statement, bool) {
	st := Split(sql)
	if len(st) == 0 {
		return Statement{}, false
	}
	pick := st[0]
	for _, s := range st {
		if s.Start <= caret {
			pick = s
		}
		if caret <= s.End {
			break
		}
	}
	return pick, true
}
