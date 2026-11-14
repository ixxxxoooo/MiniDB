# MiniDB 项目规则

Go + MyGo 原生 UI 的桌面数据库客户端。本文件只记「改代码时容易出错、又不能从代码直接看出来」的规则，细节在 `docs/` 里，不在这里重复。
与全局规则冲突时以本文件为准。

## 动手前先读

- `docs/ARCHITECTURE.md`：分层、依赖白名单、被测试锁定的约束、新增一种数据库的步骤。
- `docs/UI_GUIDE.md`：界面令牌、按钮、标签、表格的既定规则，新界面照着做，不另起一套。
- 找可复用代码：`grep` 现有实现和 `app` 门面。项目**没有**「开发进度.md」（已删除），不要新建，也不要按全局规则去读写它。

## 架构红线（`internal/archtest` 强制，违反会让 `go test` 失败）

- 依赖方向 `storage → database → schemaindex/ai/agent → app → ui`。`ui` 只能依赖 `app`、`model`、`sqltext`、`logger`、`version`、`locales`；要数据库能力，先在 `app` 门面加转调，**不放宽白名单**。
- 数据库差异只有两种写法：`Caps()` 里的一项能力，或方言的方法/可选接口。**不要**在方言文件之外写 `if dbType == X` 或用类型字符串比较。
- 单文件 ≤ 300 行。登记了上限的大文件（如 `ui/shell.go`）**一行都不能再加**：新逻辑放进新文件或已有的小文件，在原文件里最多改一行调用。
- 不许 `_ = store.Put(...)`：写失败要返回或记日志。

## 编码约定

- 错误：后端用 `model.Errf(code, ...)`，界面用 `errMessage` 翻译；用户可见文案都走 `locales/*.json`（改完跑 `go tool mygo generate` 和 `go tool mygo i18n check`，不要手改 `catalog_gen.go`）。已有代码里零散的中文字符串（如 `Note`）是历史遗留，新代码不要再加。
- 界面：颜色和尺寸走 `theme.go` 令牌，按钮只用 `actBtn` / `fieldBtn` / `footBtn` 三档，不在调用处单设高度字号。后台协程里不要捕获 `*ui.Context`，刷新用 `up.post` 或 `Invalidate`。
- 这是桌面应用，不是 Web 接口：全局规则里的「接口统一返回状态码 + 消息 + 数据」不适用，沿用 `model.Error` 错误码即可。
- 新增文件在主要类型或文件头注释里加 `@author ixxxxoooo`；方法写清用途、参数、返回值。不要批量给旧文件补。
- **凡是会向数据库发语句的路径，都要让语句出现在控制台日志里**，包括隐式发出的 `BEGIN` / `COMMIT` / `ROLLBACK`。用户把控制台当作「实际执行了什么」的唯一依据；漏记等于撒谎。入口是 `Shell.logSQL`，后台发出的语句经 `app` 总线事件（参考 `TxStmtEvent`）转进来。

## 测试

- 改行为先写会失败的测试，再修；修 bug 要**确认测试在没有修复时确实失败**，再确认修复后通过。
- 解析数据库输出的代码（执行计划等）用 `internal/database/testdata/` 里的**真实输出**做固定样例，不要手写想象中的格式；同一功能在不同服务器版本上输出可能不同，样例要标明来自哪个版本。
- 需要真实库的测试沿用现有约定：`INTEGRATION_TEST=1` + `MINIDB_IT_<库>_HOST/PORT/USER/PASSWORD`，CI 的服务容器会自动跑；不要另起环境变量。测试只在自己建的库里操作并清理，**不要连用户自己的库**，需要实例时起临时的。
- 界面用 `ui.Tester` 无窗口测试；要看实际画面用 `MINIDB_SHOTS=<目录> go test ./internal/ui/`，并真的打开图片看。

## 提交前

```bash
go vet ./... && go tool mygo vet .
go tool mygo i18n check
go test ./...
```

- 提交信息中文，`type: 概述`，type 取 `feat` / `fix` / `refactor` / `test` / `docs` / `chore`；一次提交只做一件事，必要时列要点说明原因。
- 用户可见的改动发版前要补进 `CHANGELOG.md`（顶部「未发布」小节，发版时改成版本号）；改了架构约定同步更新 `docs/ARCHITECTURE.md`。
- 不提交 `.mygo/`、`.specstory/`、`AGENTS.md`（都已在 `.gitignore`）。临时分析文档放 `.specstory/history/docs/`，正式文档放 `docs/`。
- 开发版运行：`go tool mygo dev`；打包：`go tool mygo build`。
