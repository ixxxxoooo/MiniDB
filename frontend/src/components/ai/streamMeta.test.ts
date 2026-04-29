import { describe, expect, it } from "vitest";
import { createStreamMetaFilter, extractNextStepMetaChoices, stripStreamMetaBlocks } from "./streamMeta";

describe("stripStreamMetaBlocks", () => {
  it("应移除完整 meta 块并保留正文与 SQL", () => {
    const input = [
      "```tableplus-ai-meta",
      '{"autoExecute":{"enabled":true,"mode":"first_sql_readonly","reason":"user_requested_result"}}',
      "```",
      "这里是说明",
      "```sql",
      "SELECT * FROM users;",
      "```",
    ].join("\n");

    const output = stripStreamMetaBlocks(input);
    expect(output).not.toContain("tableplus-ai-meta");
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
      "```tableplus-ai-next-steps",
      '{"choices":[{"label":"看角色分布","prompt":"继续：查看角色分布"}]}',
      "```",
      "这是正文末尾",
    ].join("\n");
    const output = stripStreamMetaBlocks(input);
    expect(output).toContain("分析完成，给你下一步建议：");
    expect(output).toContain("这是正文末尾");
    expect(output).not.toContain("tableplus-ai-next-steps");
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

describe("createStreamMetaFilter", () => {
  it("应在分段流式过程中吞掉未闭合的 meta JSON", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("先输出一段正常说明\n")).toBe("先输出一段正常说明\n");
    expect(filter.flush("```tableplus-ai-meta\n")).toBe("先输出一段正常说明\n");
    expect(filter.flush('{"autoExecute":{"enabled":true')).toBe("先输出一段正常说明\n");
    expect(filter.flush(',"mode":"first_sql_readonly"}}')).toBe("先输出一段正常说明\n");
    expect(filter.flush("\n```\n这里是最终正文")).toBe("先输出一段正常说明\n\n这里是最终正文");
  });

  it("应在 meta 开头被拆分时也不闪现半截前缀", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("前置说明\n``")).toBe("前置说明\n");
    expect(filter.flush("`tablepl")).toBe("前置说明\n");
    expect(filter.flush("us-ai-meta\n")).toBe("前置说明\n");
    expect(filter.flush('{"autoExecute":{"enabled":true}}')).toBe("前置说明\n");
    expect(filter.flush("\n```\n正文来了")).toBe("前置说明\n\n正文来了");
  });

  it("应在 next-steps meta 分段流式时持续隐藏", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("先给说明\n")).toBe("先给说明\n");
    expect(filter.flush("```tableplus-ai-next-steps\n")).toBe("先给说明\n");
    expect(filter.flush('{"choices":[{"label":"A","prompt":"继续A"}]}')).toBe("先给说明\n");
    expect(filter.flush("\n```\n最后结论")).toBe("先给说明\n\n最后结论");
  });

  it("不应吞掉普通 SQL 代码块分段输出", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("```sql\n")).toBe("```sql\n");
    expect(filter.flush("SELECT 1\n")).toBe("```sql\nSELECT 1\n");
    expect(filter.flush("```\n")).toBe("```sql\nSELECT 1\n```\n");
  });

  it("应在 DSML 块被拆分流式时持续隐藏协议文本", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("先看数据\n< | DSML | function_calls>\n")).toBe("先看数据\n");
    expect(filter.flush("< | DSML | invoke name=\"sql_readonly_execute\">\n")).toBe("先看数据\n");
    expect(filter.flush("< | DSML | parameter name=\"sql\" string=\"true\">SELECT 1</ | DSML | parameter>\n")).toBe("先看数据\n");
    expect(filter.flush("</ | DSML | invoke>\n</ | DSML | function_calls>\n最终结论")).toBe("先看数据\n\n最终结论");
  });

  it("应在全角 DSML 块被拆分流式时持续隐藏协议文本", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("趋势分析\n<｜DSML｜function_calls>\n")).toBe("趋势分析\n");
    expect(filter.flush("<｜DSML｜invoke name=\"sql_readonly_execute\">\n")).toBe("趋势分析\n");
    expect(filter.flush("<｜DSML｜parameter name=\"sql\" string=\"true\">SELECT 1</｜DSML｜parameter>\n")).toBe("趋势分析\n");
    expect(filter.flush("</｜DSML｜invoke>\n</｜DSML｜function_calls>\n最终结论")).toBe("趋势分析\n\n最终结论");
  });

  it("应在 tool_calls 变体被拆分流式时持续隐藏协议文本", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("先看结果\n< | DSML | tool_calls>\n")).toBe("先看结果\n");
    expect(filter.flush('["tblA","tblB"]')).toBe("先看结果\n");
    expect(filter.flush("</ | DSML | parameter>\n最终结论")).toBe("先看结果\n\n最终结论");
  });
});

describe("extractNextStepMetaChoices", () => {
  it("应提取 next-steps 结构化选项", () => {
    const input = [
      "正文",
      "```tableplus-ai-next-steps",
      '{"choices":[{"label":"看角色分布","prompt":"继续：看角色分布"},{"label":"看登录用户","prompt":"继续：看登录用户"}]}',
      "```",
    ].join("\n");
    const choices = extractNextStepMetaChoices(input);
    expect(choices.length).toBe(2);
    expect(choices[0]).toEqual({ label: "看角色分布", prompt: "继续：看角色分布" });
  });

});
