package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// maxFileLines 是单个源文件的行数上限；超过它的历史大文件登记在 fileLineCeilings 里。
const maxFileLines = 300

// fileLineCeilings 记录已经超过 maxFileLines 的文件各自的行数上限，只许下调、不许上调：
// 文件被拆小后请把这里的数字改小（降到 300 以内就删掉这一行）；新文件不得超过 maxFileLines。
var fileLineCeilings = map[string]int{
	"sqltext/format.go":           915,
	"database/query.go":           898,
	"app/dump_service.go":         617,
	"ui/workbench.go":             593,
	"ui/chart.go":                 589,
	"ui/users.go":                 570,
	"database/structure_alter.go": 567,
	"ui/grid.go":                  542,
	"app/export_service.go":       535,
	"app/query_service.go":        534,
	"dataimport/reader.go":        524,
	"ui/shell.go":                 519,
	"app/connection_service.go":   516,
	"ui/sidebar.go":               510,
	"ui/workspace.go":             509,
	"ui/tableedit.go":             498,
	"ui/structview.go":            483,
	"ui/inlineai.go":              478,
	"ui/settings.go":              462,
	"ui/keymap.go":                456,
	"ui/tablebar.go":              422,
	"ui/welcome.go":               417,
	"database/manager.go":         415,
	"export/writer.go":            406,
	"ui/tableview.go":             356,
	"ui/query.go":                 348,
	"database/import.go":          347,
	"ui/gridkeys.go":              345,
	"ui/session.go":               313,
	"ui/exportview.go":            308,
	"database/dialect_sqlite.go":  305,
	"ui/editoropts.go":            304,
}

// TestFileSizeCeilings 防止大文件继续变大、新文件一开始就写得过大。
func TestFileSizeCeilings(t *testing.T) {
	seen := map[string]bool{}
	goFiles(t, func(rel, src string) {
		if strings.HasSuffix(rel, "_gen.go") {
			return
		}
		lines := strings.Count(src, "\n")
		ceiling, listed := fileLineCeilings[rel]
		seen[rel] = true
		switch {
		case !listed && lines > maxFileLines:
			t.Errorf("%s 有 %d 行，超过 %d 行上限；请拆分（不要往白名单里加新文件）", rel, lines, maxFileLines)
		case listed && lines > ceiling:
			t.Errorf("%s 有 %d 行，超过白名单里的 %d 行；大文件只许变小", rel, lines, ceiling)
		case listed && lines < ceiling:
			t.Errorf("%s 已缩小到 %d 行，请把 fileLineCeilings 里的上限从 %d 下调（≤%d 行则删除这一项）", rel, lines, ceiling, maxFileLines)
		}
	})
	for rel := range fileLineCeilings {
		if !seen[rel] {
			t.Errorf("fileLineCeilings 里的 %s 已不存在，请删除这一项", rel)
		}
	}
}

// TestNoDiscardedStoreWrites 禁止把 Put/Delete 的返回值直接丢给空白标识符（_ = x.Put(...)）：
// 写失败要么往上返回，要么至少记一条日志。
func TestNoDiscardedStoreWrites(t *testing.T) {
	root := internalRoot(t)
	fset := token.NewFileSet()
	goFiles(t, func(rel, _ string) {
		f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			if id, ok := as.Lhs[0].(*ast.Ident); !ok || id.Name != "_" {
				return true
			}
			call, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Put" || sel.Sel.Name == "Delete") {
				t.Errorf("%s: 不要丢弃 %s 的错误，请返回或记日志", fset.Position(as.Pos()), sel.Sel.Name)
			}
			return true
		})
	})
}
