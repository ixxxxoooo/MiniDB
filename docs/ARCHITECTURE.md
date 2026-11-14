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
- `ui` 的白名单只有 `app`、`logger`、`model`、`sqltext`、`version`、`locales`，**不再**直接依赖 `agent` 和 `appdata`：Agent 事件模型经 `app/facade_agent.go` 的类型别名暴露，数据目录和日志目录经 `app.DataDir()` / `app.LogDir()` 取得。
- 界面需要新的数据库能力时：先在 `app` 门面里加别名或转调，不要放开白名单。

## 其他由测试锁定的约束

| 测试 | 约束 |
|---|---|
| `TestNoDBTypeLiteralComparison` | 方言文件之外禁止用 `"postgres"` 这类字符串字面量比较数据库类型；用 `database.Postgres` 常量，更好的是查 `CapsOf(t)` |
| `TestNoDBTypeBranchOutsideDatabase` | `database` 包之外禁止用 `database.Postgres` / `model.Postgres` 这类常量或 `MySQLCompatible()` 判断类型；差异要么是 `Caps` 的一项能力，要么是方言的方法。只有 `ui/dbicons.go`、`ui/welcome.go` 这两张「类型 → 图标/颜色」的展示表例外（`uiTypeConstTables`） |
| `TestDatabaseTypeConstsOnlyInDialectFiles` | `database` 包自己也一样：类型常量只许出现在 `dialect_*.go`、`sqlgen_*.go` 和 `model_alias.go`。通用的构造函数（`BuildXxxSQL`）要按类型做不同的事，就加一个可选接口或一项 `Caps` |
| `TestFileSizeCeilings` | 单个源文件不超过 300 行；历史大文件登记了行数上限，只许变小。文件缩小后要同步下调 `fileLineCeilings` |
| `TestServiceMethodsAreTimed` | `app` 里每个 `*XxxService` 的导出方法必须调用 `logger.Perf`；纯内存读取、只设字段的登记在 `untimedServiceMethods` 并写明理由 |
| `TestUIGoroutinesAreRegistered` | `ui` 里不许有未登记的 `go` 语句：后台工作用 `up.bg`（自动打点），登记的几处在 `uiGoroutineSites` 里写明了理由 |
| `TestNoDiscardedStoreWrites` | 禁止 `_ = x.Put(...)` / `_ = x.Delete(...)`，写失败要么返回要么记日志 |

## 性能日志

`logger` 包除了日志还提供性能打点（`Perf`、`PerfUI`、`PerfDone`、`SQLText`）：超预算才写 `[PERF]` 行，按名字累计并每 5 分钟汇总，界面线程卡死时把所有协程堆栈写进日志。界面线程的帧构建、回调排队、`up.bg` 都已自动打点，数据库会话、连接、服务方法、后台作业、AI 轮次也都有。读法见 [PERFORMANCE.md](PERFORMANCE.md)，新增代码的要求见仓库根目录 `CLAUDE.md`。打点放在 `logger` 而不是单独的包，是因为各层本来就都依赖它，不需要改依赖白名单。

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
   - `Dialect` 由四个小接口组成：`Identity`（类型、名字、默认值、能力）、`Quoter`（标识符与字符串转义）、`Connector`（驱动名与 DSN）、`MetadataReader`（库、表、列、索引、DDL、统计）。外键、触发器、函数与存储过程是可选接口 `ForeignKeyReader`、`TriggerReader`、`RoutineReader`，只有声明了对应 `Caps` 才需要实现，没有的不用写空方法。
   - `Caps()` 如实声明能力（能否多库、建库、外键、触发器、存储过程、TLS、用户管理…）；界面据此决定显示哪些功能，**不要在界面里写类型判断**。
   - 可选接口按需实现：`UserAdmin`（用户管理）、`ProcessAdmin`（进程列表与终止）、`Explainer`（执行计划）、`BulkForeignKeys`（ER 图一次读出整库外键）、`Monitor`（监控采样与慢查询；对应 `Caps.Monitor`）、`SyntaxProvider`（占位符、分页、转文本、取零行、计数的写法与通用的 MySQL 写法不同，如 SQL Server 的 `@p1` 与 `OFFSET/FETCH`、ClickHouse 的 `toString`）。
   - 生成 DDL / 写入语句时和 MySQL 通用写法不同的地方，实现 `sqlgen.go` 里对应的可选接口（`TableRenamer`、`TableTruncater`、`ImportConflictWriter`、`CreateTriggerWriter`、`CreateColumnBuilder`、`StructureAlterer`、`IndexSQLWriter` …），写在 `sqlgen_xxx.go` 或自己的 `dialect_xxx*.go` 里；不实现就用通用写法。`mysqlDialect` 已经实现了 MySQL 系的写法，TiDB、MariaDB 直接继承，StarRocks 只覆盖不同的几个。
   - 调用方需要的「这种库是不是 X」一律做成 `Caps` 的一项：是不是本地文件（`LocalFile()`）、字符串里反斜杠是不是转义符（`BackslashEscape`）、只读执行靠不靠 `PRAGMA query_only`（`QueryOnlyPragma`）、能用哪一类外部备份工具（`Dump`）。需要文字的，用可选接口：给 AI 的语法提示是 `PromptHinter`（`dialect_hints.go`）。
   - 数据网格是否允许编辑由 `Caps.RowEdit` 决定（ClickHouse 为 false，界面只读并提示）；新增类型不要在界面里按类型名写判断。
   - 每个连接需要独立数据库上下文的引擎（ClickHouse）声明 `PerDatabasePool`，元数据查询用 `currentDatabase()` 而不是依赖连接串。
   - 元数据和结果值要先转成基础类型再交给通用扫描代码（见 `query_scan_value.go` 的 `normalizeOtherValue`），否则驱动特有的类型会在网格里显示成乱码。
   - `QuoteIdent`、`QuoteString` 按该方言的转义规则写，并补单元测试。
3. 如果用的是新的 `database/sql` 驱动，在 `manager.go` 里加空导入。连接的建立（SSH 隧道、DSN、连接池参数、Ping）都收在 `manager_pool.go` 的 `dial` / `openPool`，`Connect`、`Reconnect`、`TestConnection` 共用，不要在别处再写一份。
4. 元数据读取（`Tables`、`Columns`、`Indexes`、`DDL`…）都带 `ctx`，写完补一份类似 `metadata_sqlite_test.go` 的回归测试。能起真实实例的，参照 `integration_services_test.go` 加一项，并在 `.github/workflows/ci.yml` 的 `integration` 任务里加对应的服务容器。
5. 界面的类型选择、默认端口、默认用户会自动取自 `Dialect`；需要新图标时再改 `ui/conn_type.go`。
6. 运行完整验证：

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.27.4/libexec
go vet ./... && go tool mygo vet .
go tool mygo i18n check
go test ./...
```

## AI 服务商

- `ai.Config.Provider` 选择协议：空或 `openai` 走 OpenAI 兼容通道（Responses 优先，网关不支持时回退 Chat Completions 并熔断一段时间），`anthropic` 走原生 Messages API。所有服务商差异都收在 `ai/provider.go` 的 `chatProvider` 接口里，工具循环的停止规则只有 `runToolLoop` 这一份，新增协议只需实现 `toolLoop` 传输层。
- 界面里的服务商预设放在 `app/ai_presets.go`（地址、默认模型、必需请求头、是否允许不填密钥、协议）；`AIConfigured` 是判断「AI 是否可用」的唯一规则，本地服务（预设允许或地址在本机、内网）不填密钥也算已配置。预设带来的请求头仍和密钥一起加密保存。

## 二进制体积

新增 SQL Server（go-mssqldb）、ClickHouse（clickhouse-go v2）、Excel（excelize）等驱动和界面功能后，`CGO_ENABLED=0 go build -ldflags "-s -w"` 得到的 arm64 可执行文件从约 27.6 MB 增至约 34.4 MB（约 +6.8 MB，+25%）。这个数字没有按依赖逐项拆分；如后续要压缩体积，先用 `go tool nm -size` 之类的工具量出各驱动的占比再决定。

## 测试

- 单元测试：`go test ./...`（界面测试约 8 秒）。
- 集成测试需要真实实例：`INTEGRATION_TEST=1` 加上 `MINIDB_IT_MYSQL_*` / `MINIDB_IT_PG_*`（`HOST`、`PORT`、`USER`、`PASSWORD`）环境变量，运行 `go test ./internal/database/ -run TestIntegrationServices`。CI 的 `integration` 任务用服务容器自动跑。
- 执行计划：`TestIntegrationExplainMySQL`（同样用 `INTEGRATION_TEST=1` + `MINIDB_IT_MYSQL_*`）对真实 MySQL 跑普通 EXPLAIN 与 ANALYZE。MySQL 新版默认返回 v2 JSON（`query_plan`），8.x 默认老格式（`query_block`），解析器两种都认，JSON 都认不出时降级到 `FORMAT=TREE`。解析测试用的真实输出放在 `internal/database/testdata/`，文件名带格式版本。
- TiDB、StarRocks 的集成测试需要自备实例，使用 `TABLEPLUS_AI_*` 环境变量，CI 不跑。
