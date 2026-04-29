# TablePlus AI

一款 AI 增强的数据库管理桌面应用，采用 macOS 原生风格设计，参考 TablePlus UI/UX。

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | [Wails](https://wails.io/) v3 alpha |
| 后端 | Go 1.25+ |
| 前端 | React 18 + TypeScript 5 |
| 构建 | Vite 6 |
| 样式 | Tailwind CSS 3 |
| 状态管理 | Zustand 5 |
| 代码编辑器 | Monaco Editor |
| 表格组件 | @tanstack/react-table |
| 本地存储 | BoltDB (bbolt) |
| AI 客户端 | OpenAI 兼容 API |

## 支持的数据库

- **MySQL** (5.7+, 8.0+)
- **PostgreSQL** (12+)
- **SQLite**

## 功能特性

### 连接管理
- 新建/编辑/删除数据库连接
- 连接测试（一键验证连接可用性）
- 自动连接并展开指定数据库
- 断开连接（右键菜单或侧边栏操作）
- 连接配置持久化存储

### 数据浏览
- **三栏布局**：可折叠侧边栏 + 内容区 + 预览区
- **表格视图**：类 Excel 的数据表格，支持排序、行选择、斑马纹
- **分页浏览**：支持翻页查看大表
- **行预览**：选中行后按空格键在右侧预览字段详情
- **多标签页**：同时打开多个表/查询/DDL/文档

### 筛选功能（参考 TablePlus）
- 列选择 + 操作符 + 值的组合筛选
- 支持 `=`, `!=`, `>`, `<`, `>=`, `<=`, `LIKE`, `NOT LIKE`, `IS NULL`, `IS NOT NULL`, `IN` 操作符
- 多条件 AND 组合
- 一键清除筛选

### SQL 编辑器
- **Monaco Editor** 提供完整语法高亮和智能提示
- **格式化**：⌘⇧F 一键格式化 SQL（基于 sql-formatter）
- **压缩**：将 SQL 压缩为单行
- **反转义**：将转义字符还原
- **执行当前语句**：⌘↵ 执行光标所在语句（按分号分割）
- **执行选中部分**：选中文本后 ⌘↵ 仅执行选中部分
- **执行所有语句**：⌘⇧↵ 执行全部语句，结果分 Tab 展示
- **保存**：⌘S 保存 SQL
- **多语句结果**：多条语句分别执行，结果以多标签页展示

### 右键菜单
- **表右键**：复制表名、查看 DDL、表文档、导出数据、TRUNCATE、DROP
- **行右键**：预览行、复制单元格/整行/INSERT 语句、下载当前页、刷新、删除行
- **Tab 右键**：关闭、关闭其他、关闭右侧、关闭所有

### 数据库切换
- 状态栏点击当前数据库名称弹出切换器
- 搜索筛选数据库
- 支持键盘导航

### 数据导出
- 右键或工具栏"下载当前页"导出为 CSV
- 支持 CSV / JSON / SQL INSERT 三种格式

### AI 功能
- **自然语言转 SQL**：输入自然语言描述，自动生成 SQL
- **SQL 解释**：解释复杂 SQL 的执行逻辑和优化建议
- **数据洞察**：对查询结果生成数据摘要、异常检测、趋势分析
- **文档生成**：根据表结构自动生成 Markdown 文档
- **错误诊断**：SQL 执行报错时分析原因并给出修复建议
- **AI 配置**：支持 OpenAI 兼容 API（baseURL + apiKey）

### UI/UX
- **macOS Vibrancy**：半透明毛玻璃效果
- **深色/浅色主题**：完整双主题支持，跟随系统
- **快捷键**：⌘K 快速搜索、⌘T 新查询、⌘W 关闭标签、⌘, 设置
- **全局搜索**：快速查找表和操作

### 发布与自动更新
- **GitHub Actions 发布**：tag 触发构建 macOS DMG、Windows 安装包、自动更新压缩包和 `update.json`
- **应用内更新**：关于页可检查更新、显示下载进度、校验 SHA-256，并在更新包就绪后重启安装
- **平台覆盖**：当前自动更新支持 `macos-arm64`、`macos-amd64`、`windows-amd64`

## 项目结构

```
tableplus-ai/
├── main.go                          # Wails 入口：嵌入前端资源并启动 internal/app
├── internal/
│   ├── app/
│   │   ├── core.go                  # 业务依赖聚合与服务注册
│   │   ├── resources.go             # Wails 前端资源封装
│   │   └── runner.go                # Wails v3 应用、窗口、生命周期和更新器启动
│   ├── database/
│   │   ├── manager.go               # 数据库连接管理器
│   │   ├── metadata.go              # 元数据查询（库/表/列/DDL/统计）
│   │   └── query.go                 # SQL 执行和数据操作
│   ├── storage/
│   │   ├── store.go                 # BoltDB 存储引擎
│   │   └── history.go               # 查询历史记录
│   ├── ai/
│   │   ├── client.go                # OpenAI 兼容 API 客户端
│   │   ├── nl2sql.go                # 自然语言转 SQL
│   │   ├── explain.go               # SQL 解释
│   │   ├── insight.go               # 数据洞察
│   │   ├── docgen.go                # 文档生成
│   │   └── diagnose.go              # 错误诊断
│   ├── export/                      # 数据导出（CSV/JSON/SQL）
│   ├── updater/                     # GitHub Release 自动更新管理器
│   ├── version/                     # 编译期版本、commit、构建时间信息
│   └── logger/
│       └── logger.go                # 统一日志系统
├── services/                        # 服务层（Wails 绑定）
│   ├── connection_service.go
│   ├── database_service.go
│   ├── query_service.go
│   ├── doc_service.go
│   ├── settings_service.go          # 设置、应用信息、更新检查入口
│   ├── ai_service.go
│   ├── export_service.go
│   ├── history_service.go
│   └── clipboard_service.go
├── frontend/
│   ├── bindings/                    # Wails v3 生成绑定
│   ├── src/
│   │   ├── components/
│   │   │   ├── layout/              # 布局组件
│   │   │   ├── table/               # 数据表格组件
│   │   │   ├── editor/              # SQL/Markdown 编辑器
│   │   │   ├── tabs/                # 标签页管理
│   │   │   ├── ai/                  # AI 面板
│   │   │   ├── connection/          # 连接配置
│   │   │   ├── settings/            # 设置
│   │   │   └── ui/                  # 基础 UI 组件
│   │   ├── stores/                  # Zustand 状态管理
│   │   ├── hooks/                   # React Hooks
│   │   ├── types/                   # TypeScript 类型
│   │   └── lib/                     # 工具函数与 Wails runtime/backend 适配层
│   └── package.json
├── scripts/
│   ├── build.sh                     # 本地打包脚本
│   ├── set-version.sh               # 同步版本号到构建元数据
│   └── release/main.go              # 生成自动更新 update.json
├── .github/workflows/
│   ├── ci.yml                       # 测试、绑定生成、前端构建
│   └── release.yml                  # GitHub Release 与自动更新资产
└── build/
    ├── config.yml                   # Wails v3 构建配置
    ├── appicon.png                  # 应用图标
    ├── darwin/                      # macOS plist、DMG、Taskfile
    └── windows/                     # Windows manifest、NSIS、Taskfile
```

## 开发

### 环境要求

- Go 1.25+
- Node.js 18+
- Wails CLI v3 alpha

### 安装 Wails CLI

```bash
set -a && . ./project.env && set +a
go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}
```

### 安装依赖

```bash
# 安装前端依赖
cd frontend && pnpm install && cd ..

# 安装 Go 依赖
go mod tidy
```

前端包管理器统一使用 pnpm，并提交 `frontend/pnpm-lock.yaml`。不要混用 npm / Yarn 锁文件，避免 CI、本地构建和 Wails Task 解析到不同依赖树。

### 开发模式

```bash
wails3 dev -config ./build/config.yml
```

### 项目常量与版本

应用名、二进制名、bundle id、GitHub 仓库、Wails CLI 版本、macOS 最低版本和当前版本统一维护在仓库根目录的 `project.env`。需要发布或调整版本时，使用脚本统一同步派生元数据：

```bash
./scripts/set-version.sh 0.0.1
```

脚本会同步 `project.env`、Wails 配置、macOS plist、Windows 版本信息、前端 package 和 Go 运行时常量。不要手动分别修改这些派生文件。

### 构建

```bash
# 生成开发/生产绑定（修改 Go 服务签名后需要执行）
wails3 generate bindings

# Wails 标准构建
wails3 build -config ./build/config.yml

# macOS 本地打包，默认读取 project.env
wails3 task package:darwin ARCH=arm64

# 封装脚本：清理、构建 DMG，默认读取 project.env
./scripts/build.sh --arch arm64
```

构建产物位于 `bin/`、`dist/` 或 `build/bin/`（取决于具体 Task）。macOS DMG 会输出到 `dist/`，自动更新压缩包会输出到 `bin/release/<version>/`。

### 发布与更新

GitHub Actions 已配置：

- `.github/workflows/ci.yml`：push / PR 时运行 Go 测试、前端测试、生成绑定和前端构建。
- `.github/workflows/release.yml`：推送 `vX.Y.Z` tag 时构建 macOS arm64/amd64 DMG、Windows amd64 安装包、自动更新 tar/zip 资产和 `update.json`，并发布到 GitHub Releases。

创建新版本：

```bash
# 可选：先同步本地版本信息
./scripts/set-version.sh 1.0.1

git tag v1.0.1
git push origin v1.0.1
```

Release 需要包含以下关键资产：

- `TablePlus AI-<version>-macOS-arm64.dmg`
- `TablePlus AI-<version>-macOS-amd64.dmg`
- `TablePlus AI-<version>-Windows-amd64-Setup.exe`
- `tableplus-ai-<version>-macos-arm64.tar.gz`
- `tableplus-ai-<version>-macos-amd64.tar.gz`
- `tableplus-ai-<version>-windows-amd64.zip`
- `update.json`
- `checksums.txt`

应用内“关于”页会读取 `https://github.com/lwj1989/tableplus-ai/releases/latest/download/update.json` 检查最新版本。发现新版本后会下载当前平台对应的更新压缩包，校验 SHA-256，并提示重启安装；“打开发布页”仍作为手动下载兜底入口。

自动更新包约定：

- macOS：`.tar.gz` 内包含 `TablePlus AI.app`
- Windows：`.zip` 内包含更新后的 `.exe`
- 事件只传递状态、进度和错误等小 payload；大文件通过下载流处理，避免影响运行时事件性能

### 运行测试

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 task package:darwin ARCH=arm64
```

Windows 构建依赖 `github.com/mattn/go-sqlite3`，需要 CGO 环境；请在原生 Windows + MSYS2/MinGW，或 CI/Docker 交叉编译环境中验证 `windows/amd64` 产物。

## 日志

应用运行日志保存在 `~/.tableplus-ai/logs/` 目录下，按日期分割。
排查问题时可查看对应日期的日志文件。

## 数据存储

应用数据（连接配置、文档、设置等）保存在 `~/.tableplus-ai/data.db`（BoltDB 格式）。

## 快捷键

| 快捷键 | 功能 |
|--------|------|
| ⌘K | 全局搜索 |
| ⌘T | 新建查询标签页 |
| ⌘W | 关闭当前标签页 |
| ⌘, | 打开设置 |
| ⌘↵ | 执行当前 SQL 语句 |
| ⌘⇧↵ | 执行所有 SQL 语句 |
| ⌘⇧F | 格式化 SQL |
| ⌘S | 保存 SQL |
| Space | 预览选中行 |

## 许可证

MIT
