# 性能日志与卡顿排查

界面卡顿时，先看日志，不要凭感觉猜。日志里的 `[PERF]` 行能告诉你卡在哪一层、哪一处。开发时怎么加打点见 `CLAUDE.md` 的「性能日志」；API 和预算表在 `internal/logger/perf*.go`。

## 日志在哪

应用数据目录下的 `logs/<日期>.log`，macOS 上正式版是 `~/Library/Application Support/MiniDB/logs/`，开发版（`mygo dev`）是 `MiniDB Dev` 目录。保留 14 天，同时输出到终端。

```bash
grep -E "\[PERF\]" ~/Library/Application\ Support/MiniDB/logs/$(date +%F).log | tail -50
```

## 三种 `[PERF]` 行

默认只在出问题时写，平时不刷屏。

```text
[PERF] 慢 db.statement 耗时=3.10s 预算=2.00s conn=c1 db=bench_big rows=200 sql=SELECT * FROM audit_logs WHERE actor = '?'
[PERF] 卡死 UI 线程停在 ui.post.Workspace.loadTables 已 412.0ms（阈值 300.0ms），所有协程堆栈如下…
[PERF] 汇总 近 5m0s，按总耗时排序（次数 / 总计 / 平均 / 最大 / 超预算次数）
    bg.Workspace.loadTables      600   18.00s   30.0ms   30.0ms  0
    运行时 协程=34 堆=212MB 系统=290MB GC=18次 暂停合计=41.2ms
```

- **慢**：某一次超过了它的预算（预算表在 `perf_budget.go`，每个前缀一个数，有理由）。
- **卡死**：界面线程在同一段逻辑里停了超过 300ms，日志里附带所有协程的堆栈，最上面通常就是界面线程，能看到它卡在哪一行、在等什么锁或网络。同一次卡死只报一次。
- **汇总**：每 5 分钟和退出时各一次，按总耗时排序。平时没人觉得慢但总耗时很大的，就是该优化的地方。底下一行是运行时：频繁 GC 或很长的暂停本身就会让界面卡。

SQL 里的字符串字面量一律替换成 `'?'` 并截断到 160 个字符，不会把密码或个人数据写进日志。

## 名字的含义

名字是 `层.模块.动作`，从上到下对应一次操作经过的层：

| 名字 | 含义 |
| --- | --- |
| `ui.build` | 整帧的构建。立即模式下点击处理也在里面，超过 33ms 就能看出掉帧 |
| `ui.build.<段>` | 帧里的一段：`toolbar`、`sidebar`、`workspace`、`tab.table` / `tab.query` …、`grid`、`assistant`、`console`、`modals` |
| `ui.queue` | 回调从投递到真正执行等了多久。长说明界面线程一直忙，去找同一时刻的 `ui.post.*` / `ui.done.*` |
| `ui.post.<函数>` / `ui.done.<函数>` | 在界面线程上执行的回调；`ui.done` 是后台任务完成后的收尾 |
| `bg.<函数>` | `up.bg` 的后台部分，名字取自发起它的函数，自动打点 |
| `app.<服务>.<方法>` | 应用服务的方法，如 `app.Queries.ExecuteSQL`。外层方法包含内层，两个都慢时先看最里面的那个 |
| `db.statement` | 一条语句从发出到读完结果，**带脱敏后的 SQL** |
| `db.exec` / `db.query` | 会话里的一次往返；`db.query` 只量到拿到结果集，逐行读取的时间不在内 |
| `db.connect` / `.ssh` / `.ping` / `.pool` | 建连各阶段：SSH 隧道、握手认证、访问 PG 另一个库时现建连接池 |
| `db.ping` | 保活探测 |
| `schema.refresh` | 表结构索引刷新，要读整库元数据，大库上最容易慢 |
| `ai.round` / `ai.tool` | AI 的一轮模型请求 / 一次工具调用（工具里会查库，同时看 `db.*`） |
| `job.export` / `job.import` / `job.dump.*` | 长作业整体耗时，不判超时，只进汇总 |
| `app.bus.<主题>` | 总线订阅回调，应该立即返回，超过 50ms 就是违反约定 |

## 卡顿了怎么查

1. 先搜「卡死」。有的话看堆栈最上面的界面线程在干什么，这是最直接的证据。
2. 搜「慢 ui.build」。有 `ui.build.<段>` 的子项就知道是哪一块重（数据网格行列多时先怀疑 `grid`）。
3. `ui.queue` 很大：界面线程被占着，找同一时间段的 `ui.post.*` / `ui.done.*` 慢项。
4. 界面线程本身不慢，只是等数据：看 `bg.*` → `app.*` → `db.*`，从外往里找到最里面那个慢的。`db.statement` 带 SQL，`db.connect.*` 能看出慢在建连的哪一步。
5. 看汇总最后一行：堆很大、GC 次数多或暂停合计长，是内存问题。
6. 还不够细时用 `MINIDB_PERF=verbose` 重启复现，所有 ≥1ms 的打点都会写出来。

## 环境变量

| 变量 | 作用 |
| --- | --- |
| `MINIDB_PERF=verbose` | 记录所有 ≥1ms 的打点（默认只记超预算的） |
| `MINIDB_PERF=off` | 关闭全部打点 |
| `MINIDB_PERF_BUDGET="db.=300ms,ui.build=20ms"` | 临时调整某些前缀的预算 |
| `MINIDB_PERF_STALL=500ms` | 调整「卡死」的阈值，默认 300ms |
| `MYGO_FRAME_STATS=16` | 打开框架自带的逐帧统计（超过 16ms 的帧，含布局、绘制、GC 的分段），输出会带 `[MYGO]` 前缀进同一个日志。要在启动前设置 |

## 注意

- 打点很便宜（两次取时间加一次加锁累计），但不要放进循环、逐行、逐单元格的路径。
- 预算是经验值。用户自己写的查询几百毫秒很常见，所以 `db.statement` 是 2 秒；换了机器或数据量觉得太吵，用环境变量临时调，确认合理再改 `perf_budget.go`。
