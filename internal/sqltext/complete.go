package sqltext

import (
	"regexp"
	"sort"
	"strings"
)

// Column and Table describe the schema completions draw on.
type Column struct{ Name, Type string }
type Table struct {
	Name    string
	Columns []Column
}

// ItemKind says what a completion is.
type ItemKind int

const (
	KindKeyword ItemKind = iota
	KindTable
	KindColumn
)

// Item is one completion.
type Item struct {
	Label  string
	Detail string
	Kind   ItemKind
}

var sqlKeywords = strings.Fields(`SELECT FROM WHERE GROUP BY ORDER HAVING LIMIT OFFSET JOIN LEFT RIGHT INNER OUTER CROSS ON AS AND OR NOT IN IS NULL LIKE
BETWEEN EXISTS CASE WHEN THEN ELSE END DISTINCT UNION ALL INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE ALTER DROP INDEX
COUNT SUM AVG MIN MAX COALESCE IFNULL CAST ASC DESC WITH EXPLAIN TRUNCATE`)

var aliasRe = regexp.MustCompile("(?i)\\b(?:from|join|update|into)\\s+[`\"]?([A-Za-z0-9_$]+)[`\"]?(?:\\s+(?:as\\s+)?([A-Za-z_][A-Za-z0-9_]*))?")

// aliases maps alias (and table name) → table name for the tables a
// statement mentions.
func aliases(stmt string) map[string]string {
	m := map[string]string{}
	for _, g := range aliasRe.FindAllStringSubmatch(stmt, -1) {
		table := g[1]
		m[strings.ToLower(table)] = table
		if a := g[2]; a != "" && !isReserved(a) {
			m[strings.ToLower(a)] = table
		}
	}
	return m
}

func isReserved(w string) bool {
	switch strings.ToUpper(w) {
	case "WHERE", "ON", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS", "JOIN", "GROUP", "ORDER", "LIMIT", "HAVING", "UNION", "SET", "VALUES", "USING", "FULL", "NATURAL":
		return true
	}
	return false
}

// Complete suggests what can follow src[:offset], whose last word begins at
// offset-len(prefix). After "x." it offers the columns of table or alias x;
// elsewhere keywords, the tables, and the columns of the tables the current
// statement mentions. Matching is by case-insensitive prefix; results are
// ordered columns, tables, keywords.
func Complete(src string, offset int, prefix string, tables []Table) []Item {
	offset = min(max(offset, 0), len(src))
	wordStart := max(offset-len(prefix), 0)
	byName := map[string]Table{}
	for _, t := range tables {
		byName[strings.ToLower(t.Name)] = t
	}
	stmt := src
	if s, ok := At(src, wordStart); ok {
		stmt = s.Text
	}
	lp := strings.ToLower(prefix)
	match := func(label string) bool { return lp == "" || strings.HasPrefix(strings.ToLower(label), lp) }

	// qualified: "alias.<prefix>"
	if wordStart > 0 && src[wordStart-1] == '.' {
		j := wordStart - 1
		i := j
		for i > 0 && (isIdent(src[i-1])) {
			i--
		}
		qual := strings.Trim(src[i:j], "`\"")
		var out []Item
		if table, ok := aliases(stmt)[strings.ToLower(qual)]; ok {
			if t, ok := byName[strings.ToLower(table)]; ok {
				for _, c := range t.Columns {
					if match(c.Name) {
						out = append(out, Item{c.Name, c.Type, KindColumn})
					}
				}
			}
		} else if t, ok := byName[strings.ToLower(qual)]; ok { // database.table? treat qual as table
			for _, c := range t.Columns {
				if match(c.Name) {
					out = append(out, Item{c.Name, c.Type, KindColumn})
				}
			}
		}
		return out
	}

	var cols, tbls, kws []Item
	seen := map[string]bool{}
	for _, table := range aliases(stmt) {
		t, ok := byName[strings.ToLower(table)]
		if !ok || seen[strings.ToLower(t.Name)] {
			continue
		}
		seen[strings.ToLower(t.Name)] = true
		for _, c := range t.Columns {
			if match(c.Name) {
				cols = append(cols, Item{c.Name, t.Name + " · " + c.Type, KindColumn})
			}
		}
	}
	sort.SliceStable(cols, func(i, j int) bool { return cols[i].Label < cols[j].Label })
	for _, t := range tables {
		if match(t.Name) {
			tbls = append(tbls, Item{t.Name, "table", KindTable})
		}
	}
	for _, k := range sqlKeywords {
		if match(k) && lp != "" { // keywords only once something is typed
			kws = append(kws, Item{k, "keyword", KindKeyword})
		}
	}
	return append(append(cols, tbls...), kws...)
}

func isIdent(b byte) bool {
	return b == '_' || b == '$' || b == '`' || b == '"' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b >= 0x80
}
