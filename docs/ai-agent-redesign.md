# MiniDB AI 助手重设计：面向 Agent 最佳实践的架构蓝图

> 状态：设计稿 v1（待评审）
> 范围：前端 AIPanel / 后端 AIService + AI tools + 协议层
> 目标：把当前「手写 ReAct 循环 + 面条式回调 + 模型正文内嵌协议」的实现，重构成「
> 可测试、可观测、可扩展、安全可护栏」的 Agent 执行器架构。

---

## 1. 现状体检（基于代码逐行核实）

### 1.1 后端调用链

```
AIPanel (前端)
  └─ ChatAIStream(connID, dbName, messages, requestID, sessionID)   [Wails 绑定]
       └─ services/ai_service.go
            ├─ ReloadConfig()                每次调用重读 bbolt + AES 解密（已加指纹缓存）
            ├─ shouldSendFullSchema()        会话维度决定首轮发完整 DDL 还是摘要
            ├─ loadSchemaContext()           经 schemaindex 拉取/构建全量 schema
            ├─ buildChatSystemPrompt()       巨型内联 system prompt（含工具散文列表 + 协议说明）
            ├─ buildToolExecutor()           闭包捕获连接上下文
            └─ client.ChatWithToolsStreamRealtime()   internal/ai/client.go ReAct 循环
                 ├─ Responses API 优先，Chat Completions 回退（熔断 10min）
                 ├─ ParallelToolCalls=false，每轮单工具
                 ├─ maxRounds=10 硬上限
                 └─ 工具结果以「文本 Markdown」回灌给模型
  └─ EventsOn("ai:chat_stream")             事件流（自定义、无版本、单通道）
```

### 1.2 前端 AIPanel.tsx（2262 行单体）

- 会话/消息全部存 **localStorage**（隐私 + 体积 + 无服务端镜像）
- 用 `EventsOn("ai:chat_stream")` 收事件，在客户端用 `reduceAIStreamSteps` 重建时间线
- 模型回答里嵌 `minidb-next-steps` / `minidb-meta` / DSML 元数据块 → 前端正则剥离再解析（脆弱协议）
- `@tool:` / `@table:` 提及、工具/表联想、SQL 片段高亮、mermaid 懒加载、自定义 Markdown 表格解析
- `dangerouslySetInnerHTML`（prism 已转义 + mermaid 用 DOMPurify 白名单，风险低但应统一收口）

### 1.3 现状缺陷清单（按严重度）

#### P0 — 架构性缺陷（本次重设计要解决的核心）

| # | 问题 | 证据 | 业界对照 |
|---|------|------|----------|
| A1 | **没有 Agent 执行器抽象**：整个 ReAct 循环是 client.go 里一个带回调的大函数；无 run 生命周期、无状态机、无法单测 | `internal/ai/client.go` `ResponsesReActClient.Run` | Agent SDK / LangGraph 都有独立 Executor + 状态机 + 可注入钩子 |
| A2 | **协议靠「模型在正文里吐结构化块」**：`minidb-next-steps`、`minidb-meta`、DSML 都要靠前端正则剥离，模型输出格式漂移就坏 | `ai_service.go:883`、`streamMeta.ts` 整文件 | 结构化输出走独立通道（tool / response_format / 事件消息），绝不与展示文本混合 |
| A3 | **工具结果是 Markdown 文本**：`sql_readonly_execute` 返回的是拼好的字符串（"### 工具 xxx 结果\n- 字段..."），前端无法原生渲染数据表，模型还要浪费 token 重新解析 | `ai_tools.go` 各 exec 函数 | 工具应返回**结构化结果**（rows/columns/error），前端直接渲染 |
| A4 | **工具注册表三份拷贝**：前端 ListTools、LLM JSON Schema、system prompt 散文列表各写一遍，增删工具极易漏改 | `ai_tools.go:64/120`、`ai_service.go:836` | 单一事实来源注册表，schema/列表/prompt 自动生成 |
| A5 | **无会话/记忆管理**：会话只在 localStorage；后端每请求无状态；上下文只是"取最后 12 条消息"硬截断，无 token 预算、无摘要压缩 | `AIPanel.tsx:290`、`trimContextMessages` | 服务端会话存储 + 滑窗/摘要 + token 记账（OpenAI Agents SDK 的 memory 概念） |
| A6 | **无护栏纵深**：只做动词白名单（select/show/desc/explain/with），`SELECT ... INTO OUTFILE`、`SLEEP()`、大 cross join、无语句超时均未拦截；`MaxTokens`/`temperature` 配置项形同虚设（从不传给请求） | `sql_policy.go:103`、`client.go:20-25`（仅参与指纹、从不使用） | 语句分类处理器（非前缀匹配）+ 危险函数黑名单 + 超时/行数/字节护栏 + 只读账号建议 |
| A7 | **无可观测性**：无 run trace、无 token/耗时/成本上报，多轮循环出问题只能靠日志 | 全链路 | 每 run 一条 trace + usage 事件 |

#### P1 — 质量与健壮性

| # | 问题 | 证据 |
|---|------|------|
| B1 | `Minidb-meta` 自动执行链路整段是死代码（注释自述"主流程已不再调用"） | `ai_auto_execute.go`、`sql_policy.go` 的意图正则 |
| B2 | 事件协议无版本号、类型为松散字符串，前端 reducer 用大量 `as any` | `AIPanel.tsx:889`、`ChatStreamEvent` |
| B3 | 工具结果回灌模型前无长度/截断合约，长结果可能撑爆上下文 | `ai_tools.go` 输出拼接 |
| B4 | `shouldSendFullSchema` 用会话消息数启发式，个体 schema 命中即长期发摘要，用户改了表结构后首轮可能发旧摘要 | `ai_service.go:286` |
| B5 | 前端单体 2262 行：状态、流式、渲染、提及、联想全在一处，后续维护成本高 | `AIPanel.tsx` |

---

## 2. 业界最佳实践参照（设计依据）

以下模式来自 OpenAI Agents SDK、LangGraph、Anthropic "Building effective agents"、Vercel AI SDK、MCP 工具协议及 Text-to-SQL guardrail 论文共识：

1. **Agent = 执行器 + 工具 + 记忆 + 护栏，四者解耦**。模型只负责"决策"，执行器负责"生命周期"。
2. **事件驱动的流式协议**：`run.started → step.delta → tool.call → tool.result → answer.delta → run.done(usage)`，每个事件带 runID + 序号 + 类型版本；前端是纯消费者。
3. **工具是头等公民**：单一注册表（名称/描述/JSON Schema/处理器/timeout/readOnly/结果类型），LLM schema、前端列表、prompt 均自动派生；**工具返回结构化数据**，展示与模型消费分离。
4. **结构化输出走独立通道**，绝不嵌入展示文本；前端拿到的就是"可以直接渲染的东西"。
5. **记忆分层**：短期 = 本次 run 上下文（token 记账）；中期 = 会话滑窗 + 摘要压缩；长期 = 持久化会话 + schema 索引（检索相关表注入，而非全量 DDL）。
6. **护栏纵深**：输入校验 → 语句分类处理器 → 危险调用黑名单 → 超时/行数/字节上限 → 只读账号/副本库；**每条都独立可测**。
7. **人类在环**：任何非只读或高成本动作，先发 `approval_requested` 事件，等用户确认。
8. **可观测性**：每 run 一个 trace id，记录每轮 token、工具耗时、成本、错误分类。

---

## 3. 目标架构（MiniDB Agent v2）

### 3.1 分层总览

```
┌─────────────────────────────────────────────────────────────┐
│ 前端 (AIPanel 重构)                                          │
│  StreamClientHook ← 类型化事件流 (runID+seq+version)          │
│  MessageList / ToolCallCard / ResultTable / ThinkingBlock    │
└──────────────────────────────┬──────────────────────────────┘
                               │ Wails 绑定 / 事件
┌──────────────────────────────▼──────────────────────────────┐
│ 服务层 services/agent                                        │
│  ┌──────────────┐ ┌─────────────┐ ┌────────────────────────┐ │
│  │ AgentExecutor│ │ ToolRegistry│ │ SessionStore (bbolt)   │ │
│  │ 状态机+预算   │ │ 唯一事实来源 │ │ 滑窗/摘要/token 记账    │ │
│  └──────┬───────┘ └──────┬──────┘ └───────────┬────────────┘ │
│         │ emit(Event)    │ invoke(Tool)       │ 记忆         │
│  ┌──────▼───────────────▼────────────────────────▼─────────┐ │
│  │ Guardrails: SQLClassifier / DangerFuncs / Timeout / Rows │ │
│  └──────┬──────────────────────────────────────────────────┘ │
└─────────┼────────────────────────────────────────────────────┘
          ▼
│ internal/ai: LLM client (Responses/原 Chat 回退, 现在只在决策层调用) │
│ internal/database / schemaindex: 数据与检索底座                    │
```

### 3.2 核心抽象（Go）

```go
// 事件协议 v2（跨前后端共享，放 internal/agent/events.go，前端用 TS 镜像一份）
type AgentEventType string
const (
    EventRunStarted     AgentEventType = "run.started"     // payload: runID, model, schemaMode
    EventStepThinking   AgentEventType = "step.thinking"   // payload: delta（流式推理）
    EventToolRequested  AgentEventType = "tool.requested"  // payload: toolName, args, callID
    EventToolResult     AgentEventType = "tool.result"     // payload: callID, OK|ERR, structured result, durationMs
    EventAnswerDelta    AgentEventType = "answer.delta"    // payload: delta
    EventRunDone        AgentEventType = "run.done"        // payload: content, usage{tokens,cost}
    EventRunError       AgentEventType = "run.error"       // payload: code, message
    EventApproveRequest AgentEventType = "approval.requested" // payload: action, meta
)

type AgentEvent struct {
    Version int         `json:"v"`           // 协议版本
    RunID   string      `json:"runId"`
    Seq     int64       `json:"seq"`
    Type    AgentEventType `json:"type"`
    Payload any         `json:"payload"`
}

// 工具注册表（唯一事实来源）
type Tool struct {
    Name     string          // 稳定 ID，前端 @tool 联想也用它
    Description string
    Parameters map[string]any // JSON Schema
    Handler  ToolHandler     // func(ctx, ToolInput) ToolResult
    ReadOnly bool
    Timeout  time.Duration
    ResultKind string        // "rows" | "text" | "error" | "approval"
}

type ToolResult struct {
    OK      bool              `json:"ok"`
    Kind    string            `json:"kind"`        // rows/text
    Columns []string          `json:"columns,omitempty"`
    Rows    []map[string]any  `json:"rows,omitempty"`
    Text    string            `json:"text,omitempty"`
    Truncated bool            `json:"truncated,omitempty"`  // 行数/字节被截断
    DurationMs int64          `json:"durationMs"`
    ErrorCode string          `json:"errorCode,omitempty"`
}

// 执行器状态机
type AgentExecutor struct {
    runID  string
    budget RunBudget       // MaxRounds, MaxTokens, MaxCost, Timeout
    tools  *ToolRegistry
    guard  GuardrailChain
    stream EventSink        // 可注入（Wails Event / 测试 recorder）
    memory Memory           // 会话滑窗 + 摘要
}
```

### 3.3 执行器循环（可单测）

```
Execute(runCtx, userMsg):
  1. run.started
  2. context := memory.Compact(sysPrompt, userMsg, schemaCtx)   // token 记账
  3. loop round in 1..budget.MaxRounds:
       resp = llm.Step(ctx, context, tools.Schemas())
       if resp.Thinking: emit step.thinking
       if resp.ToolCalls:
         for each call:
           emit tool.requested
           if guard.Denies(call): emit tool.result{ERR, errorCode:"guarded"} ; continue
           if !tool.ReadOnly: emit approval.requested; 等待 approve/deny
           result = tools.Invoke(ctx, call)         // 自带 timeout/concurrency/cancel
           emit tool.result(结构化)
           context.Append(tool_message(result))
       else:
         emit answer.delta...; emit run.done{usage}; return
  4. 预算耗尽 → 触发无工具总结轮 → run.done（带 notice）
```

关键点：
- **循环与 LLM 解耦**：`llm.Step` 是注入的接口，测试用 fake LLM 驱动状态机走完整分支。
- **事件协议与 UI 解耦**：前端只看事件；协议加版本号，演进不破坏旧客户端。
- **工具返回结构化**：`rows` 工具直接给列+行，前端渲染 `<table>`；模型拿到的则是紧凑文本视图（由执行器 `FormatResult` 派生），两者互不干扰。
- **预算与护栏是执行器的一部分**：可注入、可单测、可在上线后调参。

### 3.4 记忆与会话

- **服务端会话存储**（bbolt 新 bucket `ai_sessions`）：runID、消息、工具轨迹、usage。前端 localStorage 仅做 UI 缓存；重开应用可恢复。
- **上下文记账**：估算每轮 token（含工具结果），超出预算时：先丢弃工具原始结果（仅留摘要）→ 再滑窗 → 最后触发 LLM 摘要压缩轮（总结旧对话为一段）。
- **Schema 注入**：保留 schemaindex，但首轮不再全量 DDL：按关键词/表提及检索 Top-N 相关表 DDL + 全量表摘要（现状已是此结构，重设计把"发完整 DDL 的时机"改为**由工具驱动**：模型需要某表结构时，让它显式调 `table_describe`，而不是预注入）。

### 3.5 护栏链（GuardrailChain，顺序可测）

1. `InputSanitizer`：校验 tool 参数符合 JSON Schema，非法参数直接返回结构性错误（不给模型"再猜一次"的机会，避免幻觉参数）。
2. `SQLClassifier`：**词法处理器而非动词前缀**——拒绝多语句、尾随注释后隐藏语句、危险函数（`SLEEP`/`PG_SLEEP`/`LOAD_FILE`/`into outfile`/`dumpfile`/`shutdown`/`bench` 等）、跨库写、`SELECT...FOR UPDATE`。
3. `RowLimit`：结果行/字节上限，超限截断并在结果里标 `truncated`（模型与 UI 都能感知）。
4. `StatementTimeout`：每条 DB 语句独立 `context.WithTimeout`（可配置，默认如 30s）。
5. `ReadOnlyAccount` **（建议项，非本次代码改动）**：生产建议给 AI 专用只读数据库账号，从通道层杜绝写。
6. `ApprovalGate`：非只读工具（未来可能引入）先发 `approval.requested`，等前端确认。

### 3.6 前端重构（AIPanel 拆分）

```
components/ai/
  agentEvents.ts          // 协议类型镜像 + 事件 reducer(纯函数, 可测)
  useAgentStream.ts       // Hook：发起 run、订阅事件、维护 timeline、取消/重试
  AgentPanel.tsx          // 布局 + 会话列表（数据来自服务端, localStorage 仅缓存）
  AgentMessageList.tsx
  AgentMessage.tsx        // 按 content-block 渲染: 思考/工具调用/表格结果/正文
  ToolResultTable.tsx     // rows/columns → 原生表格, 支持截断提示
  NextSteps.tsx           // 建议按钮: 数据来自 run.done.payload.suggestions（结构化, 不再正则剥离）
  CodeBlock.tsx           // prism 高亮收口（统一 sanitize）
```

- `run.done` 的 payload 直接带 `suggestions`，前端渲染按钮，**删除 `stripStreamMetaBlocks` / `extractNextStepMetaChoices` 整条链**。
- 工具结果按 `kind` 渲染：`rows` → 表格；`text` → 文本；`error` → 错误卡片（含 errorCode 供 i18n）。
- 停止 = 发 `CancelChatStream(runID)`（保留），执行器向上取消 `llm.Step` 与工具 ctx。

---

## 4. 分阶段实施计划

> 每阶段独立可上线、可回滚；阶段间不破坏现有协议（前端先兼容新旧事件）。

### Phase 0 — 协议与地基（最小可验证）
- [ ] 新增 `internal/agent` 包：事件协议 v2、ToolRegistry、ToolResult、EventSink（适配 Wails）
- [ ] 把现有 10 个工具迁入注册表（Handler 不变，输出从"Markdown 字符串"改为 `ToolResult{rows?|text?}` + 截断标记）；`ListTools`/`BuildAllToolDefinitions` 改为注册表派生
- [ ] 前端新增 `useAgentStream` + `agentEvents.ts`（纯 reducer），`ChatAIStream` 改为并行发 v2 事件（旧事件保留一段时间双写，前端切新协议后关掉旧）
**验收**：AI 对话功能等价，工具结果前端可原生渲染表格；协议有版本号；工具输出有截断标记。

### Phase 1 — 执行器与护栏
- [ ] `AgentExecutor` 状态机落地（llm.Step 接口化；fake LLM 单测覆盖：工具循环、预算耗尽、护栏拒绝、取消）
- [ ] `GuardrailChain`：SQL 词法分类器、危险函数黑名单、语句超时、行数/字节上限
- [ ] 移除 dead code：`ai_auto_execute.go`、`sql_policy.go` 意图正则、`streamMeta.ts` 剥离逻辑
- [ ] 预算记账（MaxRounds/MaxTokens）+ `MaxTokens`/`temperature` 真正传给 LLM
**验收**：护栏单测全绿；`SELECT ... INTO OUTFILE`/`SLEEP()` 被拒；超长结果截断标注；前端无 `as any` 事件处理。

### Phase 2 — 记忆与服务端会话
- [ ] bbolt `ai_sessions`：persist run/messages/usage；前端会话列表接服务端
- [ ] 滑窗 + 摘要压缩 + token 记账（内存内先行，持久化后置）
- [ ] schema 注入策略调整：完整 DDL 由 `table_describe` 工具按需拉取（首轮只给表摘要）
**验收**：重开应用会话可恢复；300 条消息长会话不爆上下文；schema 上下文体积下降。

### Phase 3 — 前端收尾
- [ ] AIPanel 拆分为上述组件树；删除所有元数据块剥离/DSML 处理
- [ ] suggestions 按钮数据来自 `run.done.payload`
- [ ] 统一 sanitize 收口；无 `dangerouslySetInnerHTML` 散落
**验收**：UI 与现状等价或更好；删除代码行数 > 新增（净减复杂度）。

---

## 5. 需要你拍板的决策点

| # | 决策 | 选项 | 我的建议 |
|---|------|------|----------|
| D1 | 实施范围 | ① 按 Phase 0→3 全部实施 ② 先做 Phase 0+1（协议/执行器/护栏） ③ 只出设计不做代码 | **②**：Phase 0+1 是收益最大且自洽的一批 |
| D2 | 会话存储 | 服务端 bbolt（推荐） vs 维持 localStorage | 服务端（A5 核心） |
| D3 | 非只读工具未来纳入？（如 AI 发起 UPDATE/DELETE 需审批） | 纳入（需 ApprovalGate） vs 永远只读 | 先保持只读，Gate 预留接口 |
| D4 | `Minimized maxRounds` 预算默认 | 10（现状） vs 更小（如 6）+ 预算后自动总结 | 6 + 预算兜底 |