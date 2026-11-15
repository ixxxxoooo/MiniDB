package ui

import (
	"strings"
)

// @author ixxxxoooo
//
// 表「文档」页的数据：字段表的每一行、一句话说明，以及复制出去的 Markdown。都是纯函数，不碰界面。

// docRow 是字段表的一行。ai 为真表示说明是 AI 草拟的（还没写进数据库的备注）。
type docRow struct {
	name, typ, comment string
	ai                 bool
}

// firstParagraph 取一篇 Markdown 的第一段正文：跳过标题、代码围栏、表格、引用和空行，
// 把连续的几行拼成一句。参数：md 是文档全文。返回值：第一段；没有正文时返回空串。
func firstParagraph(md string) string {
	var out []string
	fenced := false
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			fenced = !fenced
			continue
		}
		skip := fenced || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, ">") || strings.HasPrefix(t, "---")
		switch {
		case t == "" && len(out) > 0:
			return strings.Join(out, " ")
		case t == "" || skip:
			if len(out) > 0 {
				return strings.Join(out, " ")
			}
		default:
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// mdCell 把一段文字放进 Markdown 表格单元格：竖线转义、换行拼成一行。
func mdCell(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

// docMarkdown 把文档页的内容写成 Markdown：标题、表备注、一句说明和字段表。
// 参数：table 表名；comment 表备注；desc 一句说明；rows 字段表的行。返回值：Markdown 文本。
func docMarkdown(table, comment, desc string, rows []docRow) string {
	var b strings.Builder
	b.WriteString("# " + table + "\n\n")
	if comment != "" {
		b.WriteString("> " + comment + "\n\n")
	}
	if desc != "" {
		b.WriteString(desc + "\n\n")
	}
	b.WriteString("| 字段 | 类型 | 说明 |\n| --- | --- | --- |\n")
	for _, r := range rows {
		b.WriteString("| `" + r.name + "` | " + strings.ToUpper(mdCell(r.typ)) + " | " + mdCell(r.comment) + " |\n")
	}
	return b.String()
}
