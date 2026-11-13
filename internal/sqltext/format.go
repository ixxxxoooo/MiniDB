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
)

type token struct {
	kind tokKind
	text string
}

func tokenize(sql string) []token {
	var out []token
	rs := []rune(sql)
	for i := 0; i < len(rs); {
		c := rs[i]
		start := i
		switch {
		case unicode.IsSpace(c):
			for i < len(rs) && unicode.IsSpace(rs[i]) {
				i++
			}
			out = append(out, token{tkSpace, string(rs[start:i])})
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-', c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			out = append(out, token{tkComment, string(rs[start:i])})
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i += 2
			for i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/') {
				i++
			}
			i = min(i+2, len(rs))
			out = append(out, token{tkComment, string(rs[start:i])})
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
			out = append(out, token{tkString, string(rs[start:min(i, len(rs))])})
		case unicode.IsDigit(c):
			for i < len(rs) && (unicode.IsDigit(rs[i]) || rs[i] == '.') {
				i++
			}
			out = append(out, token{tkNumber, string(rs[start:i])})
		case unicode.IsLetter(c) || c == '_' || c == '@' || c == '$':
			for i < len(rs) && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_' || rs[i] == '$' || rs[i] == '@') {
				i++
			}
			out = append(out, token{tkWord, string(rs[start:i])})
		default:
			i++
			// two-char operators
			if i < len(rs) {
				two := string(rs[start : i+1])
				switch two {
				case "<=", ">=", "<>", "!=", "||", "::", "->", ":=":
					i++
				}
			}
			out = append(out, token{tkSymbol, string(rs[start:i])})
		}
	}
	return out
}

var keywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`SELECT FROM WHERE GROUP BY ORDER HAVING LIMIT OFFSET JOIN INNER LEFT RIGHT FULL OUTER CROSS ON USING
	AS AND OR NOT IN IS NULL LIKE BETWEEN EXISTS CASE WHEN THEN ELSE END DISTINCT UNION ALL INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE
	ALTER DROP INDEX PRIMARY KEY FOREIGN REFERENCES DEFAULT ASC DESC WITH RECURSIVE OVER PARTITION IF TRUE FALSE EXPLAIN ANALYZE SHOW DESCRIBE
	USE BEGIN COMMIT ROLLBACK TRUNCATE VIEW UNIQUE CONSTRAINT CHECK COLLATE INTERVAL CAST ILIKE RETURNING EXCEPT INTERSECT`) {
		keywords[k] = true
	}
}

// clause keywords that start a new line at the base indent.
var clauseStarts = map[string]bool{
	"FROM": true, "WHERE": true, "HAVING": true, "LIMIT": true, "OFFSET": true,
	"UNION": true, "EXCEPT": true, "INTERSECT": true, "VALUES": true, "SET": true, "RETURNING": true,
}

var joinWords = map[string]bool{"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true, "CROSS": true}

// Format lays a statement out one clause per line: keywords upper-cased,
// select-list items and parenthesized lists broken at commas, conditions
// after AND/OR indented. It never changes strings, comments or identifiers.
func Format(sql string) string {
	toks := tokenize(sql)
	var b strings.Builder
	depth := 0 // parenthesis depth; clause breaks only at depth 0
	atLineStart := true
	indent := func(n int) string { return strings.Repeat("  ", n) }
	newline := func(level int) {
		s := strings.TrimRight(b.String(), " ")
		b.Reset()
		b.WriteString(s)
		if b.Len() > 0 && !strings.HasSuffix(s, "\n") {
			b.WriteString("\n")
		}
		b.WriteString(indent(level))
		atLineStart = true
	}
	emit := func(s string) {
		if !atLineStart && b.Len() > 0 {
			last := b.String()[b.Len()-1]
			if last != ' ' && last != '(' && last != '\n' && s != ")" && s != "," && s != ";" && s != "." && last != '.' {
				b.WriteByte(' ')
			}
		}
		b.WriteString(s)
		atLineStart = false
	}
	prevWord := ""
	inSelect, inBetween := false, false
	for idx := 0; idx < len(toks); idx++ {
		tk := toks[idx]
		switch tk.kind {
		case tkSpace:
			continue
		case tkComment:
			emit(tk.text)
			if strings.HasPrefix(tk.text, "--") || strings.HasPrefix(tk.text, "#") {
				newline(min(depth, 1))
			}
		case tkWord:
			up := strings.ToUpper(tk.text)
			word := tk.text
			if keywords[up] {
				word = up
			}
			if depth == 0 {
				switch {
				case up == "SELECT":
					if b.Len() > 0 && prevWord != "" && prevWord != "AS" && prevWord != "ALL" && prevWord != "UNION" {
						newline(0)
					}
					inSelect = true
				case up == "BY" && (prevWord == "GROUP" || prevWord == "ORDER"):
					inSelect = false
				case (up == "GROUP" || up == "ORDER") && peek(toks, idx) == "BY":
					newline(0)
					inSelect = false
				case clauseStarts[up]:
					newline(0)
					inSelect = false
				case joinWords[up] && !joinWords[prevWord] && prevWord != "OUTER":
					newline(0)
				case up == "AND" && inBetween:
					inBetween = false
				case up == "AND" || up == "OR":
					newline(1)
				}
			}
			emit(word)
			if up == "BETWEEN" {
				inBetween = true // the AND of BETWEEN … AND … stays inline
			}
			prevWord = up
		case tkSymbol:
			switch tk.text {
			case "(":
				emit("(")
				depth++
			case ")":
				depth = max(0, depth-1)
				emit(")")
			case ",":
				emit(",")
				if depth == 0 && inSelect {
					newline(1)
				}
			case ";":
				emit(";")
				newline(0)
				prevWord = ""
				inSelect = false
			case ".":
				b.WriteString(".")
				atLineStart = false
				// glue next token
				if idx+1 < len(toks) {
					nt := toks[idx+1]
					b.WriteString(nt.text)
					idx++
				}
			default:
				emit(tk.text)
			}
		default:
			emit(tk.text)
		}
	}
	return strings.TrimSpace(b.String())
}

func peek(toks []token, i int) string {
	for j := i + 1; j < len(toks); j++ {
		if toks[j].kind == tkSpace {
			continue
		}
		return strings.ToUpper(toks[j].text)
	}
	return ""
}
