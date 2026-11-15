package ui

import (
	"strings"
)

// @author ixxxxoooo
//
// 「用户与权限」页的授权矩阵（设计稿 users 页）：把 SHOW GRANTS（MySQL）或按表合成的 GRANT（PostgreSQL）
// 的文字读成「对象 × 权限」的表。矩阵只读：要改权限走「编辑」里的 SQL 预览确认流程，不在矩阵上直接勾选。

// grantRow 是矩阵里的一行：一个授权对象（库、表等）和它上面的权限。
type grantRow struct {
	object string          // 显示的对象：*.*、shop.*、shop.orders
	privs  map[string]bool // 授予的权限（大写）；ALL PRIVILEGES 会展开成矩阵里的每一列
	extra  []string        // 不在矩阵列里的权限（PROCESS、PROXY 等），按出现顺序
	option bool            // 带 WITH GRANT OPTION
}

// mysqlGrantCols、pgGrantCols 是矩阵的列：库表里最常看的几项。
var (
	mysqlGrantCols = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX"}
	pgGrantCols    = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"}
)

// grantColumns 返回矩阵的列：角色制的库（PostgreSQL）和账号制的库（MySQL）权限名不一样。
func grantColumns(roleBased bool) []string {
	if roleBased {
		return pgGrantCols
	}
	return mysqlGrantCols
}

// splitTop 按逗号切开，但括号里的逗号不算（SELECT (id, name) 是一项）。
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// parseGrants 把一组 GRANT 语句读成矩阵的行：同一个对象的多条合并，顺序按首次出现；
// 不带 ON 的（角色授予）和只有 USAGE 的跳过。cols 是矩阵的列，用来展开 ALL PRIVILEGES。
func parseGrants(lines, cols []string) []grantRow {
	var rows []grantRow
	idx := map[string]int{}
	for _, line := range lines {
		up := strings.ToUpper(line)
		on := strings.Index(up, " ON ")
		if !strings.HasPrefix(strings.TrimSpace(up), "GRANT ") || on < 0 {
			continue
		}
		to := strings.Index(up[on:], " TO ")
		if to < 0 {
			continue
		}
		privText := strings.TrimSpace(line[len("GRANT "):on])
		obj := strings.TrimSpace(line[on+4 : on+to])
		for _, kind := range []string{"TABLE ", "SEQUENCE ", "SCHEMA ", "DATABASE "} {
			if strings.HasPrefix(strings.ToUpper(obj), kind) {
				obj = strings.TrimSpace(obj[len(kind):])
			}
		}
		obj = strings.NewReplacer("`", "", `"`, "").Replace(obj)
		i, ok := idx[obj]
		if !ok {
			i = len(rows)
			idx[obj] = i
			rows = append(rows, grantRow{object: obj, privs: map[string]bool{}})
		}
		r := &rows[i]
		r.option = r.option || strings.Contains(up, "WITH GRANT OPTION")
		for _, part := range splitTop(privText) {
			name := strings.ToUpper(strings.TrimSpace(part))
			if j := strings.Index(name, "("); j >= 0 { // 列级权限：按这一类权限算
				name = strings.TrimSpace(name[:j])
			}
			switch {
			case name == "" || name == "USAGE":
			case name == "ALL" || name == "ALL PRIVILEGES":
				for _, c := range cols {
					r.privs[c] = true
				}
			case inList(cols, name):
				r.privs[name] = true
			default:
				if !inList(r.extra, name) {
					r.extra = append(r.extra, name)
				}
			}
		}
	}
	// 只剩 USAGE 的对象（没有任何权限）不显示
	out := rows[:0]
	for _, r := range rows {
		if len(r.privs) > 0 || len(r.extra) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// inList 报告 list 里有没有 s。
func inList(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
