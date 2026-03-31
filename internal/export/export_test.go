package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempFile(t *testing.T, ext string) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test."+ext)
}

// TestToCSV 测试 CSV 导出
func TestToCSV(t *testing.T) {
	path := tempFile(t, "csv")
	columns := []string{"id", "name", "email"}
	rows := []map[string]interface{}{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": nil},
	}

	if err := ToCSV(path, columns, rows); err != nil {
		t.Fatalf("ToCSV 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "id,name,email") {
		t.Error("CSV 缺少表头")
	}
	if !strings.Contains(content, "Alice") {
		t.Error("CSV 缺少数据行")
	}
}

// TestToJSON 测试 JSON 导出
func TestToJSON(t *testing.T) {
	path := tempFile(t, "json")
	rows := []map[string]interface{}{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	if err := ToJSON(path, rows); err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("期望 2 条记录，得到 %d", len(result))
	}
}

// TestToSQL 测试 SQL INSERT 导出
func TestToSQL(t *testing.T) {
	path := tempFile(t, "sql")
	columns := []string{"id", "name"}
	rows := []map[string]interface{}{
		{"id": 1, "name": "Alice"},
	}

	if err := ToSQL(path, "users", columns, rows); err != nil {
		t.Fatalf("ToSQL 失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "INSERT INTO") {
		t.Error("SQL 导出缺少 INSERT INTO")
	}
	if !strings.Contains(content, "users") {
		t.Error("SQL 导出缺少表名")
	}
}
