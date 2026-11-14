package database

import (
	"fmt"
	"strings"
)

// sqlserverSyntax 是 SQL Server 的查询写法：@pN 占位符，分页用 OFFSET ... FETCH（必须有 ORDER BY），取零行用 TOP 0。
type sqlserverSyntax struct{}

func (sqlserverDialect) Syntax() Syntax { return sqlserverSyntax{} }

func (sqlserverSyntax) Placeholder(n int) string { return fmt.Sprintf("@p%d", n) }

func (sqlserverSyntax) CastText(col string) string {
	return fmt.Sprintf("CAST(%s AS NVARCHAR(MAX))", col)
}

// AppendPage 在 SELECT 后追加 OFFSET/FETCH：没有最外层 ORDER BY 就补 ORDER BY (SELECT NULL)。
// limit 小于 1 时不追加（FETCH 要求至少 1 行），offset 小于 0 按 0 算。
func (sqlserverSyntax) AppendPage(sel string, limit, offset int) string {
	if limit < 1 {
		return sel
	}
	if offset < 0 {
		offset = 0
	}
	sel = strings.TrimRight(strings.TrimSpace(sel), ";")
	if topLevelIndex(sel, "ORDER BY", false) < 0 {
		sel += " ORDER BY (SELECT NULL)"
	}
	return fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", sel, offset, limit)
}

func (sqlserverSyntax) ZeroRows(from string) string { return "SELECT TOP 0 * FROM " + from }

// HasRowLimit 判断最外层是不是已经有 TOP 或 OFFSET/FETCH。
func (sqlserverSyntax) HasRowLimit(sel string) bool {
	return topLevelIndex(sel, "TOP", false) >= 0 || topLevelIndex(sel, "FETCH NEXT", false) >= 0 || topLevelIndex(sel, "FETCH FIRST", false) >= 0
}

// CountOf 数一条 SELECT 的行数：子查询里不允许 ORDER BY（除非带 TOP / OFFSET），所以先去掉最外层的 ORDER BY。
func (sqlserverSyntax) CountOf(sel string) string {
	sel = strings.TrimRight(strings.TrimSpace(sel), ";")
	if i := topLevelIndex(sel, "ORDER BY", true); i >= 0 {
		sel = strings.TrimSpace(sel[:i])
	}
	return fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __auto_count__", sel)
}

// MaxParams 是单条语句能带的参数个数（SQL Server 上限 2100）。
func (sqlserverSyntax) MaxParams() int { return 2000 }
