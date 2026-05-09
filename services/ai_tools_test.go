package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"minidb/internal/ai"
)

func TestTableStatsPicksUpToConfiguredBatchLimit(t *testing.T) {
	var tables []ai.TableSchema
	var tableNames []string
	for i := 0; i < 26; i++ {
		name := fmt.Sprintf("table_%02d", i)
		tables = append(tables, ai.TableSchema{Name: name})
		tableNames = append(tableNames, name)
	}

	picked := pickTargetTablesWithArgs(
		&ai.SchemaContext{Tables: tables},
		tableNames,
		"",
		nil,
		tableStatsMaxTables,
	)

	if len(picked) != tableStatsMaxTables {
		t.Fatalf("expected %d picked tables, got %d", tableStatsMaxTables, len(picked))
	}
	if picked[0].Name != "table_00" || picked[len(picked)-1].Name != "table_19" {
		t.Fatalf("unexpected picked range: first=%s last=%s", picked[0].Name, picked[len(picked)-1].Name)
	}
}

func TestParseToolCallArgsSupportsNewToolFields(t *testing.T) {
	args := parseToolCallArgs(`{"table_name":"users","columns":["id","amount"],"limit":999}`)
	if args.TableName != "users" {
		t.Fatalf("unexpected table_name: %q", args.TableName)
	}
	if len(args.TableNames) != 1 || args.TableNames[0] != "users" {
		t.Fatalf("table_name should populate table_names fallback: %#v", args.TableNames)
	}
	if len(args.Columns) != 2 || args.Columns[0] != "id" || args.Columns[1] != "amount" {
		t.Fatalf("unexpected columns: %#v", args.Columns)
	}
	if got := clampInt(args.Limit, 20, 1, tableSampleMaxRows); got != tableSampleMaxRows {
		t.Fatalf("limit should clamp to %d, got %d", tableSampleMaxRows, got)
	}
}

func TestBuildAllToolDefinitionsIncludesNewTools(t *testing.T) {
	defs := BuildAllToolDefinitions()
	seen := map[string]bool{}
	for _, def := range defs {
		seen[def.Name] = true
	}
	for _, name := range []string{"column_fuzzy_match", "table_relationships", "table_sample", "table_profile", "sql_explain_plan"} {
		if !seen[name] {
			t.Fatalf("missing tool definition: %s", name)
		}
	}
}

func TestColumnFuzzyMatchFindsColumnComments(t *testing.T) {
	svc := &AIService{}
	schema := &ai.SchemaContext{Tables: []ai.TableSchema{
		{
			Name: "customers",
			Columns: []ai.ColumnSchema{
				{Name: "id", Type: "bigint"},
				{Name: "mobile", Type: "varchar(20)", Comment: "手机号"},
			},
		},
	}}

	result := svc.execToolColumnFuzzyMatch("", schema, aiToolCallArgs{
		Keywords: []string{"手机"},
		Limit:    10,
	}, time.Now())

	if !strings.Contains(result.ToolOutput, "customers.mobile") {
		t.Fatalf("expected mobile column match, got: %s", result.ToolOutput)
	}
}

func TestTableRelationshipsIncludesExplicitAndInferredRelations(t *testing.T) {
	svc := &AIService{}
	schema := &ai.SchemaContext{Tables: []ai.TableSchema{
		{
			Name: "customers",
			Columns: []ai.ColumnSchema{
				{Name: "id", Type: "bigint"},
			},
		},
		{
			Name: "orders",
			Columns: []ai.ColumnSchema{
				{Name: "id", Type: "bigint"},
				{Name: "customer_id", Type: "bigint", ForeignKey: "customers.id"},
			},
		},
		{
			Name: "payments",
			Columns: []ai.ColumnSchema{
				{Name: "id", Type: "bigint"},
				{Name: "order_id", Type: "bigint"},
			},
		},
	}}

	result := svc.execToolTableRelationships("", schema, nil, aiToolCallArgs{
		TableNames: []string{"customers", "orders"},
		Limit:      10,
	}, time.Now())

	if !strings.Contains(result.ToolOutput, "explicit: orders.customer_id -> customers.id") {
		t.Fatalf("expected explicit relationship, got: %s", result.ToolOutput)
	}
	if !strings.Contains(result.ToolOutput, "inferred reverse: payments.order_id -> orders.id") {
		t.Fatalf("expected inferred reverse relationship, got: %s", result.ToolOutput)
	}
}

func TestToolConcurrencyDefaultsAreBalanced(t *testing.T) {
	if tableStatsMaxConcurrency != 6 {
		t.Fatalf("table_stats concurrency should be 6, got %d", tableStatsMaxConcurrency)
	}
	if tableProfileMaxConcurrency != 5 {
		t.Fatalf("table_profile concurrency should be 5, got %d", tableProfileMaxConcurrency)
	}
	if tableStatsMaxConcurrency > tableStatsMaxTables {
		t.Fatalf("table_stats concurrency should not exceed batch size")
	}
	if tableProfileMaxConcurrency > tableProfileMaxColumns {
		t.Fatalf("table_profile concurrency should not exceed column batch size")
	}
}

func TestPickProfileColumnsHonorsRequestedLimit(t *testing.T) {
	table := ai.TableSchema{
		Name: "orders",
		Columns: []ai.ColumnSchema{
			{Name: "id"}, {Name: "amount"}, {Name: "status"},
		},
	}
	cols := pickProfileColumns(table, []string{"status", "missing", "amount"})
	if len(cols) != 2 || cols[0].Name != "status" || cols[1].Name != "amount" {
		t.Fatalf("unexpected picked columns: %#v", cols)
	}
}

func TestBuildExplainSQL(t *testing.T) {
	if got := buildExplainSQL("SELECT * FROM users;"); got != "EXPLAIN SELECT * FROM users" {
		t.Fatalf("unexpected explain SQL: %q", got)
	}
	if got := buildExplainSQL("EXPLAIN SELECT * FROM users"); got != "EXPLAIN SELECT * FROM users" {
		t.Fatalf("existing EXPLAIN should be preserved: %q", got)
	}
	if got := stripExplainPrefix("EXPLAIN UPDATE users SET name='x'"); got != "UPDATE users SET name='x'" {
		t.Fatalf("unexpected explain target: %q", got)
	}
}
