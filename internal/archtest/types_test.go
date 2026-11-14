// @author ixxxxoooo

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// dbTypeConsts 是各数据库类型的常量名（database.MySQL、model.Postgres …）。
var dbTypeConsts = map[string]bool{
	"MySQL": true, "MariaDB": true, "TiDB": true, "StarRocks": true,
	"Postgres": true, "SQLServer": true, "ClickHouse": true, "SQLite": true,
}

// typeBranchBanned 是 database 包之外不许调用的「按类型分支」入口：问能力用 Caps()，问写法用 Syntax / 方言方法。
var typeBranchBanned = map[string]bool{"MySQLCompatible": true, "IsMySQLCompatible": true}

// uiTypeConstDebt 登记 ui 里仍直接引用 model.MySQL 这类常量的文件，只许减少、不许新增。
// 其中 conn_type.go、dbicons.go、welcome.go 是「类型 → 图标/名称」的展示表，conn_dialog.go、conn_list.go、conn_views.go
// 里是新建连接时的默认类型，按类型取值本来就是它们的职责；其余是真正的历史债务，应改成 Caps 或方言方法：
// monitor_overview.go（PostgreSQL 的「事务数」标签）、tablenew.go（SQLite 的自增列类型）、
// users_editor.go（PostgreSQL 的默认 schema 与登录权限）、mariadb_hint.go（MySQL 连接到 MariaDB 的提示）。
var uiTypeConstDebt = map[string]bool{
	"ui/conn_type.go": true, "ui/dbicons.go": true, "ui/welcome.go": true,
	"ui/conn_dialog.go": true, "ui/conn_list.go": true, "ui/conn_views.go": true,
	"ui/monitor_overview.go": true, "ui/tablenew.go": true, "ui/users_editor.go": true, "ui/mariadb_hint.go": true,
}

// TestNoDBTypeBranchOutsideDatabase 禁止 database 包之外（app、ui、schemaindex、agent …）
// 通过 database.Postgres / model.Postgres 这类常量或 MySQLCompatible() 判断数据库类型：
// 差异要么是一项能力（Caps），要么是方言的方法，加一种数据库时不该去改调用方。
//
// 例外：
//   - database 包自身的 SQL 构造函数（BuildXxxSQL）仍按类型分支，是登记在「开发进度.md」里的历史债务，逐步迁入方言；
//   - model 包定义这些常量；
//   - ui 里的历史引用登记在 uiTypeConstDebt，只许减少；
//   - 测试文件不受限制（goFiles 本来就不读 _test.go）。
func TestNoDBTypeBranchOutsideDatabase(t *testing.T) {
	root := internalRoot(t)
	fset := token.NewFileSet()
	used := map[string]bool{}
	goFiles(t, func(rel, _ string) {
		if strings.HasPrefix(rel, "database/") || strings.HasPrefix(rel, "model/") || strings.HasPrefix(rel, "archtest/") {
			return
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && (pkg.Name == "database" || pkg.Name == "model") && dbTypeConsts[sel.Sel.Name] {
				if pkg.Name == "model" && uiTypeConstDebt[rel] {
					used[rel] = true
				} else {
					t.Errorf("%s: 直接引用了数据库类型常量 %s.%s，请改用 Caps() 或方言的方法", fset.Position(sel.Pos()), pkg.Name, sel.Sel.Name)
				}
			}
			if typeBranchBanned[sel.Sel.Name] {
				t.Errorf("%s: 调用了 %s，请改用 Caps() 或方言的方法", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	})
	for rel := range uiTypeConstDebt {
		if !used[rel] {
			t.Errorf("%s 已不再引用数据库类型常量，请从 uiTypeConstDebt 里删除这一项", rel)
		}
	}
}
