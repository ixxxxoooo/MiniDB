package database

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// 表之外的数据库对象：外键、触发器、函数与存储过程，以及库级操作和表克隆。
// 读取各方言各写一份；生成 SQL 的函数都是纯函数，便于预览与测试。
// SQLite 没有函数与存储过程；StarRocks 没有触发器和外键，读取返回空。

// ForeignKeyInfo 是表上的一条外键约束（可以多列）。
type ForeignKeyInfo struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnUpdate   string   `json:"onUpdate"`
	OnDelete   string   `json:"onDelete"`
}

// FKActions 是外键的 ON UPDATE / ON DELETE 可选动作。
var FKActions = []string{"NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT"}

// TriggerInfo 是表上的一个触发器。Definition 是完整的 CREATE TRIGGER 语句。
type TriggerInfo struct {
	Name       string `json:"name"`
	Timing     string `json:"timing"`
	Event      string `json:"event"`
	Definition string `json:"definition"`
}

// RoutineInfo 是一个函数或存储过程。
type RoutineInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // function / procedure
}

// ---- 外键 ----

// GetForeignKeys 读取表上的外键。
func GetForeignKeys(db Executor, dbType, dbName, table string) ([]ForeignKeyInfo, error) {
	switch {
	case IsMySQLCompatible(dbType):
		if dbType == "starrocks" {
			return nil, nil
		}
		return getMySQLForeignKeys(db, dbName, table)
	case dbType == "postgres":
		return getPostgresForeignKeys(db, table)
	case dbType == "sqlite":
		return getSQLiteForeignKeys(db, table)
	}
	return nil, nil
}

func getMySQLForeignKeys(db Executor, dbName, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.Query(`SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
			r.UPDATE_RULE, r.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE k
		JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME
		WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, dbName, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byName := map[string]*ForeignKeyInfo{}
	var order []string
	for rows.Next() {
		var name, col, refTable, refCol, upd, del string
		if err := rows.Scan(&name, &col, &refTable, &refCol, &upd, &del); err != nil {
			return nil, err
		}
		fk, ok := byName[name]
		if !ok {
			fk = &ForeignKeyInfo{Name: name, RefTable: refTable, OnUpdate: upd, OnDelete: del}
			byName[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, col)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	out := make([]ForeignKeyInfo, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out, rows.Err()
}

func getPostgresForeignKeys(db Executor, table string) ([]ForeignKeyInfo, error) {
	rows, err := db.Query(`SELECT con.conname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.conkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
			cl.relname,
			(SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(con.confkey) WITH ORDINALITY k(attnum, ord)
			   JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum),
			con.confupdtype::text, con.confdeltype::text
		FROM pg_constraint con JOIN pg_class cl ON cl.oid = con.confrelid
		WHERE con.contype = 'f' AND con.conrelid = quote_ident($1)::regclass
		ORDER BY con.conname`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ForeignKeyInfo
	for rows.Next() {
		var name, cols, refTable, refCols, upd, del string
		if err := rows.Scan(&name, &cols, &refTable, &refCols, &upd, &del); err != nil {
			return nil, err
		}
		out = append(out, ForeignKeyInfo{Name: name, Columns: strings.Split(cols, ","), RefTable: refTable,
			RefColumns: strings.Split(refCols, ","), OnUpdate: pgFKAction(upd), OnDelete: pgFKAction(del)})
	}
	return out, rows.Err()
}

func pgFKAction(code string) string {
	switch code {
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	}
	return "NO ACTION"
}

func getSQLiteForeignKeys(db Executor, table string) ([]ForeignKeyInfo, error) {
	safe, err := safeSQLitePragmaName(table)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", safe))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int]*ForeignKeyInfo{}
	var ids []int
	for rows.Next() {
		var id, seq int
		var refTable, from string
		var to, upd, del, match *string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &upd, &del, &match); err != nil {
			return nil, err
		}
		fk, ok := byID[id]
		if !ok {
			fk = &ForeignKeyInfo{RefTable: refTable, OnUpdate: deref(upd, "NO ACTION"), OnDelete: deref(del, "NO ACTION")}
			byID[id] = fk
			ids = append(ids, id)
		}
		fk.Columns = append(fk.Columns, from)
		if to != nil {
			fk.RefColumns = append(fk.RefColumns, *to)
		}
	}
	sort.Ints(ids)
	out := make([]ForeignKeyInfo, 0, len(ids))
	for _, id := range ids {
		fk := byID[id]
		fk.Name = "fk_" + table + "_" + strings.Join(fk.Columns, "_") // SQLite 的外键没有名字
		out = append(out, *fk)
	}
	return out, rows.Err()
}

func deref(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

func validFKAction(a string) (string, error) {
	a = strings.ToUpper(strings.TrimSpace(a))
	if a == "" {
		return "NO ACTION", nil
	}
	for _, v := range FKActions {
		if v == a {
			return a, nil
		}
	}
	return "", fmt.Errorf("不支持的外键动作: %s", a)
}

func quoteList(dbType string, names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = QuoteIdent(dbType, n)
	}
	return strings.Join(q, ", ")
}

// BuildAddForeignKeySQL 生成添加外键的语句。SQLite 不能给已有的表加外键。
func BuildAddForeignKeySQL(dbType, table string, fk ForeignKeyInfo) (string, error) {
	if dbType == "sqlite" {
		return "", fmt.Errorf("SQLite 不支持给已有的表添加外键，需要重建表")
	}
	if dbType == "starrocks" {
		return "", fmt.Errorf("StarRocks 不支持外键")
	}
	if strings.TrimSpace(fk.Name) == "" || len(fk.Columns) == 0 || fk.RefTable == "" || len(fk.RefColumns) != len(fk.Columns) {
		return "", fmt.Errorf("外键需要名称、列、被引用的表和与列数相同的被引用列")
	}
	upd, err := validFKAction(fk.OnUpdate)
	if err != nil {
		return "", err
	}
	del, err := validFKAction(fk.OnDelete)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) ON UPDATE %s ON DELETE %s",
		QuoteTableName(dbType, table), QuoteIdent(dbType, fk.Name), quoteList(dbType, fk.Columns),
		QuoteTableName(dbType, fk.RefTable), quoteList(dbType, fk.RefColumns), upd, del), nil
}

// BuildDropForeignKeySQL 生成删除外键的语句。
func BuildDropForeignKeySQL(dbType, table, name string) (string, error) {
	switch {
	case dbType == "sqlite":
		return "", fmt.Errorf("SQLite 不支持删除外键，需要重建表")
	case dbType == "starrocks":
		return "", fmt.Errorf("StarRocks 不支持外键")
	case dbType == "postgres":
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", QuoteTableName(dbType, table), QuoteIdent(dbType, name)), nil
	}
	return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", QuoteTableName(dbType, table), QuoteIdent(dbType, name)), nil
}

// ---- 触发器 ----

// GetTriggers 读取表上的触发器。
func GetTriggers(db Executor, dbType, dbName, table string) ([]TriggerInfo, error) {
	switch {
	case dbType == "starrocks":
		return nil, nil
	case IsMySQLCompatible(dbType):
		return getMySQLTriggers(db, dbName, table)
	case dbType == "postgres":
		return getPostgresTriggers(db, table)
	case dbType == "sqlite":
		return getSQLiteTriggers(db, table)
	}
	return nil, nil
}

func getMySQLTriggers(db Executor, dbName, table string) ([]TriggerInfo, error) {
	rows, err := db.Query(`SELECT TRIGGER_NAME, ACTION_TIMING, EVENT_MANIPULATION, ACTION_STATEMENT
		FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA = ? AND EVENT_OBJECT_TABLE = ? ORDER BY TRIGGER_NAME`, dbName, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TriggerInfo
	for rows.Next() {
		var t TriggerInfo
		var body string
		if err := rows.Scan(&t.Name, &t.Timing, &t.Event, &body); err != nil {
			return nil, err
		}
		t.Definition = fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW %s",
			QuoteIdent("mysql", t.Name), t.Timing, t.Event, QuoteIdent("mysql", table), body)
		out = append(out, t)
	}
	return out, rows.Err()
}

func getPostgresTriggers(db Executor, table string) ([]TriggerInfo, error) {
	rows, err := db.Query(`SELECT t.tgname, pg_get_triggerdef(t.oid)
		FROM pg_trigger t WHERE t.tgrelid = quote_ident($1)::regclass AND NOT t.tgisinternal ORDER BY t.tgname`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TriggerInfo
	for rows.Next() {
		var t TriggerInfo
		if err := rows.Scan(&t.Name, &t.Definition); err != nil {
			return nil, err
		}
		t.Timing, t.Event = parseTriggerHead(t.Definition)
		out = append(out, t)
	}
	return out, rows.Err()
}

func getSQLiteTriggers(db Executor, table string) ([]TriggerInfo, error) {
	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = ? ORDER BY name`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TriggerInfo
	for rows.Next() {
		var t TriggerInfo
		if err := rows.Scan(&t.Name, &t.Definition); err != nil {
			return nil, err
		}
		t.Timing, t.Event = parseTriggerHead(t.Definition)
		out = append(out, t)
	}
	return out, rows.Err()
}

var triggerHead = regexp.MustCompile(`(?i)\b(BEFORE|AFTER|INSTEAD\s+OF)\s+(INSERT|UPDATE|DELETE|TRUNCATE)`)

// parseTriggerHead 从 CREATE TRIGGER 语句里读出触发时机与事件。
func parseTriggerHead(def string) (timing, event string) {
	m := triggerHead.FindStringSubmatch(def)
	if m == nil {
		return "", ""
	}
	return strings.ToUpper(strings.Join(strings.Fields(m[1]), " ")), strings.ToUpper(m[2])
}

// TriggerTimings 与 TriggerEvents 是新建触发器时的可选项。
var (
	TriggerTimings = []string{"BEFORE", "AFTER"}
	TriggerEvents  = []string{"INSERT", "UPDATE", "DELETE"}
)

// BuildCreateTriggerSQL 生成创建触发器的语句。body 在 MySQL 里是触发的语句（可用 BEGIN…END），
// 在 SQLite 里是 BEGIN 与 END 之间的语句，在 PostgreSQL 里是要执行的触发器函数名。
func BuildCreateTriggerSQL(dbType, table string, tr TriggerInfo) (string, error) {
	if dbType == "starrocks" {
		return "", fmt.Errorf("StarRocks 不支持触发器")
	}
	name, body := strings.TrimSpace(tr.Name), strings.TrimSpace(tr.Definition)
	timing, event := strings.ToUpper(strings.TrimSpace(tr.Timing)), strings.ToUpper(strings.TrimSpace(tr.Event))
	if name == "" || body == "" {
		return "", fmt.Errorf("触发器需要名称和内容")
	}
	if !inList(TriggerTimings, timing) || !inList(TriggerEvents, event) {
		return "", fmt.Errorf("不支持的触发时机或事件: %s %s", timing, event)
	}
	tbl := QuoteTableName(dbType, table)
	switch dbType {
	case "postgres":
		return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			QuoteIdent(dbType, name), timing, event, tbl, strings.TrimSuffix(body, "()")), nil
	case "sqlite":
		return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW BEGIN %s; END",
			QuoteIdent(dbType, name), timing, event, tbl, strings.TrimSuffix(body, ";")), nil
	}
	return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW %s", QuoteIdent(dbType, name), timing, event, tbl, body), nil
}

// BuildDropTriggerSQL 生成删除触发器的语句。
func BuildDropTriggerSQL(dbType, table, name string) string {
	if dbType == "postgres" {
		return fmt.Sprintf("DROP TRIGGER %s ON %s", QuoteIdent(dbType, name), QuoteTableName(dbType, table))
	}
	return "DROP TRIGGER " + QuoteIdent(dbType, name)
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ---- 函数与存储过程 ----

// GetRoutines 列出库里的函数与存储过程。
func GetRoutines(db Executor, dbType, dbName string) ([]RoutineInfo, error) {
	switch {
	case dbType == "starrocks" || dbType == "sqlite":
		return nil, nil
	case IsMySQLCompatible(dbType):
		rows, err := db.Query(`SELECT ROUTINE_NAME, ROUTINE_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_NAME`, dbName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []RoutineInfo
		for rows.Next() {
			var r RoutineInfo
			if err := rows.Scan(&r.Name, &r.Kind); err != nil {
				return nil, err
			}
			r.Kind = strings.ToLower(r.Kind)
			out = append(out, r)
		}
		return out, rows.Err()
	case dbType == "postgres":
		rows, err := db.Query(`SELECT p.proname, CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END
			FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = current_schema() AND p.prokind IN ('f', 'p') ORDER BY p.proname`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []RoutineInfo
		for rows.Next() {
			var r RoutineInfo
			if err := rows.Scan(&r.Name, &r.Kind); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, rows.Err()
	}
	return nil, nil
}

// GetRoutineDDL 读取函数或存储过程的完整定义。
func GetRoutineDDL(db Executor, dbType, dbName, kind, name string) (string, error) {
	switch {
	case IsMySQLCompatible(dbType) && dbType != "starrocks":
		if kind != "function" && kind != "procedure" {
			return "", fmt.Errorf("未知的例程类型: %s", kind)
		}
		return showCreate(db, fmt.Sprintf("SHOW CREATE %s %s.%s", strings.ToUpper(kind), QuoteIdent(dbType, dbName), QuoteIdent(dbType, name)), 2)
	case dbType == "postgres":
		var ddl string
		err := db.QueryRow(`SELECT pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = current_schema() AND p.proname = $1 LIMIT 1`, name).Scan(&ddl)
		return ddl, err
	}
	return "", fmt.Errorf("%s 不支持存储过程与函数", dbType)
}

// showCreate 执行 SHOW CREATE …，取第 col 列（从 0 起）的定义；不同对象返回的列数不同。
func showCreate(db Executor, stmt string, col int) (string, error) {
	rows, err := db.Query(stmt)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", fmt.Errorf("没有返回定义")
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	if col >= len(vals) {
		col = len(vals) - 1
	}
	switch v := vals[col].(type) {
	case []byte:
		return string(v), nil
	case string:
		return v, nil
	case nil:
		return "", fmt.Errorf("定义为空（可能没有权限）")
	default:
		return fmt.Sprint(v), nil
	}
}

// BuildDropRoutineSQL 生成删除函数或存储过程的语句。
func BuildDropRoutineSQL(dbType, kind, name string) (string, error) {
	if kind != "function" && kind != "procedure" {
		return "", fmt.Errorf("未知的例程类型: %s", kind)
	}
	return fmt.Sprintf("DROP %s %s", strings.ToUpper(kind), QuoteIdent(dbType, name)), nil
}

// BuildDropViewSQL 生成删除视图的语句。
func BuildDropViewSQL(dbType, name string) string { return "DROP VIEW " + QuoteTableName(dbType, name) }

// ---- 库级操作 ----

// BuildCreateDatabaseSQL 生成创建数据库的语句。charset 只用于 MySQL 系，空则用 utf8mb4。
func BuildCreateDatabaseSQL(dbType, name, charset string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("数据库名不能为空")
	}
	switch {
	case dbType == "sqlite":
		return "", fmt.Errorf("SQLite 的数据库就是文件，请新建连接")
	case dbType == "postgres":
		return "CREATE DATABASE " + QuoteIdent(dbType, name), nil
	case dbType == "starrocks":
		return "CREATE DATABASE " + QuoteIdent(dbType, name), nil
	}
	if charset == "" {
		charset = "utf8mb4"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(charset) {
		return "", fmt.Errorf("字符集名不合法: %s", charset)
	}
	return fmt.Sprintf("CREATE DATABASE %s CHARACTER SET %s", QuoteIdent(dbType, name), charset), nil
}

// BuildDropDatabaseSQL 生成删除数据库的语句。
func BuildDropDatabaseSQL(dbType, name string) (string, error) {
	if dbType == "sqlite" {
		return "", fmt.Errorf("SQLite 的数据库就是文件，请删除文件")
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("数据库名不能为空")
	}
	return "DROP DATABASE " + QuoteIdent(dbType, name), nil
}

// BuildRenameDatabaseSQL 生成重命名数据库的语句。MySQL 与 TiDB 没有这个语句，返回的是
// 空，由调用方把表逐个搬到新库。
func BuildRenameDatabaseSQL(dbType, oldName, newName string) (string, error) {
	if strings.TrimSpace(newName) == "" || oldName == newName {
		return "", fmt.Errorf("新的数据库名不能为空，也不能与原名相同")
	}
	switch dbType {
	case "postgres":
		return fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", QuoteIdent(dbType, oldName), QuoteIdent(dbType, newName)), nil
	case "starrocks":
		return fmt.Sprintf("ALTER DATABASE %s RENAME %s", QuoteIdent(dbType, oldName), QuoteIdent(dbType, newName)), nil
	case "sqlite":
		return "", fmt.Errorf("SQLite 的数据库就是文件，请重命名文件")
	}
	return "", nil
}

// BuildMoveTablesSQL 生成把表从旧库搬到新库的 RENAME TABLE（MySQL 与 TiDB）。
func BuildMoveTablesSQL(dbType, oldDB, newDB string, tables []string) string {
	if len(tables) == 0 {
		return ""
	}
	parts := make([]string, len(tables))
	for i, t := range tables {
		parts[i] = fmt.Sprintf("%s.%s TO %s.%s", QuoteIdent(dbType, oldDB), QuoteIdent(dbType, t), QuoteIdent(dbType, newDB), QuoteIdent(dbType, t))
	}
	return "RENAME TABLE " + strings.Join(parts, ", ")
}

// ---- 表克隆 ----

// BuildCloneTableSQL 生成复制表结构（可连数据）的语句。SQLite 要从 DDL 改名，走 CloneSQLiteTableSQL。
func BuildCloneTableSQL(dbType, src, dst string, withData bool) ([]string, error) {
	if strings.TrimSpace(dst) == "" || src == dst {
		return nil, fmt.Errorf("新表名不能为空，也不能与原表同名")
	}
	s, d := QuoteTableName(dbType, src), QuoteTableName(dbType, dst)
	var out []string
	switch {
	case dbType == "sqlite":
		return nil, fmt.Errorf("SQLite 请用 CloneSQLiteTableSQL")
	case dbType == "postgres":
		out = append(out, fmt.Sprintf("CREATE TABLE %s (LIKE %s INCLUDING ALL)", d, s))
	default:
		out = append(out, fmt.Sprintf("CREATE TABLE %s LIKE %s", d, s))
	}
	if withData {
		out = append(out, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", d, s))
	}
	return out, nil
}

// CloneSQLiteTableSQL 把 SQLite 的 CREATE TABLE 语句改成新表名，并按需附上复制数据的语句。
func CloneSQLiteTableSQL(ddl, src, dst string, withData bool) ([]string, error) {
	if strings.TrimSpace(dst) == "" || src == dst {
		return nil, fmt.Errorf("新表名不能为空，也不能与原表同名")
	}
	re := regexp.MustCompile("(?is)^\\s*CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:\"" + regexp.QuoteMeta(strings.ReplaceAll(src, `"`, `""`)) + "\"|`" + regexp.QuoteMeta(src) + "`|\\[" + regexp.QuoteMeta(src) + "\\]|'" + regexp.QuoteMeta(src) + "'|" + regexp.QuoteMeta(src) + ")")
	loc := re.FindStringIndex(ddl)
	if loc == nil {
		return nil, fmt.Errorf("无法从建表语句里识别表名")
	}
	out := []string{"CREATE TABLE " + QuoteIdent("sqlite", dst) + ddl[loc[1]:]}
	if withData {
		out = append(out, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", QuoteIdent("sqlite", dst), QuoteIdent("sqlite", src)))
	}
	return out, nil
}
