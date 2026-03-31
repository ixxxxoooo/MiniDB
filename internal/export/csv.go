package export

import (
	"encoding/csv"
	"fmt"
	"os"
)

// ToCSV 导出为 CSV 文件
func ToCSV(filePath string, columns []string, rows []map[string]interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// UTF-8 BOM
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	if err := writer.Write(columns); err != nil {
		return err
	}

	// 写入数据
	for _, row := range rows {
		var record []string
		for _, col := range columns {
			val := row[col]
			if val == nil {
				record = append(record, "")
			} else {
				record = append(record, fmt.Sprintf("%v", val))
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
