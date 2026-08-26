import { describe, it, expect } from "vitest";
import { agentEventToLegacy, reduceAgentStream, toTimelineEntry } from "@/components/ai/agentEvents";
import type { AgentEvent } from "@/components/ai/agentEvents";

const ev = (type: AgentEvent["type"], seq: number, payload: any): AgentEvent => ({
  v: 2,
  runId: "run-1",
  seq,
  type,
  payload,
});

describe("tool.result 结构化管线", () => {
  it("rows 结果经 legacy 映射保留 columns/rows/truncated", () => {
    const rows = [{ id: 1, name: "a" }, { id: 2, name: "b" }];
    const legacy = agentEventToLegacy(ev("tool.result", 3, {
      callId: "c1",
      toolName: "sql_readonly_execute",
      ok: true,
      kind: "rows",
      columns: ["id", "name"],
      rows,
      truncated: true,
      durationMs: 3,
    }));
    expect(legacy?.columns).toEqual(["id", "name"]);
    expect(legacy?.rows).toEqual(rows);
    expect(legacy?.truncated).toBe(true);
  });

  it("reducer 汇总 rows 载荷", () => {
    const entries = [toTimelineEntry(ev("tool.result", 1, {
      callId: "c1",
      toolName: "table_sample",
      ok: true,
      kind: "rows",
      columns: ["id"],
      rows: [{ id: 1 }],
      truncated: false,
    }))];
    const reduced = reduceAgentStream(entries);
    expect(reduced.tools[0].columns).toEqual(["id"]);
    expect(reduced.tools[0].rows).toHaveLength(1);
  });

  it("error 结果不携带 rows", () => {
    const legacy = agentEventToLegacy(ev("tool.result", 1, {
      callId: "c1",
      toolName: "sql_readonly_execute",
      ok: false,
      kind: "error",
      errorCode: "guarded",
    }));
    expect(legacy?.type).toBe("tool_error");
    expect(legacy?.columns).toBeUndefined();
  });
});