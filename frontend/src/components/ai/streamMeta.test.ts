import { describe, expect, it } from "vitest";
import { createStreamMetaFilter, stripStreamMetaBlocks } from "./streamMeta";

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

  it("不应吞掉普通 SQL 代码块分段输出", () => {
    const filter = createStreamMetaFilter();
    expect(filter.flush("```sql\n")).toBe("```sql\n");
    expect(filter.flush("SELECT 1\n")).toBe("```sql\nSELECT 1\n");
    expect(filter.flush("```\n")).toBe("```sql\nSELECT 1\n```\n");
  });
});
