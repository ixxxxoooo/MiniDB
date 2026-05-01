package services

import (
	"fmt"
	"testing"

	"tableplus-ai/internal/ai"
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
