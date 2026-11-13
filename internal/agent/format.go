package agent

import (
	"fmt"
	"sort"
	"strings"
)

// 模型看到的查询结果上限：行数太少会让它基于残缺数据下结论，太多又会撑爆上下文。
const (
	ModelRowsDefault = 50
	ModelCellDefault = 200
)

// FormatForModel 把结构化结果派生为模型消费的紧凑文本。
// 失败以 "ERROR:" 开头（系统提示词约定了这个前缀），并附上下一步建议；
// 行被截断时明确说出"共多少行、看到了多少"，避免模型把局部当全部。
func (r *ToolResult) FormatForModel(maxRows, maxCell int) string {
	if r == nil {
		return "ERROR: 工具没有返回结果"
	}
	if !r.OK {
		text := "ERROR: " + r.Text
		if r.Hint != "" {
			text += "\n建议: " + r.Hint
		}
		return text
	}
	if r.ModelText != "" {
		return r.ModelText
	}
	if r.Kind != ResultKindRows {
		return r.Text
	}
	if len(r.Rows) == 0 {
		return "查询成功，0 行"
	}
	columns := r.Columns
	if len(columns) == 0 {
		for k := range r.Rows[0] {
			columns = append(columns, k)
		}
		sort.Strings(columns)
	}
	shown := len(r.Rows)
	if maxRows > 0 && shown > maxRows {
		shown = maxRows
	}
	var sb strings.Builder
	switch {
	case r.TotalRows > int64(shown):
		sb.WriteString(fmt.Sprintf("共 %d 行，仅展示前 %d 行（如需其余数据，请加 WHERE / 聚合 / LIMIT 缩小范围）\n\n", r.TotalRows, shown))
	default:
		sb.WriteString(fmt.Sprintf("共 %d 行\n\n", shown))
	}
	sb.WriteString("| " + strings.Join(columns, " | ") + " |\n")
	sb.WriteString("|" + strings.Repeat(" --- |", len(columns)) + "\n")
	for _, row := range r.Rows[:shown] {
		cells := make([]string, len(columns))
		for i, col := range columns {
			cells[i] = FormatCell(row[col], maxCell)
		}
		sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return strings.TrimSpace(sb.String())
}

// FormatCell 把单元格值转成表格里的一段文字：NULL 与空串区分、换行与竖线转义、按字符数截断。
func FormatCell(v any, maxCell int) string {
	if v == nil {
		return "NULL"
	}
	cell := strings.TrimSpace(fmt.Sprint(v))
	if cell == "" {
		return "''"
	}
	if runes := []rune(cell); maxCell > 0 && len(runes) > maxCell {
		cell = string(runes[:maxCell]) + "…"
	}
	cell = strings.ReplaceAll(cell, "\n", " ")
	return strings.ReplaceAll(cell, "|", "\\|")
}
