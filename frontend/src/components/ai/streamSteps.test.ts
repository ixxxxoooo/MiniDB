import { describe, expect, it } from "vitest";
import { reduceAIStreamSteps, normalizeStreamThinkingContent, type AIStreamStep } from "./streamSteps";

function reduce(events: Array<{ type: string } & Record<string, any>>): AIStreamStep[] {
  return events.reduce<AIStreamStep[]>(
    (steps, event, index) => reduceAIStreamSteps(steps, event, 1000 + index),
    []
  );
}

describe("reduceAIStreamSteps", () => {
  it("keeps Cursor-style order from reasoning to tool, observation, answer, and done", () => {
    const steps = reduce([
      { type: "reasoning", delta: "先确认表结构。", sequence: 1 },
      { type: "tool_start", toolCallId: "call_1", toolName: "table_describe", toolInput: "{\"table\":\"users\"}", sequence: 2 },
      { type: "tool_result", toolCallId: "call_1", toolName: "table_describe", toolOutput: "id, name", durationMs: 12, sequence: 3 },
      { type: "answer_start", sequence: 4 },
      { type: "delta", delta: "结论", sequence: 5 },
      { type: "done", content: "结论", sequence: 6 },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["thinking", "tool", "observation", "answer"]);
    expect(steps[0]).toMatchObject({ kind: "thinking", state: "done" });
    expect(steps[1]).toMatchObject({ kind: "tool", state: "success", toolName: "table_describe" });
    expect(steps[2]).toMatchObject({ kind: "observation", state: "success", content: "id, name" });
    expect(steps[3]).toMatchObject({ kind: "answer", state: "done", content: "结论" });
  });

  it("deduplicates updates for the same tool call", () => {
    const steps = reduce([
      { type: "tool_args", toolCallId: "call_1", toolName: "sql_readonly_execute", toolInput: "{\"sql\":\"SELECT 1\"}", sequence: 2 },
      { type: "tool_start", toolCallId: "call_1", toolName: "sql_readonly_execute", sequence: 1 },
      { type: "tool_sql", toolCallId: "call_1", toolName: "sql_readonly_execute", toolSql: "SELECT 1", sequence: 3 },
      { type: "tool_result", toolCallId: "call_1", toolName: "sql_readonly_execute", toolOutput: "### 工具结果", sequence: 4 },
      { type: "tool_result", toolCallId: "call_1", toolName: "sql_readonly_execute", toolOutput: "### 工具结果更新", sequence: 5 },
    ]);

    expect(steps.filter((step) => step.kind === "tool")).toHaveLength(1);
    expect(steps.filter((step) => step.kind === "observation")).toHaveLength(1);
    expect(steps.find((step) => step.kind === "tool")).toMatchObject({
      toolInput: "{\"sql\":\"SELECT 1\"}",
      toolSql: "SELECT 1",
      state: "success",
      sequence: 1,
    });
    expect(steps.find((step) => step.kind === "observation")).toMatchObject({
      content: "### 工具结果更新",
    });
  });

  it("keeps an answer step after later process events", () => {
    const steps = reduce([
      { type: "answer_start", sequence: 10 },
      { type: "tool_start", toolCallId: "late", toolName: "table_stats", sequence: 2 },
      { type: "tool_result", toolCallId: "late", toolName: "table_stats", toolOutput: "ok", sequence: 3 },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["tool", "observation", "answer"]);
  });

  it("does not reclassify answer text when process events are interleaved", () => {
    const steps = reduce([
      { type: "tool_start", toolCallId: "first", toolName: "table_stats", sequence: 1 },
      { type: "tool_result", toolCallId: "first", toolName: "table_stats", toolOutput: "ok", sequence: 2 },
      { type: "answer_start", sequence: 3 },
      { type: "delta", delta: "先拿到基础统计。", sequence: 4 },
      { type: "tool_start", toolCallId: "second", toolName: "sql_readonly_execute", sequence: 5 },
      { type: "tool_result", toolCallId: "second", toolName: "sql_readonly_execute", toolOutput: "rows", sequence: 6 },
      { type: "answer_start", sequence: 7 },
      { type: "delta", delta: "继续补充明细。", sequence: 8 },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["tool", "observation", "answer", "tool", "observation", "answer"]);
    expect(steps.filter((step) => step.kind === "answer").map((step) => step.content)).toEqual([
      "先拿到基础统计。",
      "继续补充明细。",
    ]);
  });

  it("finishes running tool steps when done event arrives without tool_result", () => {
    const steps = reduce([
      { type: "tool_start", toolCallId: "call_x", toolName: "table_stats", sequence: 1 },
      { type: "done", content: "结果", sequence: 2 },
    ]);

    const toolStep = steps.find((s) => s.kind === "tool");
    expect(toolStep).toMatchObject({ state: "success" });
    const answerStep = steps.find((s) => s.kind === "answer");
    expect(answerStep).toMatchObject({ state: "done", content: "结果" });
  });

  it("shows status progress for models without reasoning", () => {
    const steps = reduce([
      { type: "status", delta: "loading_schema", sequence: 1 },
      { type: "status", delta: "waiting_model", sequence: 2 },
      { type: "answer_start", sequence: 3 },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["status", "status", "answer"]);
    expect(steps[0]).toMatchObject({ kind: "status", state: "done", status: "loading_schema" });
    expect(steps[1]).toMatchObject({ kind: "status", state: "running", status: "waiting_model" });
  });

  it("keeps introspective wording in the answer", () => {
    const steps = reduce([
      { type: "answer_start", sequence: 1 },
      {
        type: "delta",
        delta: "我需要先说明限制：当前只能基于已返回的数据判断。\n\n统计结果如下：\n- 共 3 条",
        sequence: 2,
      },
      {
        type: "done",
        content: "我需要先说明限制：当前只能基于已返回的数据判断。\n\n统计结果如下：\n- 共 3 条",
        sequence: 3,
      },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["answer"]);
    expect(steps[0]).toMatchObject({
      kind: "answer",
      content: "我需要先说明限制：当前只能基于已返回的数据判断。\n\n统计结果如下：\n- 共 3 条",
    });
  });

  it("keeps normal result headings in the answer", () => {
    const steps = reduce([
      { type: "answer_start", sequence: 1 },
      { type: "delta", delta: "**查询结果**\n\n统计结果如下：\n\n| metric | value |", sequence: 2 },
      { type: "done", content: "**查询结果**\n\n统计结果如下：\n\n| metric | value |", sequence: 3 },
    ]);

    expect(steps.map((step) => step.kind)).toEqual(["answer"]);
    expect(steps[0]).toMatchObject({ kind: "answer", content: "**查询结果**\n\n统计结果如下：\n\n| metric | value |" });
  });
});

describe("normalizeStreamThinkingContent", () => {
  it("removes DSML and meta protocol text from expandable thinking", () => {
    const output = normalizeStreamThinkingContent([
      "先分析",
      "< | DSML | tool_calls>",
      "< | DSML | invoke name=\"table_stats\">x</ | DSML | invoke>",
      "</ | DSML | tool_calls>",
      "```tableplus-ai-next-steps",
      "{\"choices\":[]}",
      "```",
    ].join("\n"));

    expect(output).toContain("先分析");
    expect(output).not.toContain("DSML");
    expect(output).not.toContain("tableplus-ai-next-steps");
  });

  it("strips <think> tags and preserves content", () => {
    const output = normalizeStreamThinkingContent("<think>这是推理过程</think>");
    expect(output).toBe("这是推理过程");
    expect(output).not.toContain("<think>");
  });

  it("preserves reasoning content as-is without template filtering", () => {
    const output = normalizeStreamThinkingContent("问题复述：用户需要查询数据\n分析：需要先获取表结构");
    expect(output).toContain("问题复述：");
    expect(output).toContain("分析：");
  });

  it("preserves markdown in reasoning content for the thinking block", () => {
    const output = normalizeStreamThinkingContent("**Determining report counts**\n\n- reports\n- conditions");
    expect(output).toBe("**Determining report counts**\n\n- reports\n- conditions");
  });
});
