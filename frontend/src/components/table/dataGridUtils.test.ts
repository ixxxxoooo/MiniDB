import { describe, expect, it } from "vitest";
import { normalizeDisplayDateValue } from "./dataGridUtils";

describe("normalizeDisplayDateValue", () => {
  it("datetime 统一为空格分隔并补齐秒", () => {
    expect(normalizeDisplayDateValue("2024-03-05 10:20:30.123456", "datetime")).toBe("2024-03-05 10:20:30");
    expect(normalizeDisplayDateValue("2024-03-05T10:20:30Z", "datetime")).toBe("2024-03-05 10:20:30");
    expect(normalizeDisplayDateValue("2024-03-05 10:20", "datetime")).toBe("2024-03-05 10:20:00");
  });

  it("date 只保留日期部分", () => {
    expect(normalizeDisplayDateValue("2024-03-05 10:20:30", "date")).toBe("2024-03-05");
    expect(normalizeDisplayDateValue("2024-03-05", "date")).toBe("2024-03-05");
  });

  it("time 取时分秒", () => {
    expect(normalizeDisplayDateValue("10:20:30.5", "time")).toBe("10:20:30");
    expect(normalizeDisplayDateValue("10:20", "time")).toBe("10:20");
  });

  it("空值返回空串", () => {
    expect(normalizeDisplayDateValue(null, "datetime")).toBe("");
    expect(normalizeDisplayDateValue("", "date")).toBe("");
  });
});
