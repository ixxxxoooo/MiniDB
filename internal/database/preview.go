package database

import (
	"fmt"
	"minidb/internal/sqltext"
	"sort"
	"strings"
	"time"
)

// SQLLiteral renders v as an inline SQL literal, for showing a statement to
// the user. Execution always binds parameters instead.
func SQLLiteral(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if x {
			return "1"
		}
		return "0"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(x)
	case time.Time:
		return "'" + x.Format("2006-01-02 15:04:05") + "'"
	case []byte:
		return sqltext.QuoteString(string(x))
	}
	return sqltext.QuoteString(fmt.Sprint(v))
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func equalities(dbType DBType, m map[string]interface{}, sep string) string {
	parts := make([]string, 0, len(m))
	for _, k := range sortedKeys(m) {
		if m[k] == nil && sep == " AND " {
			parts = append(parts, QuoteIdent(dbType, k)+" IS NULL")
			continue
		}
		parts = append(parts, QuoteIdent(dbType, k)+" = "+SQLLiteral(m[k]))
	}
	return strings.Join(parts, sep)
}

// PreviewChanges returns the statements CommitTableDataChanges would run, in
// its order (deletes, inserts, updates), with values inlined.
func PreviewChanges(dbType DBType, dbName, table string, deletePKs, inserts []map[string]interface{}, updates []RowChange) []string {
	tbl := quoteTable(dbType, dbName, table)
	var out []string
	for _, pk := range deletePKs {
		out = append(out, fmt.Sprintf("DELETE FROM %s WHERE %s;", tbl, equalities(dbType, pk, " AND ")))
	}
	for _, row := range inserts {
		keys := sortedKeys(row)
		cols := make([]string, len(keys))
		vals := make([]string, len(keys))
		for i, k := range keys {
			cols[i], vals[i] = QuoteIdent(dbType, k), SQLLiteral(row[k])
		}
		out = append(out, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", tbl, strings.Join(cols, ", "), strings.Join(vals, ", ")))
	}
	for _, u := range updates {
		out = append(out, fmt.Sprintf("UPDATE %s SET %s WHERE %s;", tbl, equalities(dbType, u.Changes, ", "), equalities(dbType, u.PrimaryKey, " AND ")))
	}
	return out
}
