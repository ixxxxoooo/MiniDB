import { describe, expect, it } from "vitest";
import { stripStreamMetaBlocks } from "./streamMeta";

describe("stripStreamMetaBlocks", () => {
  it("应移除完整 meta 块并保留正文与 SQL", () => {
    const input = [
      "```minidb-meta",
      '{"autoExecute":{"enabled":true,"mode":"first_sql_readonly","reason":"user_requested_result"}}',
      "```",
      "这里是说明",
      "```sql",
      "SELECT * FROM users;",
      "```",
    ].join("\n");

    const output = stripStreamMetaBlocks(input);
    expect(output).not.toContain("minidb-meta");
    expect(output).toContain("这里是说明");
    expect(output).toContain("SELECT * FROM users;");
  });

  it("不应误删普通 SQL 代码块", () => {
    const input = [
      "这里是查询",
      "```sql",
      "SELECT id, name FROM users;",
      "```",
    ].join("\n");

    const output = stripStreamMetaBlocks(input);
    expect(output).toBe(input);
  });

  it("应移除 next-steps meta 块", () => {
    const input = [
      "分析完成，给你下一步建议：",
      "```minidb-next-steps",
      '{"choices":[{"label":"看角色分布","prompt":"继续：查看角色分布"}]}',
      "```",
      "这是正文末尾",
    ].join("\n");
    const output = stripStreamMetaBlocks(input);
    expect(output).toContain("分析完成，给你下一步建议：");
    expect(output).toContain("这是正文末尾");
    expect(output).not.toContain("minidb-next-steps");
  });

  it("应移除 DSML function_calls 协议块", () => {
    const input = [
      "让我查看一下样本：",
      "< | DSML | function_calls>",
      "< | DSML | invoke name=\"sql_readonly_execute\">",
      "< | DSML | parameter name=\"sql\" string=\"true\">SELECT * FROM products LIMIT 10</ | DSML | parameter>",
      "</ | DSML | invoke>",
      "</ | DSML | function_calls>",
      "以下是结论",
    ].join("\n");

    const output = stripStreamMetaBlocks(input);
    expect(output).toContain("让我查看一下样本：");
    expect(output).toContain("以下是结论");
    expect(output).not.toContain("DSML");
    expect(output).not.toContain("function_calls");
  });

  it("应移除全角分隔符 DSML 协议块", () => {
    const input = [
      "查看时间趋势分析：",
      "<｜DSML｜function_calls>",
      "<｜DSML｜invoke name=\"sql_readonly_execute\">",
      "<｜DSML｜parameter name=\"sql\" string=\"true\">SELECT 1</｜DSML｜parameter>",
      "</｜DSML｜invoke>",
      "</｜DSML｜function_calls>",
      "这是最终结果",
    ].join("\n");
    const output = stripStreamMetaBlocks(input);
    expect(output).toContain("查看时间趋势分析：");
    expect(output).toContain("这是最终结果");
    expect(output).not.toContain("function_calls");
    expect(output).not.toContain("sql_readonly_execute");
  });

  it("应移除 tool_calls 变体和残留 parameter 标签", () => {
    const input = [
      "已拿到工具结果，继续推理…",
      "让我分批获取更多统计信息。< | DSML | tool_calls>",
      '["tblBwDataSourceAlarm","tblBwDataSourceAlarmHistory"]</ | DSML | parameter>',
    ].join("\n");
    const output = stripStreamMetaBlocks(input);
    expect(output).toContain("已拿到工具结果，继续推理…");
    expect(output).not.toContain("DSML");
    expect(output).not.toContain("tool_calls");
    expect(output).not.toContain("parameter");
    expect(output).not.toContain("tblBwDataSourceAlarm");
  });

  it("不应误删普通 JSON 内容", () => {
    const input = [
      "示例 JSON:",
      '{"name":"demo","sql":"SELECT 1"}',
    ].join("\n");
    expect(stripStreamMetaBlocks(input)).toBe(input);
  });
});
