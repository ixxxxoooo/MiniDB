package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// newMetaDB 建一个带两张表、一个视图、一个索引和一个触发器的 SQLite 库，用作元数据读取的回归保护。
func newMetaDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT DEFAULT 'a@b.c')`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id) ON DELETE CASCADE, amount REAL)`,
		`CREATE UNIQUE INDEX idx_users_email ON users(email)`,
		`CREATE INDEX idx_orders_user_amount ON orders(user_id, amount)`,
		`CREATE VIEW v_big_orders AS SELECT * FROM orders WHERE amount > 100`,
		`CREATE TRIGGER trg_orders_ins AFTER INSERT ON orders BEGIN SELECT 1; END`,
		`INSERT INTO users(name, email) VALUES ('u1','u1@x'), ('u2','u2@x'), ('u3','u3@x')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	return db
}

func TestSQLiteGetServerVersion(t *testing.T) {
	db := newMetaDB(t)
	v, err := GetServerVersion(db, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v, "SQLite ") {
		t.Fatalf("版本 = %q，应以 'SQLite ' 开头", v)
	}
}

func TestSQLiteGetDatabases(t *testing.T) {
	db := newMetaDB(t)
	for name, fn := range map[string]func(Executor, string) ([]DatabaseInfo, error){
		"GetDatabases": GetDatabases, "GetDatabaseNames": GetDatabaseNames,
	} {
		dbs, err := fn(db, "sqlite")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(dbs) != 1 || dbs[0].Name != "main" {
			t.Fatalf("%s = %+v，SQLite 应只有 main", name, dbs)
		}
	}
}

func TestSQLiteGetTables(t *testing.T) {
	db := newMetaDB(t)
	tables, err := GetTables(db, "sqlite", "main")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tb := range tables {
		got[tb.Name] = tb.Type
	}
	want := map[string]string{"orders": "table", "users": "table", "v_big_orders": "view"}
	if len(got) != len(want) {
		t.Fatalf("表 = %v，想要 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s 类型 = %q，想要 %q", k, got[k], v)
		}
	}
	// 结果按名称排序
	for i := 1; i < len(tables); i++ {
		if tables[i-1].Name > tables[i].Name {
			t.Errorf("表未按名称排序: %v", tables)
		}
	}
}

func TestSQLiteGetColumns(t *testing.T) {
	db := newMetaDB(t)
	cols, err := GetColumns(db, "sqlite", "main", "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("列数 = %d，想要 3", len(cols))
	}
	byName := map[string]ColumnInfo{}
	for _, c := range cols {
		byName[c.Name] = c
	}
	if !byName["id"].IsPrimary {
		t.Error("id 应为主键")
	}
	if byName["name"].Nullable {
		t.Error("name 是 NOT NULL，Nullable 应为 false")
	}
	if !byName["email"].Nullable {
		t.Error("email 可空")
	}
	if d := byName["email"].DefaultValue; d == nil || *d != "'a@b.c'" {
		t.Errorf("email 默认值 = %v", d)
	}

	// 外键信息挂在列上
	ocols, err := GetColumns(db, "sqlite", "main", "orders")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ocols {
		if c.Name == "user_id" && c.ForeignKey != "users.id" {
			t.Errorf("user_id 外键 = %q", c.ForeignKey)
		}
	}
}

func TestSQLiteGetColumnsRejectsUnsafeName(t *testing.T) {
	db := newMetaDB(t)
	if _, err := GetColumns(db, "sqlite", "main", "users; DROP TABLE users"); err == nil {
		t.Fatal("含非法字符的表名应被拒绝")
	}
	if _, err := GetIndexes(db, "sqlite", "main", "a b"); err == nil {
		t.Fatal("含空格的表名应被拒绝")
	}
}

func TestSQLiteGetIndexes(t *testing.T) {
	db := newMetaDB(t)
	idx, err := GetIndexes(db, "sqlite", "main", "orders")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ix := range idx {
		if ix.Name == "idx_orders_user_amount" {
			found = true
			if ix.IsUnique || len(ix.Columns) != 2 || ix.Columns[0] != "user_id" || ix.Columns[1] != "amount" {
				t.Errorf("索引 = %+v", ix)
			}
		}
	}
	if !found {
		t.Fatalf("没有读到 idx_orders_user_amount: %+v", idx)
	}

	uidx, err := GetIndexes(db, "sqlite", "main", "users")
	if err != nil {
		t.Fatal(err)
	}
	for _, ix := range uidx {
		if ix.Name == "idx_users_email" && !ix.IsUnique {
			t.Error("idx_users_email 应为唯一索引")
		}
	}
}

func TestSQLiteGetDDL(t *testing.T) {
	db := newMetaDB(t)
	ddl, err := GetDDL(db, "sqlite", "main", "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ddl, "CREATE TABLE users") {
		t.Fatalf("DDL = %q", ddl)
	}
	if _, err := GetDDL(db, "sqlite", "main", "no_such_table"); err == nil {
		t.Fatal("不存在的表应返回错误")
	}
}

func TestSQLiteGetTableStats(t *testing.T) {
	db := newMetaDB(t)
	st, err := GetTableStats(db, "sqlite", "main", "users")
	if err != nil {
		t.Fatal(err)
	}
	if st.RowCount != 3 || st.Engine != "SQLite" {
		t.Fatalf("统计 = %+v", st)
	}
}

func TestSQLiteForeignKeysAndTriggers(t *testing.T) {
	db := newMetaDB(t)
	fks, err := GetForeignKeys(db, "sqlite", "main", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(fks) != 1 || fks[0].RefTable != "users" || fks[0].OnDelete != "CASCADE" {
		t.Fatalf("外键 = %+v", fks)
	}
	trs, err := GetTriggers(db, "sqlite", "main", "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(trs) != 1 || trs[0].Name != "trg_orders_ins" || trs[0].Timing != "AFTER" || trs[0].Event != "INSERT" {
		t.Fatalf("触发器 = %+v", trs)
	}
	rs, err := GetRoutines(db, "sqlite", "main")
	if err != nil || len(rs) != 0 {
		t.Fatalf("SQLite 没有存储过程: %v %v", rs, err)
	}
}

func TestSQLiteGetTableOverview(t *testing.T) {
	db := newMetaDB(t)
	ov, err := GetTableOverview(db, "sqlite", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(ov) != 3 {
		t.Fatalf("概览 = %+v", ov)
	}
}

func TestUnsupportedDBTypeReturnsError(t *testing.T) {
	db := newMetaDB(t)
	if _, err := GetTables(db, "oracle", "x"); err == nil {
		t.Fatal("不支持的库类型应返回错误")
	}
	if _, err := GetServerVersion(db, "oracle"); err == nil {
		t.Fatal("不支持的库类型应返回错误")
	}
}

func TestParseEnumOptionsFromType(t *testing.T) {
	cases := map[string][]string{
		"enum('a','b')":       {"a", "b"},
		"set('x','y','z')":    {"x", "y", "z"},
		"enum('it''s','ok')":  {"it's", "ok"},
		"varchar(20)":         nil,
		"ENUM('Red','Green')": {"Red", "Green"},
		"enum('a,b','c')":     {"a,b", "c"},
	}
	for in, want := range cases {
		got := parseEnumOptionsFromType(in)
		if len(got) != len(want) {
			t.Errorf("%s = %v，想要 %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s[%d] = %q，想要 %q", in, i, got[i], want[i])
			}
		}
	}
}
