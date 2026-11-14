package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildAddForeignKeySQL(t *testing.T) {
	fk := ForeignKeyInfo{Name: "fk_o_u", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: "cascade"}
	got, err := BuildAddForeignKeySQL("mysql", "orders", fk)
	want := "ALTER TABLE `orders` ADD CONSTRAINT `fk_o_u` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE"
	if err != nil || got != want {
		t.Fatalf("mysql: %q %v", got, err)
	}
	got, _ = BuildAddForeignKeySQL("postgres", "orders", fk)
	if !strings.Contains(got, `"orders" ADD CONSTRAINT "fk_o_u" FOREIGN KEY ("user_id") REFERENCES "users" ("id")`) {
		t.Fatalf("postgres: %q", got)
	}
	if _, err := BuildAddForeignKeySQL("sqlite", "orders", fk); err == nil {
		t.Fatal("SQLite 不能加外键")
	}
	if _, err := BuildAddForeignKeySQL("mysql", "orders", ForeignKeyInfo{Name: "x", Columns: []string{"a", "b"}, RefTable: "t", RefColumns: []string{"a"}}); err == nil {
		t.Fatal("列数不一致应报错")
	}
	bad := fk
	bad.OnDelete = "DROP TABLE"
	if _, err := BuildAddForeignKeySQL("mysql", "orders", bad); err == nil {
		t.Fatal("非法动作应报错，防止拼接注入")
	}
}

func TestBuildDropForeignKeySQL(t *testing.T) {
	if got, _ := BuildDropForeignKeySQL("mysql", "orders", "fk1"); got != "ALTER TABLE `orders` DROP FOREIGN KEY `fk1`" {
		t.Fatal(got)
	}
	if got, _ := BuildDropForeignKeySQL("postgres", "orders", "fk1"); got != `ALTER TABLE "orders" DROP CONSTRAINT "fk1"` {
		t.Fatal(got)
	}
}

func TestBuildCreateTriggerSQL(t *testing.T) {
	tr := TriggerInfo{Name: "trg", Timing: "before", Event: "insert", Definition: "SET NEW.x = 1"}
	if got, _ := BuildCreateTriggerSQL("mysql", "t", tr); got != "CREATE TRIGGER `trg` BEFORE INSERT ON `t` FOR EACH ROW SET NEW.x = 1" {
		t.Fatal(got)
	}
	if got, _ := BuildCreateTriggerSQL("sqlite", "t", TriggerInfo{Name: "trg", Timing: "AFTER", Event: "DELETE", Definition: "DELETE FROM log;"}); got != `CREATE TRIGGER "trg" AFTER DELETE ON "t" FOR EACH ROW BEGIN DELETE FROM log; END` {
		t.Fatal(got)
	}
	if got, _ := BuildCreateTriggerSQL("postgres", "t", TriggerInfo{Name: "trg", Timing: "AFTER", Event: "UPDATE", Definition: "audit()"}); got != `CREATE TRIGGER "trg" AFTER UPDATE ON "t" FOR EACH ROW EXECUTE FUNCTION audit()` {
		t.Fatal(got)
	}
	if _, err := BuildCreateTriggerSQL("mysql", "t", TriggerInfo{Name: "x", Timing: "DURING", Event: "INSERT", Definition: "x"}); err == nil {
		t.Fatal("非法时机应报错")
	}
	if BuildDropTriggerSQL("postgres", "t", "trg") != `DROP TRIGGER "trg" ON "t"` || BuildDropTriggerSQL("mysql", "t", "trg") != "DROP TRIGGER `trg`" {
		t.Fatal("DROP TRIGGER")
	}
	if tm, ev := parseTriggerHead(`CREATE TRIGGER x INSTEAD OF DELETE ON v BEGIN SELECT 1; END`); tm != "INSTEAD OF" || ev != "DELETE" {
		t.Fatalf("parseTriggerHead = %q %q", tm, ev)
	}
}

func TestBuildDatabaseSQL(t *testing.T) {
	if got, _ := BuildCreateDatabaseSQL("mysql", "shop", ""); got != "CREATE DATABASE `shop` CHARACTER SET utf8mb4" {
		t.Fatal(got)
	}
	if got, _ := BuildCreateDatabaseSQL("postgres", "shop", ""); got != `CREATE DATABASE "shop"` {
		t.Fatal(got)
	}
	if _, err := BuildCreateDatabaseSQL("mysql", "shop", "utf8; DROP"); err == nil {
		t.Fatal("字符集应校验")
	}
	if _, err := BuildCreateDatabaseSQL("sqlite", "x", ""); err == nil {
		t.Fatal("SQLite 不能建库")
	}
	if got, _ := BuildDropDatabaseSQL("mysql", "a`b"); got != "DROP DATABASE `a``b`" {
		t.Fatal(got)
	}
	if got, _ := BuildRenameDatabaseSQL("postgres", "a", "b"); got != `ALTER DATABASE "a" RENAME TO "b"` {
		t.Fatal(got)
	}
	if got, _ := BuildRenameDatabaseSQL("mysql", "a", "b"); got != "" {
		t.Fatalf("MySQL 没有重命名语句: %q", got)
	}
	if got := BuildMoveTablesSQL("mysql", "a", "b", []string{"t1", "t2"}); got != "RENAME TABLE `a`.`t1` TO `b`.`t1`, `a`.`t2` TO `b`.`t2`" {
		t.Fatal(got)
	}
}

func TestBuildCloneTableSQL(t *testing.T) {
	got, _ := BuildCloneTableSQL("mysql", "a", "b", true)
	if !reflect.DeepEqual(got, []string{"CREATE TABLE `b` LIKE `a`", "INSERT INTO `b` SELECT * FROM `a`"}) {
		t.Fatalf("%q", got)
	}
	got, _ = BuildCloneTableSQL("postgres", "a", "b", false)
	if !reflect.DeepEqual(got, []string{`CREATE TABLE "b" (LIKE "a" INCLUDING ALL)`}) {
		t.Fatalf("%q", got)
	}
	if _, err := BuildCloneTableSQL("mysql", "a", "a", false); err == nil {
		t.Fatal("同名应报错")
	}
	got, err := CloneSQLiteTableSQL("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)", "users", "users2", true)
	if err != nil || got[0] != `CREATE TABLE "users2" (id INTEGER PRIMARY KEY, name TEXT)` || got[1] != `INSERT INTO "users2" SELECT * FROM "users"` {
		t.Fatalf("%q %v", got, err)
	}
	got, _ = CloneSQLiteTableSQL(`CREATE TABLE "my tab" (a INT)`, "my tab", "x", false)
	if got[0] != `CREATE TABLE "x" (a INT)` {
		t.Fatalf("%q", got)
	}
}

// SQLite 端到端：读外键、触发器，视图 DDL，克隆表。
func TestSQLiteObjects(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE)`,
		`CREATE TABLE log (msg TEXT)`,
		`CREATE TRIGGER trg_users AFTER INSERT ON users FOR EACH ROW BEGIN INSERT INTO log VALUES ('x'); END`,
		`CREATE VIEW v_users AS SELECT * FROM users`,
		`INSERT INTO users(name) VALUES ('a'), ('b')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	fks, err := GetForeignKeys(context.Background(), db, "sqlite", "", "orders")
	if err != nil || len(fks) != 1 {
		t.Fatalf("fks=%+v err=%v", fks, err)
	}
	if fk := fks[0]; fk.RefTable != "users" || fk.Columns[0] != "user_id" || fk.RefColumns[0] != "id" || fk.OnDelete != "CASCADE" {
		t.Fatalf("fk = %+v", fk)
	}
	trs, err := GetTriggers(context.Background(), db, "sqlite", "", "users")
	if err != nil || len(trs) != 1 || trs[0].Name != "trg_users" || trs[0].Timing != "AFTER" || trs[0].Event != "INSERT" {
		t.Fatalf("triggers=%+v err=%v", trs, err)
	}
	ddl, err := GetDDL(context.Background(), db, "sqlite", "", "v_users")
	if err != nil || !strings.Contains(ddl, "CREATE VIEW") {
		t.Fatalf("view ddl=%q err=%v", ddl, err)
	}

	// 触发器可以通过生成的 SQL 删掉再建回来
	if _, err := db.Exec(BuildDropTriggerSQL("sqlite", "users", "trg_users")); err != nil {
		t.Fatal(err)
	}
	create, err := BuildCreateTriggerSQL("sqlite", "users", TriggerInfo{Name: "trg2", Timing: "AFTER", Event: "INSERT", Definition: "INSERT INTO log VALUES ('y')"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(create); err != nil {
		t.Fatalf("%s: %v", create, err)
	}

	// 克隆
	src, _ := GetDDL(context.Background(), db, "sqlite", "", "users")
	stmts, err := CloneSQLiteTableSQL(src, "users", "users_copy", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users_copy`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("clone rows=%d err=%v", n, err)
	}
	if rs, _ := GetRoutines(context.Background(), db, "sqlite", ""); len(rs) != 0 {
		t.Fatal("SQLite 没有存储过程")
	}
}
