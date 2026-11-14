# MiniDB 架构说明

本文说明包的分层、依赖规则，以及新增一种数据库要改哪些地方。规则由 `internal/archtest` 里的测试强制执行，违反时 `go test ./...` 会失败。

## 分层

```
storage / appdata  →  database  →  schemaindex / ai / agent  →  app  →  ui
          model、sqltext、logger 是被各层共用的底层包
```

| 包 | 职责 |
|---|---|
| `model` | 纯数据类型：`DBType`、`ConnectionConfig`、元数据结构、带错误码的 `model.Error`。不依赖任何内部包 |
| `sqltext` | SQL 文本处理：分句、转义（`QuoteString`）、语句动词判断。不依赖任何内部包 |
| `logger` / `appdata` | 日志、应用数据目录 |
| `storage` | SQLite 存储（`Store`）、密钥加密（`secrets.go`）、AI 会话。`legacy_bolt.go` 只用于升级旧数据，已标记废弃 |
| `database` | 连接管理 `Manager`、各数据库的方言 `Dialect`、元数据读取、查询与导入 SQL 构造 |
| `schemaindex` / `ai` / `agent` | 表结构索引缓存、AI 客户端、AI 工具调用 |
| `app` | 应用服务层（连接、查询、导出、设置、状态…），界面只通过它访问后端 |
| `ui` | mygo 界面。只依赖 `app`、`model`、`sqltext` 等；不允许直接依赖 `database` / `storage` / `keychain` / `sshtunnel`。需要的数据库类型和纯函数从 `app/database_facade.go`、`app/facade_misc.go` 取 |

## 依赖规则（`archtest/deps_test.go`）

- `allowedDeps` 是每个包允许依赖的内部包白名单，**只许收紧，不许放宽**；新增依赖要先在白名单里登记并写明理由。
- 检测到循环依赖会失败。
- `ui` 仍直接引用 `agent` 的事件模型（app 与界面之间的约定）和 `appdata` 的路径函数，白名单注释里记录了这笔债。
- 界面需要新的数据库能力时：先在 `app` 门面里加别名或转调，不要放开白名单。

## 其他由测试锁定的约束

| 测试 | 约束 |
|---|---|
| `TestNoDBTypeLiteralComparison` | 方言文件之外禁止用 `"postgres"` 这类字符串字面量比较数据库类型；用 `database.Postgres` 常量，更好的是查 `CapsOf(t)` |
| `TestFileSizeCeilings` | 单个源文件不超过 300 行；历史大文件登记了行数上限，只许变小。文件缩小后要同步下调 `fileLineCeilings` |
| `TestNoDiscardedStoreWrites` | 禁止 `_ = x.Put(...)` / `_ = x.Delete(...)`，写失败要么返回要么记日志 |

## 错误与多语言

- 后端错误用 `model.Errf(code, ...)` / `model.Wrapf(code, ...)`，带错误码。
- 界面通过 `errMessage` 把错误码翻译成当前语言：词条键是 `"err" + 首字母大写的错误码`，参数为 `{a0}`、`{a1}`。没有对应词条时退回 `err.Error()`。
- 词条文件 `locales/zh-Hans.json`、`locales/en.json` 要保持排序和 2 空格缩进；改完运行 `go tool mygo generate` 重新生成 `locales/catalog_gen.go`，并用 `go tool mygo i18n check` 校验。

## 敏感字段加密

- 连接密码、SSH 口令、AI 密钥落库前用 `Store.EncryptString` 加密，读取时用 `DecryptString`；没有 `enc:v1:` 前缀的旧明文原样返回，并在读取后顺手迁移成密文。
- 密钥来源通过 `storage.OpenStore(path, storage.WithKeyProvider(p))` 注入（`main.go` 里正式版用系统钥匙串，没指定时用数据目录里的密钥文件）。密钥属于各自的 `Store`，没有进程级全局状态，测试里互不影响。

## 界面状态

- 界面的持久化状态走 `app.UIStateService`（分桶：`settings`、`workspace_state`、`pins`），不要在界面里直接碰 `storage`。
- 自动保存：在 UI 线程里取快照 → 按内容哈希去重 → 交给串行的 `stateWriter` 写盘；退出时调用同步的 `SaveState()` 落盘。
- 异步加载用 `ui.loader[T]`，它带序号，丢弃过期的返回。
- 后台协程里不能捕获 `*ui.Context`（`mygo vet` 会拦），要刷新界面请用 `c.Services().Invalidate()`。

## 新增一种数据库

1. `internal/model/model.go`：新增 `DBType` 常量。
2. `internal/database/dialect_xxx.go`：实现 `Dialect` 接口，在 `init()` 里 `register(xxxDialect{})`。
   - `Caps()` 如实声明能力（能否多库、建库、外键、触发器、存储过程、TLS、用户管理…）；界面据此决定显示哪些功能，**不要在界面里写类型判断**。
   - 支持用户管理或进程列表时，额外实现 `UserAdmin` / `ProcessAdmin`。
   - `QuoteIdent`、`QuoteString` 按该方言的转义规则写，并补单元测试。
3. 如果用的是新的 `database/sql` 驱动，在 `manager.go` 里加空导入。
4. 元数据读取（`Tables`、`Columns`、`Indexes`、`DDL`…）都带 `ctx`，写完补一份类似 `metadata_sqlite_test.go` 的回归测试。能起真实实例的，参照 `integration_services_test.go` 加一项，并在 `.github/workflows/ci.yml` 的 `integration` 任务里加对应的服务容器。
5. 界面的类型选择、默认端口、默认用户会自动取自 `Dialect`；需要新图标时再改 `ui/conn_type.go`。
6. 运行完整验证：

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.27.4/libexec
go vet ./... && go tool mygo vet .
go tool mygo i18n check
go test ./...
```

## 测试

- 单元测试：`go test ./...`（界面测试约 8 秒）。
- 集成测试需要真实实例：`INTEGRATION_TEST=1` 加上 `MINIDB_IT_MYSQL_*` / `MINIDB_IT_PG_*`（`HOST`、`PORT`、`USER`、`PASSWORD`）环境变量，运行 `go test ./internal/database/ -run TestIntegrationServices`。CI 的 `integration` 任务用服务容器自动跑。
- TiDB、StarRocks 的集成测试需要自备实例，使用 `TABLEPLUS_AI_*` 环境变量，CI 不跑。
