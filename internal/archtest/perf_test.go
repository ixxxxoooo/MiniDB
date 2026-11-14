// @author ixxxxoooo

package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// 性能日志的约束（规则和解读见 CLAUDE.md「性能日志」、docs/PERFORMANCE.md）。

// untimedServiceMethods 登记不需要打点的服务方法，值是理由。
// 只收两类：纯内存、每帧都会调用的读取（打点的开销和统计噪声比它本身还大）；只设置字段或取消任务、不做 I/O 的方法。
// 其余的服务方法一律要带 logger.Perf，登记前先想想它是不是真的不会慢。
var untimedServiceMethods = map[string]string{
	"AIService.SetEnv": "只设置字段", "ConnectionService.SetEnv": "只设置字段", "DumpService.SetEnv": "只设置字段",
	"ExportService.SetEnv": "只设置字段", "ImportService.SetEnv": "只设置字段",
	"ConnectionService.IsConnected":           "纯内存读取",
	"ConnectionService.KeepaliveInterval":     "纯内存读取",
	"ConnectionService.SetKeepaliveInterval":  "只设置字段",
	"ConnectionService.StartKeepalive":        "只起后台协程，保活的每次探测在 db.ping 里计时",
	"ConnectionService.StopKeepalive":         "只通知后台协程退出",
	"QueryService.TransactionStatus":          "纯内存读取，界面每帧都调用",
	"QueryService.SetQueryTimeout":            "只设置字段",
	"QueryService.DefaultSelectTableSQL":      "只拼字符串",
	"QueryService.CancelQuery":                "只调用取消函数",
	"SchemaIndexService.GetSchemaIndexStatus": "纯内存读取",
	"MonitorService.Supported":                "纯内存读取",
	"AIService.CancelChatStream":              "只调用取消函数",
	"ExportService.CancelExport":              "只调用取消函数",
	"ImportService.CancelImport":              "只调用取消函数",
	"DumpService.Cancel":                      "只调用取消函数",
	"SettingsService.GetLogPath":              "纯内存读取",
	"SettingsService.GetAppInfo":              "纯内存读取",
}

// TestServiceMethodsAreTimed 要求 app 包里每个 *XxxService 的导出方法都调用 logger.Perf / PerfUI / PerfDone，
// 这样卡顿时 [PERF] 日志和汇总里一定有它。新增服务方法忘了打点，这里会直接失败。
func TestServiceMethodsAreTimed(t *testing.T) {
	seen := map[string]bool{}
	var missing []string
	goFiles(t, func(rel, src string) {
		if !strings.HasPrefix(rel, "app/") {
			return
		}
		f, err := parser.ParseFile(token.NewFileSet(), rel, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || !fd.Name.IsExported() {
				continue
			}
			star, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			id, ok := star.X.(*ast.Ident)
			if !ok || !strings.HasSuffix(id.Name, "Service") {
				continue
			}
			key := id.Name + "." + fd.Name.Name
			seen[key] = true
			if _, exempt := untimedServiceMethods[key]; exempt {
				continue
			}
			if !callsPerf(fd.Body) {
				missing = append(missing, rel+": "+key)
			}
		}
	})
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("%s 没有性能打点：在方法开头加 defer logger.Perf(\"app.<服务>.<方法>\", ...)()；确实不需要的登记到 untimedServiceMethods 并写明理由", m)
	}
	for key := range untimedServiceMethods {
		if !seen[key] {
			t.Errorf("untimedServiceMethods 里的 %s 已经不存在，请删掉这一行", key)
		}
	}
}

// callsPerf 报告函数体里有没有调用 logger.Perf、logger.PerfUI 或 logger.PerfDone。
func callsPerf(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "logger" && strings.HasPrefix(sel.Sel.Name, "Perf") {
				found = true
			}
		}
		return !found
	})
	return found
}

// uiGoroutineSites 登记 ui 包里允许直接写 go 语句的文件及个数，值是理由。
// 后台工作一律用 up.bg(work, done)：它按调用函数自动打点（bg.* 与 ui.done.*）；
// 要在 UI 线程上执行用 up.post。直接起协程会绕过打点，也容易在后台碰界面状态。
var uiGoroutineSites = map[string]struct {
	n   int
	why string
}{
	"ui/async.go":       {1, "up.bg 本身，里面已经打点"},
	"ui/session.go":     {1, "自动保存的定时器，只经 up.post 回到 UI 线程（自动打点）"},
	"ui/updates.go":     {1, "检查更新的定时器，只经 up.post 回到 UI 线程（自动打点）"},
	"ui/tooltip.go":     {1, "悬浮提示的到点重绘：只睡眠后 Invalidate，不做实事"},
	"ui/statewriter.go": {1, "状态落盘的串行写协程，每次写入已用 bg.stateWriter 打点"},
}

// TestUIGoroutinesAreRegistered 禁止 ui 包里出现没有登记的 go 语句。
func TestUIGoroutinesAreRegistered(t *testing.T) {
	got := map[string]int{}
	goFiles(t, func(rel, src string) {
		if !strings.HasPrefix(rel, "ui/") {
			return
		}
		f, err := parser.ParseFile(token.NewFileSet(), rel, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if _, ok := n.(*ast.GoStmt); ok {
				got[rel]++
			}
			return true
		})
	})
	for rel, n := range got {
		if site, ok := uiGoroutineSites[rel]; !ok || n > site.n {
			t.Errorf("%s 有 %d 处 go 语句，没有登记或超出登记的个数：后台工作请用 up.bg（自动打点），确实需要裸协程再登记到 uiGoroutineSites 并写明理由", rel, n)
		}
	}
	for rel := range uiGoroutineSites {
		if got[rel] == 0 {
			t.Errorf("uiGoroutineSites 里的 %s 已经没有 go 语句，请删掉这一行", rel)
		}
	}
}
