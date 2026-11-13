# MiniDB → MyGo 重构方案

> 日期：2026-11-13　|　源项目：`~/minidb`（Wails v3 alpha.78 + React，约 4.7 万行）　|　目标：`~/MiniDB-mygo`（MyGo v0.9.0）
> 状态：已确认（2026-11-13），按第 9 节的阶段顺序实施。
>
> **已确认的决定（与正文冲突时以此为准）**
> 1. 走原生 UI 路线；阶段 0 不通过时退回 webview。
> 2. **不做"一个连接一个窗口"**，保留现有的单窗口 + 工作区栏（WorkspaceBar）。正文 3.2 第 1 条作废，4.2 的 `Workspace` 改为同一窗口内每个连接一份。
> 3. Mermaid 预览和使用统计先不做。
> 4. 数据目录迁到系统标准位置，旧 `~/.minidb` 保留。
> 5. 无签名证书，用 ad-hoc 签名。**只考虑 macOS**，Windows / Linux 的打包、CI 和验证暂不做。
> 6. 代码放在现有仓库 `ixxxxoooo/MiniDB` 的新分支 `mygo`。

---

## 1. 结论先行

**用 MyGo 的原生 UI（纯 Go、GPU 绘制、无 webview）重写界面层，领域层 Go 代码原样迁移，Wails 绑定层和整个前端工程删除。**

| | 现状（Wails + React） | 重构后（MyGo 原生 UI） |
|---|---|---|
| 语言 | Go + TypeScript，两套类型靠绑定生成同步 | 只有 Go |
| 构建依赖 | Go、Node、pnpm、Wails CLI、CGO（Windows 需 MinGW） | 只有 Go 1.27，无 cgo，任意机器交叉编译全平台 |
| 包体 / 内存 | 二进制 37 MB | MyGo 官方数据：约 7 MB / 约 44 MB（待阶段 0 实测） |
| 平台 | macOS、Windows amd64 | macOS、Windows、Linux（amd64 + arm64） |
| 前后端通信 | 10 个绑定服务、约 100 个方法、字符串事件 | 无 IPC，视图直接调用 Go |
| 自动更新 | 自研 1500 行，仅 SHA-256 校验 | `mygo.Updater`，Ed25519 签名 + 增量更新 |
| UI 测试 | 仅少量纯函数的 vitest | `ui.Tester` 无窗口跑视图，`go test` 覆盖交互 |

### 为什么不是"换壳保留 React"

MyGo 也支持 webview 前端，把 Wails 绑定换成 MyGo 绑定就能跑，工作量最小。但那样只换了运行时，拿不到 MyGo 的主要收益，而且 MiniDB 的重型控件在 MyGo 原生 UI 里都有对应物：

| MiniDB 现有 | MyGo 原生对应 | 核实情况 |
|---|---|---|
| DataGrid（TanStack Table + Virtual，1242 行） | `ui.Table`：只构建可见行、单元格块选、列宽/列序/固定列、多列排序、表头菜单、单元格菜单、就地编辑 | 已读文档 `docs/ui/table.md` |
| Monaco SQL 编辑器 | `ui/editor`：内置 SQL 语法、多光标、折叠、查找替换、补全/悬停/诊断回调、AI 幽灵文本 `SetSuggestion` | 已读文档，源码确认含 SQL 语言 |
| react-markdown + 流式 | `ui/markdown`：GFM 表格、流式回复、可自定义代码块渲染 | 已读文档 |
| 侧边栏树、Tab、分栏、命令面板、Toast、对话框 | `ui.Tree` / `Outline` / `Tabs` / `Split` / `Dialog` / `Toast` / `Popover` | 已读控件清单 |
| 结构变更预览 | `ui/editor` 的 diff viewer | 已读文档 |

### 这个选择的风险（如实说明）

- **MyGo 是 0.9.0，未到 1.0**，主干提交非常频繁，API 仍可能变。不过现状用的 Wails v3 也是 alpha，风险同级。
- **我还没有实际跑过 MyGo**。上表是读文档和源码得到的，不是实测。大结果集滚动、中文输入法、宽表横向滚动这三项必须在阶段 0 实测。
- 应对：阶段 0 设 go/no-go 关卡（第 9 节）；架构上让界面层不承载业务逻辑（第 4 节），万一原生 UI 不达标，可退回 webview 前端而领域层和应用层不用重写。

---

## 2. 源项目盘点

### 2.1 原样迁移（约 1.4 万行 Go，含测试）

| 包 | 行数（含测试） | 处理 |
|---|---|---|
| `internal/database` | 约 5800 | 原样迁移，仅换 SQLite 驱动（见 5.1） |
| `internal/agent` | 约 1350 | 原样迁移 |
| `internal/ai` | 约 1850 | 原样迁移，删单次调用的旧入口（见 3.3） |
| `internal/schemaindex` | 约 820 | 迁移，去掉 `SetWailsApplication` |
| `internal/sshtunnel` | 约 540 | 原样迁移 |
| `internal/dataimport` | 约 570 | 原样迁移 |
| `internal/export` | 约 220 | 原样迁移 |
| `services/ai_tools*.go` | 约 1870 | 迁入应用层 |

### 2.2 重写

| 部分 | 现状 | 重构后 |
|---|---|---|
| `services/*`（约 5000 行） | Wails 绑定服务，返回 `map[string]interface{}`、`(bool, string)` 等为 JS 妥协的签名 | 应用层 `internal/app`，Go 惯用签名（`context.Context` + `error`） |
| `frontend/src`（约 2.3 万行 TS/TSX） | React + Zustand + Tailwind | `internal/ui/*` Go 视图 |
| `internal/storage`（BoltDB + 自管密钥文件） | 见 5.2 | SQLite + 系统钥匙串 |

### 2.3 删除

`internal/updater`（1500 行）、`internal/app`（Wails 生命周期）、`frontend/bindings`、`build/`、`Taskfile.yml`、`scripts/`、`project.env`、`wails.json`、`ClipboardService`、`WindowControls.tsx`、`TitleTooltipBridge.tsx`、`lib/wails/*`。

---

## 3. 功能取舍

### 3.1 保留并迁移

连接管理（标签、颜色、测试、SSH 隧道、只读模式、加密备份/导入）、库表浏览、数据网格（分页、排序、结构化筛选、原始 SQL 筛选、任意列搜索）、行编辑（批量提交/回滚）、SQL 编辑器（执行当前/选中/全部、多结果集、格式化、历史/收藏）、表结构与索引编辑、DDL 查看、导出（CSV/JSON/SQL 流式、可取消）、导入、进程列表、AI 助手（流式、工具时间线、会话持久化、护栏）、表文档、命令面板、中英双语、明暗主题、自动更新。

### 3.2 改进（交互与 UI）

1. **一个连接一个窗口**，取代窗口内的 WorkspaceBar。启动显示连接管理窗口；打开连接即开新窗口，连接颜色体现在标题栏。这是 TablePlus / DataGrip 的做法，好处是窗口级状态隔离、可并排对比两个库、系统级窗口切换直接可用，也消除了"断开当前连接后切到哪个工作区"这类历史 bug 的根源。每个窗口用 `StateKey` 记住位置大小。
2. **原生标题栏与菜单栏**。用 `TitleBarHidden` 保留系统红绿灯/标题按钮，不再自绘窗口控制按钮；所有快捷键注册为菜单项加速键，用户能在菜单里发现它们。
3. **写操作先预览 SQL**。数据编辑和结构变更提交前，弹出将要执行的 SQL（结构变更用 diff viewer 显示 DDL 前后对比），确认后才执行。标记为生产环境的连接强制二次确认。
4. **数据网格用电子表格式块选**。拖选单元格块，复制为 TSV / CSV / JSON / SQL INSERT / Markdown；NULL 与空字符串视觉区分；行号列固定；列宽列序按表持久化。
5. **行详情改为右侧检查器面板**，取代 Space 弹窗。JSON 字段在面板内用代码编辑器展示，替代 `JSONPreviewDialog`。
6. **SQL 编辑器补全接入 schema 索引**：表名、列名、别名解析；执行出错时把错误位置标成波浪线诊断；AI 补全走幽灵文本，Tab 接受。
7. **查询全链路可取消**。所有执行入口带 `context.Context`，工具栏显示运行中状态和取消按钮（现状只有批量执行可取消）。
8. **设置改为独立原生窗口**（Cmd+,），不再是主窗口内的模态框。
9. **AI 写操作走人工确认**。现状只允许只读 SQL；改为 Agent 可提议写语句，但必须经过第 3 条的预览确认后由用户执行。

### 3.3 砍掉或降级

| 功能 | 处理 | 理由 |
|---|---|---|
| Mermaid 图预览 | 首版不做，代码块按源码显示 | 原生 UI 无 Mermaid 渲染器；后续可用 `ui.WebView` 嵌入或自绘 ER 图 |
| Tiptap 富文本表文档编辑器 | 改为 Markdown 源码编辑 + 实时预览分栏 | 数据库工具用户以写 Markdown 为主，少一个重依赖 |
| AI 单次调用入口（`NaturalLanguageToSQL` / `ExplainSQL` / `AnalyzeData` / `DiagnoseError` / `ChatAI`） | 合并进 Agent，作为预置提示词 | 与 Agent 重复，是旧架构残留 |
| 行级接口（`InsertRow` / `UpdateRow` / `DeleteRow` / `BatchUpdateRows`） | 只留 `CommitTableDataChanges` | 单一事务提交路径 |
| 非流式导出（`ExportCSV` / `ExportJSON` / `ExportSQL`） | 只留流式 | 重复 |
| 使用统计（analytics） | 移除 | 本地优先的工具默认不应上报；需要的话再加 |
| 应用内日志查看器 | 改为"在访达中显示日志"加一个只读查看窗口 | 低频功能 |

### 3.4 新增（低成本）

Linux 支持、arm64 Windows、增量更新、窗口位置记忆、单实例锁、`.sql` / `.sqlite` 文件关联（双击打开）。

---

## 4. 目标架构

```
MiniDB-mygo/
├── main.go                  # 单实例锁、日志、装配、mygo.App.Run
├── mygo.json                # 应用名、标识、版本、更新公钥、各平台打包配置
├── go.mod                   # go 1.27；tool github.com/egoist/mygo/cmd/mygo
├── locales/                 # zh-Hans.json、en.json（ICU MessageFormat）
├── resources/icon.png
├── internal/
│   ├── database/  agent/  ai/  schemaindex/  sshtunnel/  dataimport/  export/   # 领域层：不依赖 mygo
│   ├── store/               # SQLite 持久化 + 钥匙串密钥 + BoltDB 一次性迁移
│   ├── app/                 # 应用层：用例编排，不依赖 ui 包
│   │   ├── connections.go   query.go   tabledata.go   structure.go
│   │   ├── exporter.go      importer.go   assistant.go   settings.go
│   │   └── tasks.go         # 后台任务：进度、取消
│   └── ui/                  # 界面层：只读写状态、调用 app，不做 I/O
│       ├── theme/           # 颜色、间距、字体令牌
│       ├── welcome/         # 连接管理窗口
│       ├── workspace/       # 连接窗口：shell、sidebar、tabs、statusbar
│       ├── grid/            # 数据网格 + 筛选栏 + 检查器
│       ├── sqledit/         # SQL 编辑器 + 结果区 + 历史
│       ├── structure/       # 列/索引编辑 + DDL diff
│       ├── assistant/       # AI 面板
│       ├── palette/         # 命令面板
│       └── settings/        # 设置窗口
└── .github/workflows/       # ci.yml、release.yml
```

### 4.1 三条硬规则

1. **依赖单向**：`ui → app → 领域层`。领域层和应用层不 import `mygo/ui`。这保证了应用层可以纯 `go test`，也是退回 webview 方案时的隔离带。
2. **视图函数不做 I/O**。MyGo 每帧重新执行视图函数；数据库查询、AI 请求、文件读写全部在 goroutine 里做，结果通过 `Window.Update` 回到 UI 线程写入状态。
3. **长集合只用 `List` / `Table`**。MyGo 文档实测：`Scroll` 里放 3000 行每帧 11 ms，`List` 里 0.2 ms。侧边栏表列表、结果集、历史记录都走虚拟化控件。

### 4.2 状态模型

```go
// 每个连接窗口一份，仅 UI 线程读写
type Workspace struct {
    conn    *app.Connection
    sidebar SidebarState          // 库/表树、搜索词
    tabs    []Tab                 // 接口：TableTab / QueryTab / StructureTab / DocTab
    active  int
    ai      AssistantState
    tasks   []*app.Task           // 导出、导入、长查询
}
```

Tab 状态（打开的表、未执行的 SQL 草稿）持久化到 SQLite，重启后恢复，取代现在的 Zustand + localStorage。

### 4.3 异步与流式

Wails 的字符串事件（`ai:chat_stream`、导出进度等）全部消失，改为：

- 应用层方法接收 `context.Context` 和一个类型化回调或 channel；
- UI 层在 goroutine 里消费，每条用 `win.Update(func(){ ... })` 写状态；
- `internal/agent/events.go` 现有的事件类型直接复用，不再需要序列化成 JSON 再在前端重建时间线（`streamSteps.ts` 412 行可删）。

---

## 5. 关键技术决策

### 5.1 SQLite 驱动：`mattn/go-sqlite3` → `modernc.org/sqlite`

现状 Windows 构建因 cgo 需要 MinGW。换成纯 Go 的 `modernc.org/sqlite`（同样是 `database/sql` 驱动，改动限于驱动注册名和少量错误类型判断），整个项目即可 `CGO_ENABLED=0`，在一台 Mac 上出全平台包。MyGo 自带的 sqlite 插件不是 `database/sql` 驱动，不适合用来连用户的数据库。

### 5.2 本地存储：BoltDB → SQLite，密钥进系统钥匙串

- BoltDB 是 KV，查询历史搜索、AI 会话列表都要全量扫描。换 SQLite 后历史可按连接/时间/关键字查，也能存 Tab 恢复状态和窗口状态。应用自身的库同样用 `modernc.org/sqlite`，保持单一驱动。
- 主密钥现在是明文文件 `~/.minidb/secret.key`，与密文同目录，等于没有防护。改用 `mygo.Secrets` 存入系统钥匙串（macOS Keychain / Windows Credential Manager / Linux Secret Service）；无可用钥匙串时回退到文件并在设置里提示。
- 数据目录从 `~/.minidb` 迁到系统标准位置（`mygo.PathUserData`）。**首次启动自动迁移**：读旧 BoltDB 和密钥，写入新库，旧目录保留不删。迁移代码带测试。

### 5.3 自动更新：自研 → `mygo.Updater`

`mygo keygen` 生成 Ed25519 密钥对，公钥写入 `mygo.json`，私钥进 CI secret；发布走 GitHub Releases（仓库 `ixxxxoooo/MiniDB`）。界面用官方 updater 插件的原生窗口（发布说明、跳过此版本、稍后提醒）。删掉 `internal/updater`、`scripts/release`、`update.json` 生成逻辑。

注意：已安装的 Wails 版无法自动升级到 MyGo 版（清单格式和签名机制不同）。当前版本号是 0.0.1，影响面小；在 README 和 Release 说明里要求手动下载一次即可。

### 5.4 国际化

`frontend/src/i18n` 的约 640 个键转成 `locales/zh-Hans.json` 和 `locales/en.json`，`mygo generate` 生成类型化的 Go 函数，`mygo i18n check` 在 CI 里检查缺失键。源语言设为 `zh-Hans`。

### 5.5 版本与兼容

- Go 1.27（本机已是 1.27.3）。
- MyGo 固定在 `v0.9.0` 标签，不跟主干；升级单独提交、单独验证。
- macOS 最低版本从 10.15 提到 12.0（MyGo 默认值）。

---

## 6. 界面布局

```
┌──────────────────────────────────────────────────────────────────────┐
│ ●●●  [连接色条] prod-mysql · orders ▾     ⌘K 搜索…        ▶ 运行  AI │  隐藏式标题栏
├───────────────┬──────────────────────────────────────┬───────────────┤
│ 🔍 过滤表      │ users × │ orders × │ Query 1 × │  +   │ 检查器 / AI    │
│ ▾ 表 (128)    ├──────────────────────────────────────┤ （可折叠）     │
│    users      │ 筛选栏：[列 ▾][= ▾][值    ] + │ WHERE… │               │
│    orders     ├──────────────────────────────────────┤  行详情        │
│    …          │ #  │ id │ name    │ created_at │ …   │  或            │
│ ▸ 视图 (4)    │ 1  │ 1  │ alice   │ 2026-…     │     │  AI 对话       │
│               │ 2  │ 2  │ NULL    │ 2026-…     │     │               │
│               │ …（Table，虚拟化）                    │               │
├───────────────┴──────────────────────────────────────┴───────────────┤
│ 2 处未提交修改 [预览 SQL] [丢弃]      1–200 / 12,480 行 · 38 ms · 只读 │  状态栏
└──────────────────────────────────────────────────────────────────────┘
```

- 三栏用 `ui.Split`，宽度持久化；右栏在"检查器"与"AI"之间切换。
- 表 Tab 内部用分段控件切换 数据 / 结构 / DDL / 文档，不再为同一张表开多个 Tab。
- 未提交修改集中在状态栏显示，这是唯一的提交入口。

---

## 7. 测试策略

| 层 | 方式 |
|---|---|
| 领域层 | 现有约 40 个 `_test.go` 原样迁移，必须全绿 |
| 存储迁移 | 用旧 BoltDB 固件文件做迁移测试 |
| 应用层 | 新写，对 SQLite 内存库跑用例 |
| 界面层 | `ui.NewTester` 跑关键流程：打开表 → 改单元格 → 预览 SQL → 提交；写 SQL → 运行 → 取消；连接表单校验 |
| 集成 | `internal/database/integration_test.go` 现有的 MySQL / PostgreSQL 集成测试保留，CI 用 service 容器 |
| 静态检查 | `go vet`、`mygo vet`（检查 UI 元素生命周期误用）、`mygo i18n check` |

---

## 8. 构建与发布

- 开发：`go tool mygo dev`（Go 文件变更自动重建重启）。
- 打包：`go tool mygo build -platform darwin/universal,windows/amd64,windows/arm64,linux/amd64,linux/arm64`，产出 `.dmg`、NSIS 安装包、`.deb` / `.rpm`。
- CI：`ci.yml` 跑测试和静态检查；`release.yml` 在推 `vX.Y.Z` 标签时构建、签名、`-upload` 到 GitHub Release 草稿。
- 签名：macOS 需要 Developer ID 证书才能公证，Windows 需要代码签名证书。**这两项需要你提供**；没有的话先用 ad-hoc 签名，用户首次打开要手动放行。

---

## 9. 实施阶段

每个阶段结束时应用可运行、测试全绿，并提交一次。

### 阶段 0：技术验证（go/no-go 关卡）

脚手架 `mygo init -template native`，做三个一次性原型：

1. `ui.Table` 显示 10 万行 × 60 列，测滚动帧耗时、横向滚动、块选复制；
2. `ui/editor` SQL 模式：中文输入法、补全回调、5000 行脚本编辑；
3. `ui/markdown` 流式渲染含表格和代码块的长回复。

**通过标准**：滚动与输入无可感卡顿；中文输入法候选窗位置正确；三个原型在 macOS 上稳定。Windows / Linux 我在本机无法实测，需要你在对应机器上跑一次或接受以 CI 构建成功为准。

**不通过则**：改走 MyGo webview 前端方案，复用现有 React 代码，第 4 节的领域层和应用层设计不变。

### 阶段 1：地基

领域层迁入并测试全绿；换 `modernc.org/sqlite`；`internal/store`（SQLite + 钥匙串 + BoltDB 迁移）；`internal/app` 骨架；i18n 目录；主题令牌；单实例锁；日志。

### 阶段 2：连接与浏览

连接管理窗口（增删改、测试、SSH、标签颜色、备份导入）；连接窗口外壳（标题栏、侧边栏树、Tab、状态栏）；数据库切换；只读表数据网格（分页、排序、筛选）。

### 阶段 3：查询

SQL 编辑器、执行与取消、多结果集、schema 补全、错误诊断、格式化、历史与收藏、命令面板。

### 阶段 4：编辑

单元格编辑、增删行、SQL 预览后提交；检查器面板；结构与索引编辑加 DDL diff；DDL 视图；删表/清空表确认。

### 阶段 5：AI

AI 面板（流式 Markdown、工具时间线、结构化结果表）、`@表` 提及、会话列表、写操作确认流、编辑器幽灵文本补全、AI 设置。

### 阶段 6：外围

导出、导入、进程列表、表文档、设置窗口、原生菜单栏、文件关联、日志查看。

### 阶段 7：发布

更新密钥与 updater 插件、CI/CD、图标、README 中英文、从 Wails 版迁移的说明。

---

## 10. 需要你确认的事项

1. **原生 UI 路线**（第 1 节）是否接受，包括阶段 0 不通过时退回 webview 的预案。
2. **一个连接一个窗口**（3.2 第 1 条）。这是对现有使用习惯改动最大的一项。
3. **砍掉的功能**（3.3），尤其是 Mermaid 和使用统计。
4. **数据目录迁移**（5.2）到系统标准位置，旧 `~/.minidb` 保留不删。
5. **签名证书**（第 8 节）是否有 Apple Developer ID 和 Windows 证书。
6. 新代码是放进现有仓库 `ixxxxoooo/MiniDB` 的新分支，还是新建仓库。

---

## 附录 A：阶段 0 实测结果（2026-11-13，macOS arm64，MyGo v0.9.0，Go 1.27.3）

原型位于会话临时目录，不入库；结论如下。

| 项目 | 结果 | 判定 |
|---|---|---|
| 10 万行 × 60 列表格，纵向/横向滚动、空闲帧 | 中位数约 2.1 ms，p95 约 2.6–2.8 ms，最大 3.8 ms；每帧只构建约 1650 个文本 | 通过 |
| 表格单元格块选（点击 + Shift 点击） | 选中行列正确，`SelectedColumns` 可取；暗色主题下渲染正常 | 通过 |
| SQL 编辑器，约 3900 行脚本打字 / 滚动 | 打字中位数 0.27 ms，滚动 0.04 ms；SQL 高亮、行号、当前行高亮正常 | 通过 |
| 输入法组合态（headless） | `Compose` 后能取到输入法候选窗锚点；提交的中文进入缓冲区 | 通过（仅 headless） |
| 流式 Markdown（5 KB，含表格、SQL 代码块、有序列表、中文） | 每帧中位数约 10 ms，p95 约 16 ms，最大 36 ms；表格、代码块高亮与复制按钮渲染正确 | 通过，但见下 |
| 真实窗口启动 | 二进制 17 MB，可启动，窗口渲染正常 | 通过 |

**结论：go，继续原生 UI 路线。**

### 遗留问题（不阻塞，但要在对应阶段处理）

1. **内存比官方数字高**：真实窗口空闲时 RSS 约 160 MB，官方宣称 hello-world 约 44 MB。原型含 6 万个列定义、5 千行 SQL、一篇 Markdown，没拆开定位是谁占的。阶段 2 做完真实表格后再测一次，超过现有 Wails 版（需同样口径测）才需要追。
2. **流式 Markdown 最大帧 36 ms**：对应换页/长块解析的偶发帧，超过 16 ms 会掉一帧。AI 面板阶段（阶段 5）用真实长回复再测；必要时用 `Reveal` 改按行/段落揭示来降频。
3. **输入法没在真实窗口里验证**：headless 测不到系统输入法的候选窗位置和上屏行为。**需要你在阶段 1 完成后，在真实窗口的 SQL 编辑器和输入框里用系统中文输入法打几句话确认**。
4. **补全弹窗没有视觉验证**：`OnCompletion` 回调已接通，但我渲染的截图里没拍到弹窗，阶段 3 做 schema 补全时再核对。
5. Windows / Linux 按你的决定不验证。

## 附录 B：阶段 1 进度

已完成（分支 `mygo`，`CGO_ENABLED=0 go test ./...` 全绿）：

- 删除 Wails 绑定服务、React 前端、Wails 构建配置、自研更新器、旧构建脚本（均在 `main` 分支历史中，可随时 `git show main:路径` 取回；`services/ai_tools*.go` 的工具逻辑留到阶段 5 迁入应用层）。
- `go.mod` 改为 MyGo v0.9.0 + `go tool mygo`；移除 `wailsapp/wails` 和 `mattn/go-sqlite3`。
- SQLite 驱动换成纯 Go 的 `modernc.org/sqlite`，驱动名 `sqlite3` → `sqlite`，`database` 包的 SQLite 测试通过。
- `schemaindex` 去掉对 Wails 的依赖：`SetWailsApplication` 改为 `OnStatusChanged(func(StatusEvent))` 回调。

已完成（续）：`main.go`（日志、单实例锁、窗口位置记忆）、`mygo.json`、图标、`internal/ui` 外壳及其 `Tester` 测试；`go tool mygo build -skip-dmg` 在 7.7 秒内产出 10.6 MB 的 `MiniDB.app`（ad-hoc 签名，旧 Wails 版二进制为 37 MB）。

实测补充：只含一个输入框和一个 SQL 编辑器的空壳 `.app`，空闲 RSS 约 114 MB（第 5 秒 CPU 16%，第 8 秒 8%，是否收敛到 0 还没测够长时间）。比官方 hello-world 的 44 MB 高，和遗留问题 1 是同一件事，要在阶段 2 与旧 Wails 版同口径对比后再下结论。

待做：`internal/store`（SQLite + 钥匙串 + BoltDB 迁移）、`internal/app` 骨架、`main.go` / `mygo.json` / i18n 目录 / 主题令牌 / 单实例锁。

---

## 附录 C：实施结果（2026-11-13）

### 与原方案的偏差（以此为准）

- **界面风格保持原版**（用户在实施中明确要求）：正文 3.2 中凡与原版布局冲突的改进作废。已按 `docs/FRONTEND_DEVELOPMENT_GUIDE.md` 与 `globals.css` 的令牌还原：36px 顶栏（插头 / 断开 / 切库 / SQL / 居中连接胶囊 / 搜索 / AI / 主题 / 设置）、多连接时 60px 工作区栏、260px 侧边栏（"TABLES (n)"）、28px 标签栏（激活标签下方强调色线）、表视图底栏（数据/结构/信息/文档分段 + 增删行 + 刷新 + 提交 + 行数/耗时 + 分页）、空格行预览、强调色跟随连接颜色、浅/深色令牌。实现在 `internal/ui/theme.go`、`layout.go`、`tablebar.go`。
- 保留的改进（不改变布局）：提交前预览 SQL / DDL、查询可取消、schema 补全、AI 结构化结果表、SQL 块一键插入/执行、会话恢复、退出前未提交修改确认。
- 快捷键沿用原版：⌘K 切换数据库、⌘P 命令面板、⌘I AI、⌘T 新查询。

### 完成情况

| 阶段 | 状态 |
|---|---|
| 0 技术验证 | 完成，见附录 A |
| 1 地基（领域层、SQLite 存储、钥匙串、BoltDB 导入、应用层、i18n） | 完成 |
| 2 连接与浏览 | 完成：连接列表/表单/SSH、备份导入、工作区、侧边栏、分页/排序/筛选 |
| 3 查询 | 完成：编辑器、补全、多语句、取消、多结果集、格式化、历史/收藏、命令面板 |
| 4 编辑 | 完成：单元格编辑、增删行、预览后事务提交、结构与索引编辑、DDL、删表/清空 |
| 5 AI | 完成：流式面板、工具时间线、结构化结果、会话、配置；已移除旧的双写事件通道和 5 个单次调用接口 |
| 6 外围 | 完成：导出/导入（进度、取消）、进程列表、表文档、设置、原生菜单栏、行预览 |
| 7 发布 | 部分：CI 与发布工作流、README、构建通过；自动更新的密钥与安装流程未配置 |

### 已知缺口（如实）

1. **没有在真实窗口里验证输入法**；headless 测试只覆盖到组合态与上屏文本。
2. **只用 SQLite 做了端到端测试**。MySQL / PostgreSQL / TiDB / StarRocks 的领域层测试沿用旧测试，但我没有连真实服务器跑过新界面。
3. **自动更新**：设置里只有"检查更新"；没有更新窗口、下载安装流程，也没有生成签名密钥（需要你运行 `go tool mygo keygen`）。
4. **编辑器里的 AI 幽灵补全、执行出错时的波浪线诊断**未做；补全只来自 schema 索引。
5. 原版有而本版没有的细节：schema 索引状态面板（胶囊上的柱状图图标）、紧凑模式 / 滚动条显示开关、表文档的富文本（改为 Markdown 源码 + 预览，按方案 3.3）、Mermaid（按决定先不做）、行预览里的 JSON 折叠。
6. 内存：空闲约 124 MB RSS（官方 hello-world 约 44 MB），旧 Wails 版父进程约 109 MB 外加 WebKit 进程；没有逐项定位。
7. 签名为 ad-hoc；没有 Developer ID 与公证。Windows / Linux 按决定不做。
