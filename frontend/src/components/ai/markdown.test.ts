import { describe, expect, it } from "vitest";
import { normalizeAIMarkdown } from "@/components/ai/markdown";

describe("normalizeAIMarkdown", () => {
  it("normalizes malformed pipe tables into GFM-friendly tables", () => {
    const input = [
      "### 列表报表部分（名称示例）",
      "| 报表 | 负责人 | 创建人 | 是否公开 ||",
      "--- | --- | --- | --- |",
      "| 报表A | huwenang | haihaiang huwen | ✅ 公开 ||",
    ].join("\n");

    expect(normalizeAIMarkdown(input)).toContain(
      [
        "### 列表报表部分（名称示例）",
        "",
        "| 报表 | 负责人 | 创建人 | 是否公开 |",
        "| --- | --- | --- | --- |",
        "| 报表A | huwenang | haihaiang huwen | ✅ 公开 |",
      ].join("\n")
    );
  });

  it("preserves fenced code blocks", () => {
    const input = [
      "```sql",
      "select 'a|b' as value;",
      "```",
    ].join("\n");

    expect(normalizeAIMarkdown(input)).toBe(input);
  });

  it("does not turn ordinary pipe text into a table", () => {
    const input = [
      "状态 A | 状态 B",
      "今天 | 明天",
    ].join("\n");

    expect(normalizeAIMarkdown(input)).toBe(input);
  });
});
