package ui

import (
	"path/filepath"
	"strings"
)

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, " ⏎ ")
}

// insertSQL puts sql into the editor at the caret, on its own line.
func (q *queryTab) insertSQL(sql string) {
	src := q.ed.Buffer.String()
	text := sql
	if strings.TrimSpace(src) != "" {
		text = "\n" + sql
		q.ed.Select(len(src), len(src))
	}
	q.ed.Insert(text)
}

func histDB(db string) string {
	if db == "" {
		return ""
	}
	return " · " + filepath.Base(db)
}
