# MiniDB 界面规范

界面是 [MyGo](https://mygo.run/) 的原生 UI（纯 Go、立即模式）。这份文档记录视觉与交互上已经定下来的规则，新增界面时照着做，不另起一套。具体的数值和颜色以代码为准：`internal/ui/theme.go`（令牌与尺寸）、`internal/ui/btn.go`（按钮）、`internal/ui/tabstrip.go`（标签）、`internal/ui/grid.go`（数据网格）。

## 1. 气质

桌面数据库工具：信息密度高、层级清晰、克制，参考 TablePlus 和 macOS 的工具软件。数据安全感优先——危险操作要能一眼认出来，并且要二次确认。

不要做：营销页式的大图大字、大面积渐变装饰、卡片套卡片、为普通区域加大圆角。

## 2. 令牌

颜色、尺寸都走 `theme.go` 里的 `tokens` 和常量，业务代码里不写死颜色值。

| 类别 | 规则 |
| --- | --- |
| 底色层级 | `surface`（内容区）→ `surfaceSecondary`（次级区域、斑马纹、表头）→ `surfaceElev`（浮层）；工具栏、标签栏用 `toolbar` |
| 强调色 | `accent`，**跟随当前连接的颜色**（`Shell.tokens` 已处理）；表达"当前连接的上下文"就用它，不要另起主色 |
| 文字 | `fg` / `fgSecondary` / `fgMuted` 三档 |
| 状态 | `success` / `warning` / `danger` / `info`；禁用只降透明度，不改布局 |
| 分隔 | 边框优先于阴影：`border`（常规）、`borderSubtle`（表格内细线）；阴影只给弹窗、菜单、浮层 |
| 语义色 | 图标按对象着色（表蓝、视图紫、查询绿，`iconColor`）；值按类型着色（数字蓝、时间青，`valueColor`） |

尺寸（DIP）：工具栏 36（`sizeToolbar`）、标准按钮 28（`sizeBtn`）、小按钮 24（`sizeBtnSm`）、标签 28（`sizeTab`）、输入框 32（`sizeInput`）；字号正文 13、密集信息（表格、标签、侧栏）12；圆角按钮 7、面板 12。表格、SQL、标识符用等宽字体 `mono`，工具栏和菜单用界面字体。

## 3. 按钮

只有三个层级，高度、字号、图标、内边距各自固定，**不要在调用处单独设 `Height` / `FontSize`**：

| 函数 | 用在 | 内容 |
| --- | --- | --- |
| `actBtn` / `actPrimary`（24） | 面板、列表、工具栏里的操作 | 图标 + 文字 |
| `fieldBtn` / `fieldPrimary`（28） | 和 28px 输入框同排 | 图标 + 文字 |
| `footBtn` / `footPrimary`（28） | 对话框底部的取消、确认 | 纯文字 |

只有图标的按钮用 `tipBtn` / `toggleBtn`，必须带悬停提示；有快捷键的提示里带上 `keyHint`。

## 4. 标签

主标签栏、查询结果页的「结果 N」、结构视图下半区的「索引 / 外键 / 触发器」是**同一种标签**，外观集中在 `tabstrip.go`：

- 高度 `sizeTab`（28），底边一条贯穿的分隔线，标签之间只隔 1px 竖线，没有空隙；
- 选中的标签铺内容区底色，顶上一道 2px 的连接色线，盖住底线，和下面的内容连成一体；悬浮的标签略微提亮；
- 主标签栏的标签可以关闭（悬浮出现 ×，未保存时显示圆点，中键关闭），右端有「＋」新建查询；结果页等不可关闭的标签用 `tabStrip`（`trailing` 放靠右的操作按钮）。

底部的 Data / Structure / DDL / Doc 和侧栏顶部的「项目 / 查询 / 历史」是另一类：`pillTabs`，圆角分段开关（侧栏里 `grow` 为真，三段等分整行、更高一些）。

## 5. 表格

数据网格和结构视图都建在 `ui.Table` 上，共用同一套外观：

- 主题：`gridSpacing`、12px 等宽字、表头用正文色；行高固定 `gridRowH`，所以下方的斑马纹（`paintGridBg`）能对齐；
- 列宽拖表头边缘调整，列序拖表头挪动，双击边缘按内容适配；
- 行状态用整行底色：新增偏绿、待删除偏红、选中偏强调色（`cellBox` 的 tint 会盖满单元格含内边距，行内没有缝）；
- 选中行不要整行刷成实心强调色（里面的输入框、勾选框会看不清）：给表格换一份"淡强调色"的主题，网格里的控件另用原主题。

### 结构视图（`structview.go`、`objects.go`）

- 上半区是列的网格，**就地编辑**（名称、类型、可空、默认值、备注都是单元格里的控件）；新增行铺绿、待删除行铺红；
- MySQL 系数据库可以**拖动行上下换位**调整列序（`ListState.Reorder`，生成 `AFTER` / `FIRST`）；SQLite、PostgreSQL 改不了列序，不给拖动；
- 下半区用 `tabStrip` 切换 索引 / 外键 / 触发器，标签栏右端是当前这一项的「添加」按钮；上下两块之间的分隔条可以拖；
- **添加索引不弹窗**：表格末尾直接多一行，名称格拿到焦点，列用 `TokenField` 选，名称留空自动取 `idx_表_列`；外键、触发器表单较复杂，仍用对话框；
- 数据行隔行铺斑马纹（库自带的斑马纹要开单元格选择，和拖动排序冲突，所以由 `structRowTint` 在单元格里铺），选中的行不铺；
- 选中行后 ⌫ / Delete 删除；右键菜单有上移、下移、删除；提交走 `commitStructure` → `safeGate`，⌘S 提交。

## 6. 对话框与危险操作

- 对话框统一用 `Shell.dialog`，表单项用 `dlgField`，底部 `footBtn` / `footPrimary`；
- 删除、清空等不可恢复的操作用 `askDanger`（默认焦点在"取消"）；会丢失未保存内容的用 `askWarn`；
- 写库操作经过 `safeGate`（安全模式：静默 / 确认 / 密码），并写入控制台日志；
- 反馈用 `toastOK` / `toastErr` / `toastWarn`，不要用弹窗报告普通结果。

## 7. 国际化

所有用户可见的文字都进 `locales/zh-Hans.json` 和 `locales/en.json`，然后 `go tool mygo generate`，代码里用生成的 `locales.Xxx()`。提交前 `go tool mygo i18n check` 要通过。不再被引用的键要及时删掉。

## 8. 快捷键

窗口级快捷键由 `keymap.go`（按键组合与配置名）和 `shortcuts.go`（统一处理，焦点在哪里都生效）负责，用户可以在设置 → 快捷键里改；新增命令先登记再接处理函数，不要在视图里零散地写全局 `Shortcut`。只在某个元素有焦点时才生效的按键（如表格里的 ⌫）用该元素自己的 `Shortcut`，这样焦点在输入框时按键归输入框。

## 9. 编码约定

- 视图函数每帧都会执行：**不做 I/O**，阻塞操作放进 `uiThread.bg`，结果回到 UI 线程；
- 元素要有稳定的 `c.Key(...)`：位置变化会让 `AutoFocus` 抢走焦点；
- 依赖方向 `ui → app → 领域层`，`app` 和领域包不引用 `mygo/ui`；
- 注释用中文，说明"为什么"，不复述代码。

## 10. 测试与验收

- 界面用 `ui.Tester` 无窗口地跑（`newEnv`、`openWorkspace`、`e.tt.*`），`go test ./...` 覆盖交互；
- `MINIDB_SHOTS=<目录> go test ./internal/ui -run <名字>` 会把 `e.shot(...)` 的截图写成 PNG，改完界面务必看一眼；
- 验收清单：浅色 / 深色都看过；中英文都不截断；`go vet ./... && go tool mygo vet .`、`go tool mygo i18n check`、`go test ./...` 全部通过；新增行为有对应测试。
