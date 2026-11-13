package schemaindex

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"minidb/internal/ai"
)

func shopSchema() *ai.SchemaContext {
	col := func(name, typ, comment string) ai.ColumnSchema {
		return ai.ColumnSchema{Name: name, Type: typ, Comment: comment}
	}
	return &ai.SchemaContext{DatabaseType: "mysql", DatabaseName: "shop", Tables: []ai.TableSchema{
		{Name: "users", Comment: "用户表", Columns: []ai.ColumnSchema{col("id", "bigint", ""), col("email", "varchar(255)", "登录邮箱"), col("created_at", "datetime", "注册时间")}},
		{Name: "orders", Comment: "订单表", Columns: []ai.ColumnSchema{col("id", "bigint", ""), col("user_id", "bigint", "下单用户"), col("total_amount", "decimal(10,2)", "订单金额"), col("status", "tinyint", "订单状态")}},
		{Name: "order_items", Comment: "订单明细", Columns: []ai.ColumnSchema{col("order_id", "bigint", ""), col("sku", "varchar(64)", "商品编码"), col("qty", "int", "数量")}},
		{Name: "t_pay_log", Comment: "支付流水", Columns: []ai.ColumnSchema{col("trade_no", "varchar(64)", "交易号"), col("paidAt", "datetime", "支付时间")}},
		{Name: "categories", Comment: "商品分类", Columns: []ai.ColumnSchema{col("name", "varchar(64)", "分类名称")}},
	}}
}

func names(hits []TableHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Name
	}
	return out
}

func TestTokenize(t *testing.T) {
	cases := map[string][]string{
		"user_id":     {"user", "id"},
		"orderItems":  {"order", "item"},
		"t_pay_log":   {"t", "pay", "log"},
		"v2Order":     {"v", "2", "order"},
		"用户表":         {"用户", "户表"},
		"订单":          {"订单"},
		"Orders":      {"order"},
		"categories":  {"category"},
		"status":      {"status"},
		"这个库的用户":      {"这个", "个库", "用户"},
		"用户 email 邮箱": {"用户", "email", "邮箱"},
	}
	for in, want := range cases {
		if got := tokenize(in); !reflect.DeepEqual(got, want) {
			t.Errorf("tokenize(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSearchTablesFindsChineseCommentsAndIdentifiers(t *testing.T) {
	s := NewSearcher(shopSchema())
	cases := []struct {
		query []string
		first string
	}{
		{[]string{"用户"}, "users"},
		{[]string{"订单明细"}, "order_items"},
		{[]string{"支付"}, "t_pay_log"},
		{[]string{"order", "item"}, "order_items"},
		{[]string{"category"}, "categories"},
		{[]string{"订单金额"}, "orders"},
		{[]string{"邮箱"}, "users"},
		{[]string{"pay"}, "t_pay_log"},
	}
	for _, c := range cases {
		hits := s.Tables(c.query, 5)
		if len(hits) == 0 || hits[0].Name != c.first {
			t.Errorf("Tables(%v) = %v, want %s first", c.query, names(hits), c.first)
		}
	}
	if hits := s.Tables([]string{"orders"}, 5); len(hits) == 0 || hits[0].Name != "orders" {
		t.Errorf("an exact table name must beat tables that merely mention it: %v", names(hits))
	}
	if s.Tables([]string{"完全无关的词语"}, 5) != nil || s.Tables(nil, 5) != nil {
		t.Error("no match is nil, not an arbitrary table")
	}
}

func TestSearchColumns(t *testing.T) {
	s := NewSearcher(shopSchema())
	hits := s.Columns([]string{"金额"}, nil, 3)
	if len(hits) == 0 || hits[0].Table != "orders" || hits[0].Column != "total_amount" || hits[0].Comment != "订单金额" {
		t.Fatalf("金额 → %+v", hits)
	}
	if hits := s.Columns([]string{"paid_at"}, nil, 3); len(hits) == 0 || hits[0].Column != "paidAt" {
		t.Fatalf("snake_case query finds a camelCase column: %+v", hits)
	}
	if hits := s.Columns([]string{"id"}, []string{"ORDERS"}, 10); len(hits) == 0 || hits[0].Table != "orders" {
		t.Fatalf("scoping to a table (case-insensitive): %+v", hits)
	} else {
		for _, h := range hits {
			if h.Table != "orders" {
				t.Fatalf("out-of-scope hit %+v", h)
			}
		}
	}
}

func TestSuggestAndLookup(t *testing.T) {
	s := NewSearcher(shopSchema())
	if got := s.Suggest("ordrs", 3); len(got) == 0 || got[0] != "orders" {
		t.Errorf("typo: %v", got)
	}
	if got := s.Suggest("user", 3); len(got) == 0 || got[0] != "users" {
		t.Errorf("prefix: %v", got)
	}
	if got := s.Suggest("订单", 3); len(got) == 0 {
		t.Errorf("a Chinese word falls back to search: %v", got)
	}
	if tbl, ok := s.Table("  Users "); !ok || tbl.Name != "users" {
		t.Error("lookup ignores case and spaces")
	}
	if _, ok := s.Table("nope"); ok {
		t.Error("unknown table")
	}
}

func TestManagerIndexCachesSearcherUntilSchemaChanges(t *testing.T) {
	var version atomic.Value
	version.Store("v1")
	m := newTestManager(t, func(connID, dbName string) (*ai.SchemaContext, error) {
		return testSchema(version.Load().(string)), nil
	})
	ctx := context.Background()
	_, a, err := m.Index(ctx, "c", "db1")
	if err != nil {
		t.Fatal(err)
	}
	_, b, _ := m.Index(ctx, "c", "db1")
	if a != b {
		t.Fatal("same schema, same searcher")
	}
	testSchemaChanged := func(s *ai.SchemaContext) {
		s.Tables[0].Columns = append(s.Tables[0].Columns, ai.ColumnSchema{Name: "phone"})
	}
	m.buildSchema = func(connID, dbName string) (*ai.SchemaContext, error) {
		s := testSchema("v1")
		testSchemaChanged(s)
		return s, nil
	}
	if _, err := m.Refresh(ctx, "c", "db1", RefreshReasonManual); err != nil {
		t.Fatal(err)
	}
	_, c, _ := m.Index(ctx, "c", "db1")
	if c == a || len(c.Columns([]string{"phone"}, nil, 1)) != 1 {
		t.Fatal("a changed schema must produce a new searcher that sees the new column")
	}
}

func TestOldRecordIsServedThenRebuilt(t *testing.T) {
	var builds atomic.Int32
	m := newTestManager(t, func(connID, dbName string) (*ai.SchemaContext, error) {
		builds.Add(1)
		return testSchema("rebuilt"), nil
	})
	target, err := m.resolveTarget("c", "db1")
	if err != nil {
		t.Fatal(err)
	}
	old, _ := m.recordFromSchema(target, testSchema("old"), RefreshReasonManual)
	old.Version = 0 // 旧版本写下的记录：没有数据库索引信息
	if err := m.persistRecord(old); err != nil {
		t.Fatal(err)
	}
	schema, err := m.GetSchema(context.Background(), "c", "db1")
	if err != nil || schema.DatabaseVersion != "old" {
		t.Fatalf("the old record is usable immediately: %v %v", schema, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for builds.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if builds.Load() == 0 {
		t.Fatal("an old-format record must be rebuilt in the background")
	}
	st, _ := m.GetStatus("c", "db1")
	if !strings.HasPrefix(string(st.Source), "") || st.TableCount != 1 {
		t.Fatalf("status %+v", st)
	}
}
