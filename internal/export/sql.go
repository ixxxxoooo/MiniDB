package export

import (
	"fmt"
	"os"
	"strings"
)

// ToSQL 导出为 SQL INSERT 语句
func ToSQL(filePath, tableName string, columns []string, rows []map[string]interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	for _, row := range rows {
		var values []string
		for _, col := range columns {
			val := row[col]
			values = append(values, escapeValue(val))
		}

		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n",
			tableName,
			strings.Join(columns, ", "),
			strings.Join(values, ", "))

		if _, err := file.WriteString(sql); err != nil {
			return err
		}
	}

	return nil
}

func escapeValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case float64:
		return fmt.Sprintf("%v", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	default:
		return fmt.Sprintf("'%v'", v)
	}
}
