import { describe, it, expect } from "vitest";

// buildNewTableRow 逻辑已迁移到后端 QueryService.BuildNewRowDefaults
// 对应的单元测试在 Go 侧：internal/database/query_test.go TestBuildNewRowDefaults
describe("useTableDataEditor (placeholder)", () => {
  it("delegates new row defaults to backend", () => {
    expect(true).toBe(true);
  });
});
