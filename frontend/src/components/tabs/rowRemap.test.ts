import { describe, expect, it } from "vitest";
import { makeRowRemap, remapEditedCells, remapIndexSet } from "./rowRemap";

describe("rowRemap", () => {
  it("shifts surviving indexes past every removed row", () => {
    const remap = makeRowRemap(new Set([5, 7]));
    expect([0, 4, 5, 6, 7, 8, 9].map(remap)).toEqual([0, 4, -1, 5, -1, 6, 7]);
  });

  it("is independent of removal order", () => {
    expect(makeRowRemap(new Set([7, 5]))(8)).toBe(6);
  });

  it("remaps index sets and drops removed ones", () => {
    const remap = makeRowRemap(new Set([2, 3]));
    expect(Array.from(remapIndexSet(new Set([1, 2, 4, 6]), remap)).sort()).toEqual([1, 2, 4]);
  });

  it("remaps edited cell keys, keeping colons inside column names", () => {
    const remap = makeRowRemap(new Set([1]));
    const out = remapEditedCells({ "0:a": 1, "1:a": 2, "3:b:c": 3 }, remap);
    expect(out).toEqual({ "0:a": 1, "2:b:c": 3 });
  });
});
