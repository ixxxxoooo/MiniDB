package database

import "fmt"

// 外键引用写成 "表.列"，放在 ColumnInfo.ForeignKey 里。MySQL 系的读取在 metadata.go。

// fillSQLiteForeignKeys 用 PRAGMA foreign_key_list 补全外键。读取失败时静默跳过。
func fillSQLiteForeignKeys(db Executor, safeName string, columns []ColumnInfo) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", safeName))
	if err != nil {
		return
	}
	defer rows.Close()
	refs := map[string]string{}
	for rows.Next() {
		var id, seq int
		var table, from string
		var to, onUpdate, onDelete, match *string
		if rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match) != nil {
			continue
		}
		ref := table
		if to != nil && *to != "" {
			ref += "." + *to
		}
		refs[from] = ref
	}
	for i := range columns {
		if ref, ok := refs[columns[i].Name]; ok {
			columns[i].ForeignKey = ref
		}
	}
}

// fillPostgresForeignKeys 读取当前 schema 下该表的外键。读取失败时静默跳过。
func fillPostgresForeignKeys(db Executor, tableName string, columns []ColumnInfo) {
	rows, err := db.Query(`SELECT kcu.column_name, ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1 AND tc.table_schema = current_schema()`, tableName)
	if err != nil {
		return
	}
	defer rows.Close()
	refs := map[string]string{}
	for rows.Next() {
		var col, refTable, refCol string
		if rows.Scan(&col, &refTable, &refCol) == nil {
			refs[col] = refTable + "." + refCol
		}
	}
	for i := range columns {
		if ref, ok := refs[columns[i].Name]; ok {
			columns[i].ForeignKey = ref
		}
	}
}

// SplitForeignKey 把 "表.列" 拆开；没有列时只返回表名。
func SplitForeignKey(ref string) (table, column string) {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '.' {
			return ref[:i], ref[i+1:]
		}
	}
	return ref, ""
}
