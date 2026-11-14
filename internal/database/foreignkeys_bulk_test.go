package database

import (
	"context"
	"reflect"
	"testing"
)

// SQLite 没有批量接口，AllForeignKeys 逐表退回，结果仍含联合外键和级联动作。
func TestAllForeignKeysFallsBackPerTable(t *testing.T) {
	db := newMetaDB(t)
	for _, s := range []string{
		`CREATE TABLE pairs (a INTEGER, b INTEGER, PRIMARY KEY (a, b))`,
		`CREATE TABLE items (id INTEGER PRIMARY KEY, pa INTEGER, pb INTEGER, FOREIGN KEY (pa, pb) REFERENCES pairs(a, b))`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	d, _ := DialectOf(SQLite)
	if _, ok := d.(BulkForeignKeys); ok {
		t.Fatal("SQLite 不应实现批量接口")
	}
	got, err := AllForeignKeys(context.Background(), db, SQLite, "", []string{"users", "orders", "pairs", "items"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("应有 2 个外键: %+v", got)
	}
	by := map[string]TableForeignKey{}
	for _, fk := range got {
		by[fk.Table] = fk
	}
	if o := by["orders"]; o.RefTable != "users" || o.OnDelete != "CASCADE" || !reflect.DeepEqual(o.Columns, []string{"user_id"}) {
		t.Errorf("orders: %+v", o)
	}
	if i := by["items"]; i.RefTable != "pairs" || !reflect.DeepEqual(i.Columns, []string{"pa", "pb"}) || !reflect.DeepEqual(i.RefColumns, []string{"a", "b"}) {
		t.Errorf("联合外键: %+v", i)
	}
}

// MySQL、PostgreSQL 实现了批量接口；这里验证把多行合并成联合外键的逻辑，真库的查询在集成测试里跑。
func TestFKGrouperMergesCompositeKeys(t *testing.T) {
	var g fkGrouper
	g.add("items", "fk_pair", "pa", "pairs", "a", "NO ACTION", "CASCADE")
	g.add("items", "fk_pair", "pb", "pairs", "b", "NO ACTION", "CASCADE")
	g.add("items", "fk_owner", "owner", "users", "id", "NO ACTION", "NO ACTION")
	g.add("logs", "fk_owner", "owner", "users", "id", "NO ACTION", "NO ACTION") // 不同表的同名约束不能合并
	if len(g.out) != 3 {
		t.Fatalf("应合并成 3 个: %+v", g.out)
	}
	if fk := g.out[0]; !reflect.DeepEqual(fk.Columns, []string{"pa", "pb"}) || !reflect.DeepEqual(fk.RefColumns, []string{"a", "b"}) || fk.OnDelete != "CASCADE" {
		t.Errorf("联合外键: %+v", fk)
	}
	for _, typ := range []DBType{MySQL, Postgres, TiDB} {
		d, _ := DialectOf(typ)
		if _, ok := d.(BulkForeignKeys); !ok {
			t.Errorf("%s 应实现批量接口", typ)
		}
	}
}
