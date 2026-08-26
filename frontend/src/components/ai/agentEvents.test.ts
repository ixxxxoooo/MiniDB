import { describe, it, expect } from "vitest";
import {
  reduceAgentStream,
  toTimelineEntry,
  agentEventToLegacy,
  type AgentEvent,
} from "@/components/ai/agentEvents";

const ev = (type: AgentEvent["type"], seq: number, payload: any): AgentEvent => ({
  v: 2,
  runId: "run-1",
  seq,
  type,
  payload,
});

describe("reduceAgentStream", () => {
  it("收敛 thinking/tool/answer/done 时间线", () => {
    const entries = [
      toTimelineEntry(ev("run.started", 1, { model: "gpt-4o", schemaMode: "summary" })),
      toTimelineEntry(ev("step.thinking", 2, "先看表结构")),
      toTimelineEntry(ev("tool.requested", 3, { callId: "c1", toolName: "table_describe", arguments: { table_names: ["users"] } })),
      toTimelineEntry(ev("tool.result", 4, {
        callId: "c1",
        toolName: "table_describe",
        ok: true,
        kind: "text",
        text: "users 字段...",
        durationMs: 12,
      })),
      toTimelineEntry(ev("answer.delta", 5, "结果：")),
      toTimelineEntry(ev("answer.delta", 6, "500 行")),
      toTimelineEntry(ev("run.done", 7, { content: "结果：500 行", rounds: 1, suggestions: [{ label: "看看详情", prompt: "查看详情" }] })),
    ];
    const reduced = reduceAgentStream(entries);
    expect(reduced.started?.model).toBe("gpt-4o");
    expect(reduced.thinking).toEqual(["先看表结构"]);
    expect(reduced.tools).toHaveLength(1);
    expect(reduced.tools[0].toolName).toBe("table_describe");
    expect(reduced.answer).toBe("结果：500 行");
    expect(reduced.done?.suggestions).toHaveLength(1);
  });

  it("乱序 seq 也能正确排序", () => {
    const entries = [
      toTimelineEntry(ev("answer.delta", 10, "B")),
      toTimelineEntry(ev("answer.delta", 5, "A")),
    ];
    const reduced = reduceAgentStream(entries);
    expect(reduced.answer).toBe("AB");
  });

  it("rows 工具结果保留结构化载荷", () => {
    const rows = [{ id: 1, name: "a" }];
    const entries = [toTimelineEntry(ev("tool.result", 1, {
      callId: "c2",
      toolName: "sql_readonly_execute",
      ok: true,
      kind: "rows",
      columns: ["id", "name"],
      rows,
      truncated: false,
      durationMs: 5,
    }))];
    const reduced = reduceAgentStream(entries);
    expect(reduced.tools[0].kind).toBe("rows");
    expect(reduced.tools[0].rows).toEqual(rows);
  });

  it("error 事件暴露错误信息", () => {
    const entries = [toTimelineEntry(ev("run.error", 1, { code: "stream_failed", message: "网络错误" }))];
    const reduced = reduceAgentStream(entries);
    expect(reduced.error?.message).toBe("网络错误");
  });
});

describe("agentEventToLegacy", () => {
  it("tool.result rows 映射为非错误 tool_result", () => {
    const legacy = agentEventToLegacy(ev("tool.result", 1, {
      callId: "c1",
      toolName: "sql_readonly_execute",
      ok: true,
      kind: "rows",
      columns: ["id"],
      rows: [{ id: 1 }],
    }));
    expect(legacy?.type).toBe("tool_result");
    expect(legacy?.toolCallId).toBe("c1");
  });

  it("error kind 映射为 tool_error", () => {
    const legacy = agentEventToLegacy(ev("tool.result", 1, {
      callId: "c1",
      toolName: "sql_readonly_execute",
      ok: false,
      kind: "error",
      errorCode: "guarded",
    }));
    expect(legacy?.type).toBe("tool_error");
  });

  it("未知类型返回 null", () => {
    expect(agentEventToLegacy({ v: 2, runId: "r", seq: 1, type: "approval.requested", payload: {} })).toBeNull();
  });
});