package ui

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/sqltext"
)

// 查询标签的补全。

// favoriteKeywords completes a bound keyword to its whole statement.
func (q *queryTab) keywordCompletions(prefix string) []editor.Completion {
	var out []editor.Completion
	lp := strings.ToLower(prefix)
	for kw, sql := range q.w.keywords {
		if lp == "" || strings.HasPrefix(strings.ToLower(kw), lp) {
			out = append(out, editor.Completion{Label: kw, Detail: "★ " + firstLines(sql, 1), Kind: editor.CompletionSnippet, Insert: sql})
		}
	}
	return out
}

func (q *queryTab) complete(r editor.CompletionRequest) {
	src := q.ed.Buffer.String()
	var out []editor.Completion
	pf := q.w.sh.prefs
	for _, it := range sqltext.Complete(src, r.Offset, r.Prefix, q.w.schema) {
		kind := editor.CompletionKeyword
		switch it.Kind {
		case sqltext.KindTable:
			kind = editor.CompletionType
		case sqltext.KindColumn:
			kind = editor.CompletionField
		}
		insert := it.Label
		if kind == editor.CompletionKeyword && pf.CompUpper {
			insert = strings.ToUpper(insert)
		}
		if pf.CompSpace {
			insert += " "
		}
		out = append(out, editor.Completion{Label: it.Label, Detail: it.Detail, Kind: kind, Insert: insert})
	}
	out = append(q.keywordCompletions(r.Prefix), out...)
	q.ed.ShowCompletions(r.Offset-len(r.Prefix), out)
}
