package export

import (
	"encoding/json"
	"fmt"
	"os"
)

// ToJSON 导出为 JSON 文件
func ToJSON(filePath string, rows []map[string]interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	return encoder.Encode(rows)
}
