# TablePlus AI

一款 AI 增强的数据库管理桌面应用，采用 macOS 原生风格设计，参考 TablePlus UI/UX。

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | [Wails](https://wails.io/) v2 |
| 后端 | Go 1.23 |
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

## 项目结构

```
tableplus-ai/
├── main.go                          # Wails 应用入口
├── app.go                           # 应用生命周期管理
├── internal/
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
│   └── logger/
│       └── logger.go                # 统一日志系统
├── services/                        # 服务层（Wails 绑定）
│   ├── connection_service.go
│   ├── database_service.go
│   ├── query_service.go
│   ├── doc_service.go
│   ├── settings_service.go
│   ├── ai_service.go
│   ├── export_service.go
│   └── history_service.go
├── frontend/
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
│   │   └── lib/                     # 工具函数
│   └── package.json
└── build/
    └── appicon.png                  # 应用图标
```

## 开发

### 环境要求

- Go 1.23+
- Node.js 18+
- Wails CLI v2

### 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 安装依赖

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 安装 Go 依赖
go mod tidy
```

### 开发模式

```bash
wails dev
```

### 构建

```bash
wails build
```

构建产物位于 `build/bin/` 目录。

### 运行测试

```bash
go test ./internal/... -v
```

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
