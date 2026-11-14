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

// uiTypeConstTables 登记 ui 里按数据库类型取图标、颜色、标记的展示表：dbicons.go、welcome.go。
// 它们的职责就是「类型 → 图形」，不是业务分支；其余文件一律不许直接引用 model.MySQL 这类常量。
var uiTypeConstTables = map[string]bool{"ui/dbicons.go": true, "ui/welcome.go": true}

// TestNoDBTypeBranchOutsideDatabase 禁止 database 包之外（app、ui、schemaindex、agent …）
// 通过 database.Postgres / model.Postgres 这类常量或 MySQLCompatible() 判断数据库类型：
// 差异要么是一项能力（Caps），要么是方言的方法，加一种数据库时不该去改调用方。
//
// 例外：
//   - database 包自己的约束见 TestDatabaseTypeConstsOnlyInDialectFiles；
//   - model 包定义这些常量；
//   - ui 里的图标、颜色展示表登记在 uiTypeConstTables；
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
				if pkg.Name == "model" && uiTypeConstTables[rel] {
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
	for rel := range uiTypeConstTables {
		if !used[rel] {
			t.Errorf("%s 已不再引用数据库类型常量，请从 uiTypeConstTables 里删除这一项", rel)
		}
	}
}

// TestDatabaseTypeConstsOnlyInDialectFiles 约束 database 包自己：类型常量（MySQL、Postgres …）只许出现在
// 方言文件（dialect_*.go）、语句写法文件（sqlgen_*.go）和常量定义（model_alias.go）里。
// 通用的构造函数要按类型做不同的事，就加一个可选接口（见 sqlgen.go）或一项 Caps，而不是 if dbType == X。
func TestDatabaseTypeConstsOnlyInDialectFiles(t *testing.T) {
	root := internalRoot(t)
	fset := token.NewFileSet()
	goFiles(t, func(rel, _ string) {
		if !strings.HasPrefix(rel, "database/") {
			return
		}
		name := strings.TrimPrefix(rel, "database/")
		if strings.HasPrefix(name, "dialect_") || strings.HasPrefix(name, "sqlgen_") || name == "model_alias.go" {
			return
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && dbTypeConsts[id.Name] && id.Obj == nil {
				t.Errorf("%s: 通用代码里直接引用了数据库类型常量 %s，请改用 Caps() 或方言的可选接口（sqlgen.go）", fset.Position(id.Pos()), id.Name)
			}
			return true
		})
	})
}
