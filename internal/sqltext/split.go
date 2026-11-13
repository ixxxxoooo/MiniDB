// Package sqltext holds editor-side SQL text utilities: splitting a script
// into statements, finding the statement at a caret, and formatting.
package sqltext

import (
	"regexp"
	"strings"
)

// Statement is one statement of a script, with its byte range in the source.
type Statement struct {
	Text       string // trimmed
	Start, End int    // byte offsets of Text in the source
}

var dollarTag = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*\$|^\$\$`)

// Split cuts a script at semicolons that are not inside strings, comments,
// quoted identifiers or dollar-quoted bodies. A fragment holding only
// comments is dropped; comments inside a statement stay in it.
func Split(sql string) []Statement {
	var out []Statement
	n := len(sql)
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
	i := 0
	for i < n {
		c := sql[i]
		switch {
		case c == '-' && i+1 < n && sql[i+1] == '-', c == '#':
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				i = n
			} else {
				i += j
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			j := strings.Index(sql[i+2:], "*/")
			if j < 0 {
				i = n
			} else {
				i += 2 + j + 2
			}
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < n {
				if c != '`' && sql[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if sql[j] == c {
					if j+1 < n && sql[j+1] == c {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			hasCode = true
			i = min(j, n)
		case c == '$':
			end := min(i+64, n)
			if tag := dollarTag.FindString(sql[i:end]); tag != "" {
				j := strings.Index(sql[i+len(tag):], tag)
				if j < 0 {
					i = n
				} else {
					i += len(tag) + j + len(tag)
				}
				hasCode = true
				continue
			}
			hasCode = true
			i++
		case c == ';':
			flush(i)
			i++
			start = i
		default:
			if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
				hasCode = true
			}
			i++
		}
	}
	flush(n)
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
