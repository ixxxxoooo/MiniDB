# MiniDB 前端开发与 UI 规范

本文档基于当前项目代码整理，用于后续新增页面、功能和组件时保持一致的技术方案、交互方式与视觉风格。目标不是重新设计一套系统，而是把现有代码中已经形成的规则固化下来。

## 1. 产品定位

MiniDB 是一款 AI 增强的数据库管理桌面应用，前端体验应始终服务于高频、密集、专业的数据库操作。

核心气质：

- 桌面应用优先，不做网页营销风格。
- 信息密度高，但层级清晰。
- 操作反馈明确，避免大面积装饰。
- 参考 TablePlus / macOS 工具软件：克制、紧凑、稳定。
- 数据安全感优先：危险操作必须可识别、可确认、可取消或可恢复。

不推荐：

- 大 hero、营销卡片、夸张渐变背景。
- 大圆角、大留白、纯展示型布局。
- 为单个新功能引入独立视觉体系。
- 在主工作区堆叠嵌套卡片。

## 2. 技术栈规范

当前前端技术栈：

| 分类 | 规范 |
| --- | --- |
| 框架 | React 18 + TypeScript |
| 构建 | Vite 6 |
| 桌面桥接 | Wails v3 alpha 生成绑定 |
| 样式 | Tailwind CSS 3 + CSS 变量 |
| 状态管理 | Zustand 5，按领域拆分 store |
| 表格 | `@tanstack/react-table` + `@tanstack/react-virtual` |
| SQL 编辑器 | Monaco Editor |
| Markdown 编辑 | Tiptap |
| Markdown 渲染 | `react-markdown` + `remark-gfm` + DOMPurify |
| 图标 | `lucide-react` |
| UI 基础能力 | Radix primitives，已有基础组件优先 |
| 测试 | Vitest |
| 包管理 | pnpm |

开发命令：

```bash
cd frontend
pnpm install
pnpm test
pnpm build
```

Wails 开发模式从仓库根目录运行：

```bash
wails3 dev -config ./build/config.yml
```

修改 Go 服务签名后，需要重新生成前端绑定：

```bash
wails3 generate bindings
```

## 3. 目录与职责

前端目录以功能和职责混合组织：

```text
frontend/src/
  components/
    ai/            AI 面板、流式消息、AI Markdown 处理
    connection/    数据库连接弹窗、驱动图标
    editor/        SQL / Markdown 编辑器
    layout/        应用主布局、侧边栏、工具栏、状态栏、命令面板
    settings/      设置页
    table/         数据网格、筛选、右键菜单、行预览
    tabs/          Tab 容器、表视图、查询视图、DDL、文档
    ui/            可复用基础组件
  hooks/           业务 hooks 和跨组件逻辑
  i18n/            双语文案
  lib/             工具函数、Wails runtime/service 适配
  stores/          Zustand 状态
  types/           领域类型
```

新增代码放置规则：

- 可复用基础控件放在 `components/ui`。
- 单一业务域内使用的组件放在对应业务目录，例如表格功能放在 `components/table` 或 `components/tabs`。
- 复杂业务逻辑优先抽为 hook，例如 `useTableDataEditor`、`useTableViewResources`。
- Wails 服务调用统一从 `src/lib/wails/services/*` 引入，不直接访问 `frontend/bindings`。
- 共享类型放在 `types`，不要在多个组件中重复声明同一领域类型。

## 4. TypeScript 与 React 规范

基础约束：

- 保持 `strict: true` 兼容。
- 使用函数组件与 hooks。
- 路径别名统一使用 `@/`。
- 组件 props 明确声明接口或内联类型。
- 对外暴露的 hook、组件、工具函数保持稳定命名。

组件编写建议：

- UI 状态尽量局部化；跨页面或需持久化的状态进 Zustand。
- 高频渲染区域用 `useMemo`、`useCallback`、`React.memo` 控制成本。
- 异步函数必须处理 loading、error、finally。
- 对 Wails 后端返回的可空结果做兜底，例如 `result.rows || []`。
- 避免在 render 中构造大型数组或复杂计算。
- 事件处理函数命名使用 `handleXxx`。

样式类合并：

```ts
import { cn } from "@/lib/utils";
```

所有条件类名优先使用 `cn`，它已经封装 `clsx` 与 `tailwind-merge`。

## 5. 状态管理规范

当前使用 Zustand，并按领域拆分：

- `stores/ui.ts`：布局、预览栏、分页、toast、导出任务、紧凑模式。
- `stores/theme.ts`：浅色、深色、跟随系统。
- `stores/tabs.ts`：Tab、工作区激活 Tab、查询结果缓存。
- `stores/connection.ts`：连接、数据库、表、工作区。
- `stores/sqlHistory.ts`：SQL 历史。

状态放置规则：

- 组件内部临时交互状态：使用 `useState`。
- 多组件共享状态：使用对应 store。
- 需要重启后保留：使用 Zustand `persist`。
- 不应持久化的数据：toast、导出进度、loading、临时错误、查询结果大数据。

持久化注意：

- `partialize` 中排除不可持久化或体积大的状态。
- 恢复状态后要校验引用是否仍有效，例如 tab id、workspace id。
- 不要把 Promise、DOM 节点、函数引用放入持久化状态。

## 6. Wails 服务调用规范

服务调用入口：

```ts
import * as QueryService from "@/lib/wails/services/QueryService";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";
```

约定：

- 组件不直接依赖 Go 层实现细节，只依赖生成的服务函数和 `types`。
- 业务 hooks 可封装服务调用，例如 `useDatabase` 封装连接、断开、加载表。
- 服务调用需要 try/catch，并给用户可见反馈或记录 console。
- 数据库操作类接口必须确认 `connectionId`、`database`、`table` 是否存在。
- 修改数据后要刷新本地状态或触发重新加载。
- 连接、断开等并发敏感操作应使用任务缓存，避免重复请求。

错误反馈：

- 可恢复业务错误：toast。
- 当前 tab 内错误：优先使用 `reportTabError`。
- 调试信息：`console.error("[Module] xxx:", e)`。
- 不要吞掉会影响用户判断的错误。

## 7. UI 设计原则

### 7.1 整体布局

主布局是桌面数据库工具布局：

- 顶部工具栏：窗口控制、连接状态、常用命令。
- 左侧侧边栏：工作区、库表列表、搜索、上下文菜单。
- 中央内容区：TabBar + TabContent。
- 右侧可选预览区：行详情、辅助信息。
- AI 面板：右侧可拖拽面板，不影响主表格核心工作流。

新增主功能应优先以以下形式出现：

- 新 Tab 类型。
- 现有 TableView 的子视图。
- 工具栏按钮 + 弹窗。
- 右键菜单动作。
- 设置页新增 tab。

避免创建全屏单页式入口，除非该功能确实是主工作流。

### 7.2 信息密度

默认尺寸偏紧凑：

- 工具栏高度：`var(--size-toolbar)`，默认 36px。
- 标准按钮：`var(--size-btn)`，默认 28px。
- 小按钮：`var(--size-btn-sm)`，默认 24px。
- 输入框：`var(--size-input)`，默认 32px。
- Tab 高度：`var(--size-tab)`，默认 28px。
- 常规间距：`var(--size-gap)`，默认 8px。
- 小间距：`var(--size-gap-sm)`，默认 4px。

新增功能必须同时兼容 `.compact` 模式，不要写死过大的高度、字号和间距。

### 7.3 视觉层级

层级从低到高：

1. 主背景：`var(--surface)`。
2. 次级区域：`var(--surface-secondary)`。
3. 浮层 / 弹窗 / 高亮容器：`var(--surface-elevated)`。
4. 交互强调：`var(--accent)`。
5. 危险动作：`var(--danger)`。

边框优先于阴影：

- 常规分隔用 `border-[var(--border-color)]`。
- 表格内部细分隔用 `var(--border-subtle)`。
- 阴影只用于弹窗、菜单、浮层，不用于普通页面区域。

### 7.4 色彩

必须使用 CSS 变量，不要在业务组件中硬编码大面积颜色。

核心变量：

```css
--accent
--accent-fg
--accent-hover
--surface
--surface-secondary
--surface-elevated
--fg
--fg-secondary
--fg-muted
--border-color
--border-subtle
--success
--warning
--danger
--info
```

连接色联动：

- 当前连接的 `color` 会覆盖 `--accent`、`--sidebar-accent`、`--tab-active-border`、选中行背景。
- 新组件如果表达“当前连接上下文”，应使用 `--accent`，不要另起主色。

状态颜色：

- 成功：`var(--success)`。
- 警告：`var(--warning)`。
- 危险：`var(--danger)`。
- 信息：`var(--info)`。
- 禁用：降低 opacity，不改变布局。

### 7.5 字体

全局字体：

```css
-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", Arial, sans-serif
```

等宽字体：

```css
var(--font-mono)
```

字号变量：

- `var(--size-font-base)`：主要正文，默认 15px。
- `var(--size-font-sm)`：普通控件，默认 14px。
- `var(--size-font-xs)`：辅助控件，默认 13px。
- `var(--size-font-2xs)`：密集信息，默认 12px。

使用规则：

- 表格、SQL、数据库字段、连接详情优先使用等宽字体。
- 工具栏、菜单、Tab 使用小字号。
- 不使用 viewport 缩放字号。
- 不使用负 letter-spacing。

### 7.6 圆角

圆角变量：

- 窗口：`var(--radius-window)`，12px。
- 面板：`var(--radius-panel)`，12px。
- 按钮：`var(--radius-btn)`，7px。
- 输入框：`var(--radius-input)`，8px。
- 菜单：`var(--radius-menu)`，8px。
- 小元素：`var(--radius-sm)`，4px。

规则：

- 普通按钮使用 `rounded-[var(--radius-btn)]`。
- 小标签、图标底座使用 `rounded-[var(--radius-sm)]`。
- 弹窗使用 `rounded-[var(--radius-panel)]`。
- 不为普通页面区域创建大圆角卡片。

## 8. 基础组件规范

### 8.1 Button

统一使用 `components/ui/button.tsx`：

```tsx
<Button variant="ghost" size="icon">
  <Settings className="h-3.5 w-3.5" />
</Button>
```

变体：

- `default`：主操作。
- `ghost`：工具栏、图标按钮、低强调操作。
- `outline`：次级按钮。
- `destructive`：危险操作。
- `link`：文本链接。

尺寸：

- `default`：常规按钮。
- `sm`：密集区域按钮。
- `lg`：少用，仅表单主操作。
- `icon`：图标按钮。

按钮规则：

- 工具类操作优先图标按钮，并加 Tooltip。
- 文本按钮用于明确命令，例如保存、取消、测试连接。
- 危险操作使用 `destructive` 或危险色 hover。
- 禁用态必须传 `disabled`，不要只靠 CSS 阻止点击。

### 8.2 Input

统一使用 `components/ui/input.tsx`。输入框高度、边框、focus ring 已内置。

规则：

- 表单字段必须有可见 label 或清晰上下文。
- placeholder 只做提示，不承载必填信息。
- 校验错误用边框或提示文字表达。
- 数字输入要转换类型，不要把字符串直接传给后端数字字段。

### 8.3 Tooltip

工具栏图标、含义不明显的按钮都需要 Tooltip。

规则：

- 延迟通常使用 250-300ms。
- Tooltip 文案要短，必要时包含快捷键，例如 `新建查询 (⌘T)`。
- 不使用 `title` 作为主要 Tooltip；已有 `TitleTooltipBridge` 只用于兼容。

### 8.4 Badge

Badge 用于小型状态和标签。不要用 Badge 承载主要操作。

建议：

- 标签类：使用 `secondary` 或业务色。
- 成功 / 警告 / 危险：使用对应 variant。
- 在选中行或高亮背景上，要确保对比度。

## 9. 图标规范

统一使用 `lucide-react`。

常用映射：

| 功能 | 图标建议 |
| --- | --- |
| 新连接 | `Plug` |
| 断开 | `Unplug` |
| 数据库 | `Database` |
| 表 | `Table2` |
| 设置 | `Settings` |
| 搜索 | `Search` |
| 刷新 | `RefreshCw` |
| 关闭 | `X` |
| 删除 | `Trash2` |
| 下载 / 导出 | `Download` / `FileDown` |
| AI | `Sparkles` / `Bot` |
| 执行 | `Play` |
| 复制 | `Copy` |

规则：

- 图标尺寸跟随变量：`var(--size-btn-icon)` 或 `var(--size-btn-icon-sm)`。
- 工具栏图标一般 12-16px。
- 图标按钮必须保证点击区域不小于 24px。
- 不手写 SVG，除非是已有品牌/数据库图标或无法用 lucide 表达的窗口控制图形。

## 10. 弹窗与浮层规范

常规弹窗结构：

- `fixed inset-0` 遮罩。
- `bg-black/40 backdrop-blur-sm`。
- 居中容器，`z-50`。
- `bg-[var(--surface)] border-[var(--border-color)] shadow-lg`。
- ESC 关闭。
- 点击遮罩关闭，除非存在未保存数据。

弹窗尺寸：

- 设置弹窗当前为 `760px * 560px`。
- 连接弹窗可按内容调整，但不应超过主窗口。
- 内容超出时内部滚动，不让整个窗口滚动。

危险操作：

- 删除连接、DROP、TRUNCATE、批量删除等必须二次确认。
- 文案中说明对象名称。
- 主按钮使用危险色。

## 11. 表格与数据网格规范

表格是本项目核心界面，新增相关能力必须复用 `DataGrid`。

当前表格特征：

- `@tanstack/react-table` 负责列和排序。
- `@tanstack/react-virtual` 用于大列表或侧边栏表列表虚拟滚动。
- 列宽根据字段类型、表头和采样数据自动计算。
- 支持手动拖拽列宽、双击 auto-fit。
- 使用等宽字体和细边框。
- 行选中、悬停、新增、删除、编辑都有专门状态色。

表格尺寸：

- 单元格：`px-2 py-0.5 text-xs`。
- 表头：`px-2 py-1 text-2xs font-semibold`。
- 行号列保持固定约束。
- 数据区域滚动条使用 `.scroll-always`。

表格交互：

- 单击选择行。
- 多选使用 Shift / Meta 逻辑时要保持当前模式。
- 右键菜单根据行、列、单元格上下文变化。
- 空格预览行详情。
- 编辑数据需要显式提交。
- 删除已有行进入 pending 状态，新建行可直接移除。

数据编辑规则：

- 编辑状态由 `useTableDataEditor` 管理。
- 以主键作为更新和删除依据。
- 无主键时禁止删除或更新已有行，并给出 toast。
- 新增行只提交非空字段。
- 提交成功后重置编辑状态并刷新数据。

列类型处理：

- boolean：窄列。
- date/time/datetime：使用对应输入类型和格式转换。
- enum：使用候选项。
- JSON / 大文本：宽列，提供预览。
- NULL 使用明确展示，不与空字符串混淆。

## 12. SQL 编辑器规范

SQL 编辑器使用 Monaco，相关功能放在 `components/editor/SQLEditor.tsx` 与 `tabs/QueryView.tsx`。

必须保留：

- 执行当前语句 / 选中语句。
- 执行全部语句。
- 格式化 SQL。
- 多语句拆分结果以 result tabs 展示。
- 错误结果可触发 AI 修复。

新增 SQL 功能规则：

- 根据连接类型推断 SQL dialect。
- 任何自动生成 SQL 都要允许用户检查后再执行，除非已有明确自动执行流程。
- 查询结果需要分页或 auto-limit。
- 不在前端拼接危险 SQL 执行用户不可见动作。

## 13. AI 面板规范

AI 面板是辅助工作流，不应打断表格和 SQL 主工作流。

当前约定：

- 面板懒加载。
- 使用 Wails service 与事件流接收响应。
- Markdown 渲染需通过规范化、语法高亮和 DOMPurify。
- AI 输入支持 `@table:`、`@tool:` mention。
- 流式输出有状态时间线、工具调用时间线、下一步建议。

新增 AI 功能规则：

- 所有 AI 输出默认视为不可信文本。
- 可执行 SQL 必须清楚展示，用户确认后执行。
- 解析模型 JSON 时需要容错。
- 长消息和代码块必须可复制。
- AI 失败要显示可理解错误，不只写 console。

## 14. 国际化规范

项目支持 `zh-CN` 与 `en-US`。

规则：

- 新增用户可见文案必须加入 `i18n/zh-CN.ts`、`i18n/en-US.ts` 和类型定义。
- 组件中通过 `useTranslation()` 获取 `t`。
- 非组件工具函数可使用导出的 `t` 函数，但要注意语言切换重渲染问题。
- 快捷键、品牌名、SQL 关键字不翻译。
- 错误消息尽量短，必要时补充对象名称。

示例：

```tsx
const { t } = useTranslation();
return <span>{t("tabs.newQuery")}</span>;
```

## 15. 快捷键规范

全局快捷键使用 `useKeyboard`。

当前约定：

- `mod` 在 macOS 表示 Command，在 Windows/Linux 表示 Ctrl。
- 捕获阶段监听，避免 Monaco 吞掉全局组合键。
- 精确匹配修饰键，避免误触。
- 输入法 composing 时不触发。

常用快捷键：

| 快捷键 | 功能 |
| --- | --- |
| `⌘P` | 命令面板 |
| `⌘K` | 切换数据库 |
| `⌘T` | 新 SQL 查询 |
| `⌘,` | 设置 |
| `⌘W` | 关闭当前 Tab |
| `⌘N` | 新连接 |
| `⌘]` / `⌘[` | 切换当前工作区 Tab |
| `⇧⌘]` / `⇧⌘[` | 切换工作区 |

新增快捷键要求：

- 必须在设置或快捷键说明中可发现。
- 不覆盖系统通用快捷键，除非是桌面工具常见行为。
- 不在输入框、Monaco、ProseMirror 中破坏编辑快捷键。

## 16. 滚动与选择规范

全局默认禁用文本选择，以贴近桌面应用。

允许选择的区域：

- input / textarea / select。
- contenteditable。
- Monaco。
- ProseMirror。
- AI 聊天内容 `.ai-chat-selectable`。
- 表格单元格中明确允许复制的区域。

滚动条：

- 默认自动隐藏。
- 数据网格使用 `.scroll-always` 始终可见。
- 设置中允许用户切换全局滚动条显示。

新增滚动区域要设置 `min-h-0`，避免 flex 子项溢出。

## 17. 性能规范

高频区域必须注意性能：

- 表列表、数据表格使用虚拟滚动。
- resize 操作用 `requestAnimationFrame` 节流。
- 大数据渲染要分页或截断。
- 超长单元格文本限制渲染长度。
- 代码高亮使用缓存。
- AI 面板、重型编辑器可懒加载。

避免：

- 在 render 中遍历大数据做复杂格式化。
- 每次输入都触发后端请求。
- 持久化大查询结果。
- 大范围 store selector 导致无关组件重渲染。

## 18. 可访问性与可用性

虽然是桌面工具，也要保证基础可用性：

- 可点击非 button 元素必须加 `role="button"`、`tabIndex` 和键盘处理。
- 图标按钮需要 Tooltip 或 aria-label。
- focus ring 使用 `var(--accent)`。
- 禁用态要有 `disabled` 或 `aria-disabled`。
- 弹窗支持 ESC。
- 菜单和搜索列表支持键盘上下与 Enter。
- 文字必须 truncate 或 wrap，不能溢出遮挡。

## 19. 测试规范

当前测试集中在纯逻辑：

- AI stream meta 解析。
- Markdown 表格规范化。

新增测试建议：

- 字符串解析、SQL 拆分、JSON 提取、Markdown 规范化必须写单元测试。
- 数据转换、日期格式、筛选条件构造应写单元测试。
- UI 组件如无测试框架支持，可先抽离纯函数测试。
- 修改 Wails service 签名后至少运行前端 build。

常用验证：

```bash
cd frontend
pnpm test
pnpm build
```

## 20. 新功能开发流程

新增前端功能建议按此顺序：

1. 明确入口：Tab、子视图、工具栏按钮、右键菜单、设置页或 AI 面板。
2. 明确状态归属：局部 state、现有 store、还是新 store。
3. 明确后端接口：是否已有 Wails service，是否需要生成 bindings。
4. 先复用已有基础组件：Button、Input、Tooltip、Badge、DataGrid。
5. 使用 CSS 变量和尺寸变量完成 UI。
6. 加入 i18n 文案。
7. 处理 loading、empty、error、disabled、success 状态。
8. 补充快捷键或菜单入口。
9. 运行测试和构建。
10. 用紧凑模式、浅色、深色至少各检查一次。

## 21. UI 验收清单

提交前检查：

- 是否使用 `var(--surface)`、`var(--fg)`、`var(--border-color)`、`var(--accent)` 等变量。
- 是否兼容 `.dark`。
- 是否兼容 `.compact`。
- 是否没有新增大面积硬编码颜色。
- 是否没有嵌套卡片。
- 图标按钮是否有 Tooltip。
- 文案是否走 i18n。
- loading、empty、error、disabled 状态是否完整。
- 危险动作是否有确认。
- 滚动区域是否不会撑破布局。
- 文本是否不会溢出或遮挡。
- 表格或长列表是否考虑虚拟滚动、分页或截断。
- Wails 调用是否有 try/catch。
- 是否运行 `pnpm test` 或 `pnpm build`。

## 22. 推荐代码模式

### 22.1 服务调用

```tsx
const [loading, setLoading] = useState(false);

const handleLoad = useCallback(async () => {
  if (!connectionId || !database) return;
  setLoading(true);
  try {
    const result = await SomeService.Load(connectionId, database);
    setItems(result || []);
  } catch (e) {
    useUIStore.getState().addToast("error", "加载失败");
    console.error("[Feature] 加载失败:", e);
  } finally {
    setLoading(false);
  }
}, [connectionId, database]);
```

### 22.2 图标按钮

```tsx
<Tooltip delayDuration={300}>
  <TooltipTrigger asChild>
    <Button variant="ghost" size="icon" onClick={onRefresh}>
      <RefreshCw className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
    </Button>
  </TooltipTrigger>
  <TooltipContent side="bottom">{t("common.refresh")}</TooltipContent>
</Tooltip>
```

### 22.3 紧凑表单行

```tsx
<label className="grid grid-cols-[120px_1fr] items-center gap-2 text-[length:var(--size-font-xs)]">
  <span className="text-[var(--fg-secondary)]">{t("settings.name")}</span>
  <Input value={name} onChange={(e) => setName(e.target.value)} />
</label>
```

## 23. 命名规范

文件：

- React 组件：`PascalCase.tsx`。
- hook：`useXxx.ts`。
- 工具函数：`camelCase.ts` 或领域名。
- 类型文件：领域名，例如 `database.ts`。

变量：

- 事件处理：`handleXxx`。
- 布尔值：`isXxx`、`hasXxx`、`canXxx`、`showXxx`。
- store setter：`setXxx`。
- 异步加载：`loadXxx`、`fetchXxx`。

CSS class：

- 优先 Tailwind utility。
- 全局 class 只用于跨组件机制，例如 `.compact`、`.scroll-always`、`.ai-chat-selectable`。
- 不为单个组件在 `globals.css` 中新增大量样式，除非它是全局系统能力。

## 24. 何时新增抽象

可以新增抽象：

- 三个以上地方重复同一种 UI 或逻辑。
- 逻辑复杂到影响组件阅读。
- 需要稳定复用的业务行为，例如表格编辑、快捷键、服务加载。
- 和现有 `components/ui` 风格一致的基础控件。

不建议新增抽象：

- 只有一个调用点。
- 只是为了隐藏几行 JSX。
- 与现有组件风格不一致。
- 让类型和状态流更难追踪。

## 25. 后续扩展建议

如果后续要持续扩展前端，建议优先补齐：

- `components/ui` 中的 Select、Switch、Dialog、Tabs 基础封装。
- 表格相关纯函数的单元测试。
- 快捷键配置集中声明，便于设置页展示与自定义。
- 统一 toast 组件文档和危险操作确认组件。
- 更完整的 Story / 示例页，便于维护 UI 一致性。
