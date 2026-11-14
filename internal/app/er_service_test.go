package app

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/model"
	"minidb/internal/schemaindex"
)

func newERTestEnv(t *testing.T, ddl ...string) *ERService {
	t.Helper()
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "er.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	db, _ := manager.GetDB("c1")
	for _, q := range ddl {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return NewERService(manager, func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
		return schemaindex.BuildSchemaFromDatabaseManager(ctx, manager, connID, dbName)
	}, nil)
}

func TestERGraph(t *testing.T) {
	svc := newERTestEnv(t,
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id) ON DELETE CASCADE, parent_id INTEGER REFERENCES orders(id), note TEXT)`,
		`CREATE TABLE pairs (a INTEGER, b INTEGER, PRIMARY KEY (a, b))`,
		`CREATE TABLE items (id INTEGER PRIMARY KEY, pa INTEGER, pb INTEGER, FOREIGN KEY (pa, pb) REFERENCES pairs(a, b))`,
		`CREATE TABLE lonely (id INTEGER PRIMARY KEY)`,
		`CREATE VIEW v_orders AS SELECT * FROM orders`,
	)
	g, err := svc.Graph(context.Background(), "c1", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range g.Nodes {
		names = append(names, n.Name)
	}
	if want := []string{"items", "lonely", "orders", "pairs", "users"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("节点应按名字排序且不含视图: %v", names)
	}
	if len(g.Edges) != 3 { // items→pairs（联合）、orders→orders（自引用）、orders→users
		t.Fatalf("应有 3 条边: %+v", g.Edges)
	}
	type key struct{ from, to string }
	by := map[key]model.EREdge{}
	for _, e := range g.Edges {
		by[key{e.From, e.To}] = e
	}
	if e := by[key{"orders", "users"}]; e.OnDelete != "CASCADE" || !reflect.DeepEqual(e.FromCols, []string{"user_id"}) {
		t.Errorf("orders→users: %+v", e)
	}
	if e := by[key{"items", "pairs"}]; !reflect.DeepEqual(e.FromCols, []string{"pa", "pb"}) || !reflect.DeepEqual(e.ToCols, []string{"a", "b"}) {
		t.Errorf("联合外键: %+v", e)
	}
	if _, ok := by[key{"orders", "orders"}]; !ok {
		t.Error("自引用外键应保留")
	}
	orders, _ := g.Node("orders")
	flags := map[string][2]bool{}
	for _, c := range orders.Columns {
		flags[c.Name] = [2]bool{c.PK, c.FK}
	}
	if flags["id"] != [2]bool{true, false} || flags["user_id"] != [2]bool{false, true} || flags["parent_id"] != [2]bool{false, true} || flags["note"] != [2]bool{} {
		t.Errorf("主键、外键标记: %v", flags)
	}
}

// 外键指向本库没有画的表（另一个模式、已删除的表、视图）时，这条边不返回，也不会出现悬空的端点。
func TestBuildERGraphDropsDanglingEdges(t *testing.T) {
	sc := &ai.SchemaContext{Tables: []ai.TableSchema{
		{Name: "a", Columns: []ai.ColumnSchema{{Name: "id", IsPrimary: true}, {Name: "x_id"}}},
		{Name: "b", Columns: []ai.ColumnSchema{{Name: "id", IsPrimary: true}, {Name: "a_id"}}},
	}}
	fks := []database.TableForeignKey{
		{Table: "a", ForeignKeyInfo: database.ForeignKeyInfo{Name: "fk_x", Columns: []string{"x_id"}, RefTable: "other_schema_table", RefColumns: []string{"id"}}},
		{Table: "b", ForeignKeyInfo: database.ForeignKeyInfo{Name: "fk_a", Columns: []string{"a_id"}, RefTable: "a", RefColumns: []string{"id"}}},
		{Table: "b", ForeignKeyInfo: database.ForeignKeyInfo{Name: "fk_v", Columns: []string{"a_id"}, RefTable: "a_view", RefColumns: []string{"id"}}},
	}
	g := buildERGraph(sc, map[string]bool{"a": true, "b": true}, fks)
	if len(g.Edges) != 1 || g.Edges[0].Name != "fk_a" {
		t.Fatalf("只应留下 fk_a: %+v", g.Edges)
	}
	if a, _ := g.Node("a"); a.Columns[1].FK {
		t.Error("被丢弃的外键不应把列标成 FK")
	}
}

func TestERUnsupported(t *testing.T) {
	manager := database.NewManager()
	cfg := &database.ConnectionConfig{ID: "c1", Name: "t", Type: "sqlite", Database: filepath.Join(t.TempDir(), "x.db")}
	if err := manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.CloseAll)
	svc := NewERService(manager, nil, nil)
	if _, err := svc.Graph(context.Background(), "nope", ""); model.CodeOf(err) != model.CodeConnNotFound {
		t.Fatalf("连接不存在: %v", err)
	}
	if CanER("starrocks") {
		t.Fatal("StarRocks 没有外键，不应支持 ER 图")
	}
	if !CanER("mysql") || !CanER("postgres") || !CanER("sqlite") {
		t.Fatal("MySQL、PostgreSQL、SQLite 应支持 ER 图")
	}
}

// 表结构索引落后于库（刚建了表）时，应强制重建一次再画。
func TestERGraphRefreshesStaleIndex(t *testing.T) {
	svc := newERTestEnv(t,
		`CREATE TABLE users (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`,
	)
	full := svc.schema
	stale := true
	svc.schema = func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
		sc, err := full(ctx, connID, dbName)
		if err == nil && stale {
			sc.Tables = sc.Tables[:1] // 假装索引里只有一张表
		}
		return sc, err
	}
	refreshed := 0
	svc.refresh = func(ctx context.Context, connID, dbName string) error {
		refreshed++
		stale = false
		return nil
	}
	g, err := svc.Graph(context.Background(), "c1", "")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed != 1 || len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Fatalf("refreshed=%d nodes=%d edges=%d", refreshed, len(g.Nodes), len(g.Edges))
	}
	// 索引是新的就不该再重建
	refreshed = 0
	if _, err := svc.Graph(context.Background(), "c1", ""); err != nil || refreshed != 0 {
		t.Fatalf("不该重建: refreshed=%d err=%v", refreshed, err)
	}
}
