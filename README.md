# TablePlus AI

![TablePlus AI logo](frontend/src/assets/images/logo-universal.png)

TablePlus AI is an AI-powered desktop database client built with Wails, Go, React, and TypeScript. It focuses on fast database browsing, a polished macOS-style workflow, and a practical AI assistant that can understand schema context, explain SQL, generate queries, inspect data, and safely run read-only SQL when requested.

> This project is an independent open-source project and is not affiliated with, endorsed by, or sponsored by TablePlus.

**Language:** [English](#english) | [简体中文](#简体中文)

## English

### Highlights

- **Modern desktop database client**: connection management, database switching, workspaces, tabs, table browsing, pagination, sorting, filters, row preview, and context menus.
- **AI database assistant**: works with OpenAI-compatible APIs for natural-language-to-SQL, SQL explanation, error diagnosis, data insights, table documentation, and chat-based database Q&A.
- **Tool calling with safety controls**: streaming ReAct workflow, tool timeline, fuzzy table/column matching, DDL/stats/sample/profile inspection, EXPLAIN, and guarded read-only SQL execution.
- **Data and structure editing**: table structure view/editing, index creation/deletion, row insert/update/delete, batch commit, and rollback.
- **Export and documentation**: streaming CSV, JSON, and SQL INSERT exports; Markdown table docs with AI generation.
- **Bilingual UI**: Simplified Chinese and English, with system-language detection and manual switching.
- **Local-first storage**: connections, docs, settings, and logs stay on your machine; database passwords and AI secrets are encrypted before being stored locally.

### Supported Databases

| Database | Notes |
| --- | --- |
| MySQL | Native MySQL driver |
| PostgreSQL | Supports `sslmode` |
| SQLite | Connects through a local database file |
| TiDB | MySQL-compatible protocol |
| StarRocks | MySQL-compatible protocol with handling for prepare-statement limitations |

### Feature Overview

| Area | Capabilities |
| --- | --- |
| Connections | Create, edit, delete, test, tag, color-code, and locally encrypt saved connections |
| Browsing | Tables/views, paginated grid, sorting, structured filters, raw SQL filters, JSON preview, row details |
| SQL editor | Monaco Editor, syntax highlighting, format, minify, unescape, run current/selected/all statements, multi-result tabs |
| Structure | Columns, indexes, DDL, structure commits, schema index refresh |
| AI assistant | Streaming chat, schema-aware answers, table/tool mentions, SQL generation/fixing, error auto-fix, Mermaid preview |
| Export | CSV, JSON, SQL INSERT, batched streaming export, progress display, cancellation |
| App experience | Light/dark themes, compact layout, command palette, SQL history/favorites, logs, auto update |

### Tech Stack

| Layer | Technology |
| --- | --- |
| Desktop | Wails v3 alpha |
| Backend | Go 1.25+ |
| Frontend | React 18, TypeScript 5, Vite 6 |
| UI | Tailwind CSS, Radix UI, lucide-react |
| State | Zustand |
| Editors | Monaco Editor, Tiptap |
| Data grid | TanStack Table, TanStack Virtual |
| Storage | BoltDB / bbolt |
| AI | OpenAI-compatible API, Responses API with Chat Completions fallback |

### Quick Start

#### Requirements

- Go 1.25+
- Node.js 18+
- pnpm 10+
- Wails CLI v3 alpha
- macOS 10.15+ or Windows 10/11 amd64

#### Install Wails CLI

```bash
set -a && . ./project.env && set +a
go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}
```

#### Install dependencies

```bash
cd frontend && pnpm install && cd ..
go mod download
```

#### Run in development

```bash
wails3 dev -config ./build/config.yml
```

#### Configure AI

Open `Settings -> AI Config` in the app:

- `Base URL` defaults to `https://api.openai.com/v1`
- `API Key` is your OpenAI-compatible provider key
- `Model` can be `gpt-4o` or another compatible model name
- `System Prompt` controls response language, SQL style, and safety preferences

### Build and Verify

Generate bindings after changing Go service signatures:

```bash
wails3 generate bindings
```

Run the full local verification:

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 build
```

Package for macOS:

```bash
wails3 task package:darwin ARCH=arm64
./scripts/build.sh --arch arm64
```

Windows builds use `github.com/mattn/go-sqlite3` and require CGO. Validate Windows amd64 artifacts in native Windows with MSYS2/MinGW, or in a CI/Docker cross-compilation environment.

### Release and Auto Update

The repository includes GitHub Actions workflows:

- `.github/workflows/ci.yml`: Go tests, frontend tests, binding generation, and frontend build.
- `.github/workflows/release.yml`: builds macOS DMGs, Windows installer, update archives, `update.json`, and checksums when a `vX.Y.Z` tag is pushed.

Create a release:

```bash
./scripts/set-version.sh 1.0.1
git tag v1.0.1
git push origin v1.0.1
```

The in-app updater downloads the matching archive from GitHub Releases, verifies SHA-256, and prompts the user to restart and install.

### Data and Security

| Item | Default location |
| --- | --- |
| App data | `~/.tableplus-ai/data.db` |
| Local secret key | `~/.tableplus-ai/secret.key` |
| Logs | `~/.tableplus-ai/logs/` |

Security boundaries:

- Database passwords, AI keys, and custom AI headers are encrypted before being written to BoltDB.
- AI auto execution only allows one read-only SQL statement and rejects writes, multi-statement input, and risky cases such as `EXPLAIN ANALYZE`.
- The AI provider can receive schema context and the prompts you send. Do not send sensitive business data to an untrusted model provider.

### Keyboard Shortcuts

| Shortcut | Action |
| --- | --- |
| `Cmd/Ctrl + K` | Global search / command palette |
| `Cmd/Ctrl + T` | New query tab |
| `Cmd/Ctrl + W` | Close current tab |
| `Cmd/Ctrl + ,` | Open settings |
| `Cmd/Ctrl + Enter` | Run current or selected SQL |
| `Cmd/Ctrl + Shift + Enter` | Run all SQL |
| `Cmd/Ctrl + Shift + F` | Format SQL |
| `Cmd/Ctrl + S` | Save SQL |
| `Space` | Preview selected row |
| `Esc` | Close dialogs |

### Repository Layout

```text
tableplus-ai/
├── main.go                         # Wails entry point
├── internal/
│   ├── ai/                         # OpenAI-compatible client and AI features
│   ├── app/                        # Wails app, window, and lifecycle
│   ├── appdata/                    # User data paths
│   ├── database/                   # Connections, metadata, queries, SQL dialects, structure changes
│   ├── export/                     # CSV / JSON / SQL export
│   ├── schemaindex/                # Schema index and refresh state
│   ├── storage/                    # BoltDB and local secret encryption
│   ├── updater/                    # Auto update
│   └── version/                    # Version metadata
├── services/                       # Wails-bound services
├── frontend/
│   ├── src/components/             # Layout, table, editor, AI, settings, and UI components
│   ├── src/i18n/                   # zh-CN / en-US copy
│   ├── src/stores/                 # Zustand stores
│   └── package.json
├── docs/INSTALL.md                 # Installation guide
├── scripts/                        # Version, build, and release helpers
├── build/                          # Wails config and platform assets
└── .github/workflows/              # CI and release workflows
```

### Contributing

Issues, feature requests, and pull requests are welcome. Before opening a PR, please run:

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 build
```

Please follow the existing code style and use pnpm for frontend dependencies. Do not mix npm or Yarn lockfiles into the repository.

### License

[MIT](LICENSE)

---

## 简体中文

### 项目亮点

- **现代桌面数据库客户端**：连接管理、数据库切换、多工作区、多标签页、表数据浏览、分页、排序、筛选、行预览和右键菜单。
- **AI 数据库助手**：支持 OpenAI 兼容 API，可进行自然语言生成 SQL、SQL 解释、错误诊断、数据洞察、表文档生成和会话式问答。
- **工具调用与安全执行**：AI 会话支持 ReAct 工具调用、流式状态、工具时间线、表/列模糊匹配、DDL/统计/样例/画像读取、EXPLAIN，以及只读 SQL 自动执行保护。
- **结构与数据编辑**：支持表结构查看与编辑、索引创建/删除、行新增/修改/删除、批量提交与回滚。
- **导出与文档**：支持 CSV、JSON、SQL INSERT 流式导出；支持 Markdown 表文档编辑和 AI 生成文档。
- **中英文界面**：内置简体中文与英文，支持跟随系统语言或手动切换。
- **本地优先**：连接配置、文档、设置和日志保存在本机；数据库密码和 AI 密钥会在本地存储前加密。

### 支持的数据库

| 数据库 | 说明 |
| --- | --- |
| MySQL | 原生 MySQL 驱动 |
| PostgreSQL | 支持 `sslmode` 配置 |
| SQLite | 通过本地数据库文件连接 |
| TiDB | MySQL 协议兼容连接 |
| StarRocks | MySQL 协议兼容连接，针对预编译限制做了适配 |

### 功能概览

| 模块 | 能力 |
| --- | --- |
| 连接管理 | 新建、编辑、删除、测试连接；环境标签；颜色标识；本地加密存储 |
| 数据浏览 | 表/视图列表、分页数据表格、排序、条件筛选、原始 SQL 筛选、JSON 预览、行详情 |
| SQL 编辑器 | Monaco Editor、语法高亮、格式化、压缩、反转义、执行当前/选中/全部语句、多结果标签页 |
| 结构编辑 | 字段编辑、索引管理、DDL 查看、结构变更提交、schema 索引刷新 |
| AI 助手 | 流式对话、schema 感知、表/工具提及、AI 生成/修复 SQL、错误自动修复、Mermaid 预览 |
| 数据导出 | CSV、JSON、SQL INSERT；大表分批流式导出；进度展示与取消 |
| 应用体验 | 深浅色主题、紧凑布局、命令面板、SQL 历史/收藏、日志查看、自动更新 |

### 技术栈

| 层级 | 技术 |
| --- | --- |
| 桌面框架 | Wails v3 alpha |
| 后端 | Go 1.25+ |
| 前端 | React 18, TypeScript 5, Vite 6 |
| UI | Tailwind CSS, Radix UI, lucide-react |
| 状态管理 | Zustand |
| 编辑器 | Monaco Editor, Tiptap |
| 表格 | TanStack Table, TanStack Virtual |
| 存储 | BoltDB / bbolt |
| AI | OpenAI-compatible API, Responses API with Chat Completions fallback |

### 快速开始

#### 环境要求

- Go 1.25+
- Node.js 18+
- pnpm 10+
- Wails CLI v3 alpha
- macOS 10.15+ 或 Windows 10/11 amd64

#### 安装 Wails CLI

```bash
set -a && . ./project.env && set +a
go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}
```

#### 安装依赖

```bash
cd frontend && pnpm install && cd ..
go mod download
```

#### 启动开发模式

```bash
wails3 dev -config ./build/config.yml
```

#### 配置 AI

打开应用后进入 `设置 -> AI 配置`：

- `Base URL` 默认为 `https://api.openai.com/v1`
- `API Key` 使用你的 OpenAI 兼容服务密钥
- `Model` 可填写 `gpt-4o` 或其他兼容模型名称
- `System Prompt` 可自定义回答语言、SQL 风格和安全约束

### 构建与验证

修改 Go 服务签名后先生成绑定：

```bash
wails3 generate bindings
```

本地完整验证：

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 build
```

macOS 打包：

```bash
wails3 task package:darwin ARCH=arm64
./scripts/build.sh --arch arm64
```

Windows 构建依赖 `github.com/mattn/go-sqlite3` 和 CGO 环境；建议在原生 Windows + MSYS2/MinGW，或 CI/Docker 交叉编译环境中验证。

### 发布与自动更新

项目已包含 GitHub Actions 工作流：

- `.github/workflows/ci.yml`：运行 Go 测试、前端测试、绑定生成和前端构建。
- `.github/workflows/release.yml`：推送 `vX.Y.Z` tag 后构建 macOS DMG、Windows 安装包、自动更新包、`update.json` 和校验文件。

发布新版本：

```bash
./scripts/set-version.sh 1.0.1
git tag v1.0.1
git push origin v1.0.1
```

自动更新会从 GitHub Releases 下载当前平台对应的压缩包，校验 SHA-256 后提示重启安装。

### 数据与安全

| 内容 | 默认位置 |
| --- | --- |
| 应用数据 | `~/.tableplus-ai/data.db` |
| 本地加密密钥 | `~/.tableplus-ai/secret.key` |
| 运行日志 | `~/.tableplus-ai/logs/` |

安全边界：

- 数据库密码、AI Key 和自定义 AI 请求头在写入 BoltDB 前会使用本机密钥加密。
- AI 自动执行只允许单条只读 SQL，并拒绝写操作、多语句和 `EXPLAIN ANALYZE` 等高风险场景。
- AI 能看到当前连接的 schema 上下文和你发送的问题；请不要把敏感业务数据发送给不可信模型服务。

### 常用快捷键

| 快捷键 | 功能 |
| --- | --- |
| `Cmd/Ctrl + K` | 全局搜索 / 命令面板 |
| `Cmd/Ctrl + T` | 新建查询标签页 |
| `Cmd/Ctrl + W` | 关闭当前标签页 |
| `Cmd/Ctrl + ,` | 打开设置 |
| `Cmd/Ctrl + Enter` | 执行当前或选中 SQL |
| `Cmd/Ctrl + Shift + Enter` | 执行全部 SQL |
| `Cmd/Ctrl + Shift + F` | 格式化 SQL |
| `Cmd/Ctrl + S` | 保存 SQL |
| `Space` | 预览选中行 |
| `Esc` | 关闭弹窗 |

### 项目结构

```text
tableplus-ai/
├── main.go                         # Wails 入口
├── internal/
│   ├── ai/                         # OpenAI 兼容客户端与 AI 能力
│   ├── app/                        # Wails 应用、窗口和生命周期
│   ├── appdata/                    # 用户数据路径
│   ├── database/                   # 连接、元数据、查询、SQL 方言和结构变更
│   ├── export/                     # CSV / JSON / SQL 导出
│   ├── schemaindex/                # schema 索引与刷新
│   ├── storage/                    # BoltDB 与本地密钥加密
│   ├── updater/                    # 自动更新
│   └── version/                    # 版本信息
├── services/                       # Wails 绑定服务
├── frontend/
│   ├── src/components/             # 布局、表格、编辑器、AI、设置等组件
│   ├── src/i18n/                   # zh-CN / en-US 文案
│   ├── src/stores/                 # Zustand 状态
│   └── package.json
├── docs/INSTALL.md                 # 安装说明
├── scripts/                        # 版本、构建、发布辅助脚本
├── build/                          # Wails 构建配置与平台资源
└── .github/workflows/              # CI 与 Release
```

### 贡献

欢迎提交 Issue、功能建议和 Pull Request。建议 PR 前先运行：

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 build
```

贡献时请尽量遵循现有代码风格，避免混用 npm/Yarn 锁文件；前端依赖统一使用 pnpm。

### 许可证

[MIT](LICENSE)
