package database

import (
	"strings"
	"testing"
)

func modifiedIdx() []StructureIndexEdit {
	return []StructureIndexEdit{{UID: "idx_a", Status: "modified", Name: "idx_b", Columns: []string{"x", "y"}, IsUnique: true}}
}

// 改过的已有索引：三种方言都应「删旧名、建新名」。
func TestModifiedIndexDropsOldAndCreatesNew(t *testing.T) {
	for _, tc := range []struct {
		db                DBType
		ver, drop, create string
	}{
		{"mysql", "8.0.33", "DROP INDEX `idx_a`", "ADD UNIQUE INDEX `idx_b` (`x`, `y`)"},
		{"postgres", "14.5", `DROP INDEX IF EXISTS "idx_a"`, `CREATE UNIQUE INDEX "idx_b"`},
		{"sqlite", "3.40.0", `DROP INDEX IF EXISTS "idx_a"`, `CREATE UNIQUE INDEX IF NOT EXISTS "idx_b"`},
	} {
		stmts, err := BuildStructureAlterDDLStatements(tc.db, tc.ver, "t", nil, nil, modifiedIdx())
		if err != nil {
			t.Fatalf("%s: %v", tc.db, err)
		}
		joined := strings.Join(stmts, "\n")
		di, ci := strings.Index(joined, tc.drop), strings.Index(joined, tc.create)
		if di < 0 || ci < 0 || di > ci {
			t.Fatalf("%s: want %q before %q, got:\n%s", tc.db, tc.drop, tc.create, joined)
		}
	}
}

// 外键、触发器：旧的先删（pre），新的后建（post）；空白新行被忽略。
func TestStructureObjectStatements(t *testing.T) {
	e := StructureObjectEdits{
		FKs: []StructureFKEdit{
			{UID: "fk_old", Status: "modified", Name: "fk_new", Columns: []string{"uid"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: "CASCADE", OnUpdate: "NO ACTION"},
			{UID: "fk_gone", Status: "deleted"},
			{UID: "new:1", Status: "new"},
		},
		Triggers: []StructureTriggerEdit{
			{UID: "trg_old", Status: "modified", Definition: "CREATE TRIGGER trg_old BEFORE INSERT ON t FOR EACH ROW SET NEW.a = 1"},
			{UID: "new:2", Status: "new", Name: "trg_n", Timing: "AFTER", Event: "UPDATE", Definition: "SET @x = 1"},
		},
	}
	pre, post, err := BuildStructureObjectStatements("mysql", "t", e)
	if err != nil {
		t.Fatal(err)
	}
	if len(pre) != 3 || len(post) != 3 {
		t.Fatalf("pre=%v post=%v", pre, post)
	}
	if !strings.Contains(pre[0], "DROP FOREIGN KEY `fk_old`") || !strings.Contains(pre[1], "fk_gone") || !strings.Contains(pre[2], "DROP TRIGGER `trg_old`") {
		t.Fatalf("pre = %v", pre)
	}
	if !strings.Contains(post[0], "ADD CONSTRAINT `fk_new`") || !strings.HasPrefix(post[1], "CREATE TRIGGER trg_old") || !strings.Contains(post[2], "CREATE TRIGGER `trg_n` AFTER UPDATE") {
		t.Fatalf("post = %v", post)
	}
	if (StructureObjectEdits{FKs: []StructureFKEdit{{UID: "new:1", Status: "new"}}}).Empty() == false {
		t.Fatal("blank new row must not count as a change")
	}
	if e.Empty() {
		t.Fatal("edits must not be empty")
	}
}

// 改过的触发器必须给完整的 CREATE 语句。
func TestModifiedTriggerNeedsFullStatement(t *testing.T) {
	_, _, err := BuildStructureObjectStatements("sqlite", "t", StructureObjectEdits{Triggers: []StructureTriggerEdit{{UID: "a", Status: "modified", Definition: "SELECT 1"}}})
	if err == nil {
		t.Fatal("want error")
	}
}

// 改过的索引如果名称或列被清空了，不能因此把旧索引删掉。
func TestModifiedIndexWithoutColumnsKeepsOld(t *testing.T) {
	for _, db := range []DBType{MySQL, Postgres, SQLite} {
		stmts, err := BuildStructureAlterDDLStatements(db, "8.0.33", "t", nil, nil, []StructureIndexEdit{{UID: "idx_a", Status: "modified", Name: "idx_a"}})
		if err != nil || len(stmts) != 0 {
			t.Fatalf("%s: stmts=%v err=%v", db, stmts, err)
		}
	}
}

// 整套语句的顺序：先删旧外键，再改列，最后建新外键。
func TestBuildFullStructureStatementsOrder(t *testing.T) {
	objs := StructureObjectEdits{FKs: []StructureFKEdit{
		{UID: "fk_old", Status: "deleted"},
		{UID: "new:1", Status: "new", Name: "fk_n", Columns: []string{"extra"}, RefTable: "p", RefColumns: []string{"id"}, OnDelete: "CASCADE", OnUpdate: "NO ACTION"},
	}}
	cols := []StructureColumnEdit{{UID: "new:2", Status: "new", Name: "extra", Type: "int", Nullable: true}}
	stmts, err := BuildFullStructureStatements("mysql", "8.0.33", "t", cols, nil, nil, objs)
	if err != nil || len(stmts) != 3 {
		t.Fatalf("stmts=%v err=%v", stmts, err)
	}
	if !strings.Contains(stmts[0], "DROP FOREIGN KEY") || !strings.Contains(stmts[1], "ADD COLUMN") || !strings.Contains(stmts[2], "ADD CONSTRAINT") {
		t.Fatalf("顺序不对: %v", stmts)
	}
}
