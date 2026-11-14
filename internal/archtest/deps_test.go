// Package archtest 用测试把架构约定固化下来：包依赖方向、禁止的写法、文件体量。
// 这里没有生产代码，只有测试；违反约定时 go test 会直接失败，不用等人工审查。
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "minidb/"

// allowedDeps 是各内部包允许依赖的内部包白名单。
// 规则：只许收紧，不许放宽——新增依赖必须先在这里登记并说明理由。
// 依赖方向：storage/appdata → database → schemaindex/ai/agent → app → ui。
var allowedDeps = map[string][]string{
	"agent":       {"database", "logger"},
	"ai":          {"logger"},
	"app":         {"agent", "ai", "database", "dataimport", "export", "logger", "model", "schemaindex", "sqltext", "storage", "version"},
	"appdata":     {},
	"archtest":    {},
	"database":    {"appdata", "logger", "model", "sqltext", "sshtunnel"},
	"dataimport":  {},
	"export":      {"sqltext"},
	"keychain":    {"appdata", "storage"},
	"logger":      {"appdata"},
	"model":       {},
	"schemaindex": {"ai", "database", "logger", "storage"},
	"sqltext":     {},
	"sshtunnel":   {},
	"storage":     {"appdata"},
	// ui 不许碰 storage / keychain / sshtunnel：状态存取走 app.UIState，连接走 app.Connections。
	// 遗留：ui 仍直接引用 database 的函数与编辑类（CapsOf、QuoteIdent、Structure*Edit 等），
	// 以及 agent/ai/schemaindex/export/dataimport 的类型。后续把这些收进 model 或 app 门面后，再从这里删掉。
	"ui": {"agent", "ai", "app", "appdata", "database", "dataimport", "export", "logger",
		"model", "schemaindex", "sqltext", "version", "locales"},
	"version": {},
}

// internalRoot 返回 internal 目录的绝对路径（测试运行目录是 internal/archtest）。
func internalRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

// importsOf 解析一个包目录下所有非测试文件，返回它依赖的 minidb 内部包（相对路径，如 "database"）。
func importsOf(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !strings.HasPrefix(p, modulePath) {
				continue
			}
			p = strings.TrimPrefix(p, modulePath)
			p = strings.TrimPrefix(p, "internal/")
			out[p] = true
		}
	}
	return out
}

// TestPackageDependencies 断言每个内部包只依赖白名单里的包，并且包之间没有环。
func TestPackageDependencies(t *testing.T) {
	root := internalRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	graph := map[string][]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := e.Name()
		allowed, known := allowedDeps[pkg]
		if !known {
			t.Errorf("新增了内部包 %q：请在 archtest/deps_test.go 的 allowedDeps 里登记它允许依赖哪些包", pkg)
			continue
		}
		allow := map[string]bool{}
		for _, a := range allowed {
			allow[a] = true
		}
		for dep := range importsOf(t, filepath.Join(root, pkg)) {
			graph[pkg] = append(graph[pkg], dep)
			if !allow[dep] {
				t.Errorf("包 %s 不允许依赖 %s（违反分层约定；如确需依赖，先修改白名单并说明理由）", pkg, dep)
			}
		}
	}
	if cycle := findCycle(graph); cycle != "" {
		t.Errorf("包依赖存在环: %s", cycle)
	}
}

// findCycle 在依赖图里找环，找到返回形如 "a -> b -> a" 的描述，没有返回空串。
func findCycle(graph map[string][]string) string {
	const (
		white = iota
		gray
		black
	)
	color := map[string]int{}
	var stack []string
	var visit func(n string) string
	visit = func(n string) string {
		color[n] = gray
		stack = append(stack, n)
		deps := append([]string(nil), graph[n]...)
		sort.Strings(deps)
		for _, d := range deps {
			switch color[d] {
			case gray:
				i := 0
				for stack[i] != d {
					i++
				}
				return strings.Join(append(append([]string(nil), stack[i:]...), d), " -> ")
			case white:
				if c := visit(d); c != "" {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return ""
	}
	nodes := make([]string, 0, len(graph))
	for n := range graph {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if color[n] == white {
			if c := visit(n); c != "" {
				return c
			}
		}
	}
	return ""
}

// goFiles 遍历 internal 下的非测试 Go 文件，对每个文件调用 fn(相对 internal 的路径, 内容)。
func goFiles(t *testing.T, fn func(rel, src string)) {
	t.Helper()
	root := internalRoot(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fn(filepath.ToSlash(rel), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
