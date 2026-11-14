package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// dbTypeNames 是各数据库类型的标识字符串。
var dbTypeNames = map[string]bool{"mysql": true, "postgres": true, "sqlite": true, "tidb": true, "starrocks": true}

// TestNoDBTypeLiteralComparison 禁止在方言文件之外用字符串字面量和数据库类型比较
// （如 dbType == "postgres"）。应改用 database.Postgres 等常量，或者更好地用 Caps() 判断能力。
// 方言文件（database/dialect*.go）本身不受限制。
func TestNoDBTypeLiteralComparison(t *testing.T) {
	root := internalRoot(t)
	fset := token.NewFileSet()
	goFiles(t, func(rel, _ string) {
		// ui/assistant.go 比较的是 Markdown 代码块的语言标记，不是数据库类型。
		if strings.HasPrefix(rel, "database/dialect") || strings.HasPrefix(rel, "storage/") || rel == "ui/assistant.go" {
			return
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok || (be.Op != token.EQL && be.Op != token.NEQ) {
				return true
			}
			for _, side := range []ast.Expr{be.X, be.Y} {
				lit, ok := side.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if v, _ := strconv.Unquote(lit.Value); dbTypeNames[v] {
					t.Errorf("%s: 用字符串字面量 %s 比较数据库类型，请用 database.XXX 常量或 Caps()", fset.Position(lit.Pos()), lit.Value)
				}
			}
			return true
		})
	})
}
