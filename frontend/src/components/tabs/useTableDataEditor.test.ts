import { describe, expect, it } from "vitest";
import type { ColumnInfo, ColumnMeta } from "@/types/database";
import { buildNewTableRow } from "./newRowDefaults";

function columnInfo(overrides: Partial<ColumnInfo>): ColumnInfo {
  return {
    name: "",
    type: "",
    nullable: true,
    defaultValue: null,
    isPrimary: false,
    isAutoIncrement: false,
    comment: "",
    maxLength: null,
    characterSet: "",
    collation: "",
    extra: "",
    foreignKey: "",
    ...overrides,
  };
}

describe("buildNewTableRow", () => {
  it("prefills non-null temporal columns with the current local time", () => {
    const now = new Date(2026, 4, 7, 9, 8, 7);
    const columns: ColumnMeta[] = [
      { name: "id", type: "int", nullable: false },
      { name: "created_at", type: "datetime", nullable: false },
      { name: "updated_at", type: "timestamp", nullable: false },
      { name: "memo_date", type: "date", nullable: true },
    ];
    const structureColumns: ColumnInfo[] = [
      columnInfo({ name: "id", type: "int", nullable: false, isPrimary: true, isAutoIncrement: true }),
      columnInfo({ name: "created_at", type: "datetime", nullable: false }),
      columnInfo({ name: "updated_at", type: "timestamp", nullable: false }),
      columnInfo({ name: "memo_date", type: "date", nullable: true }),
    ];

    expect(buildNewTableRow(columns, structureColumns, now)).toEqual({
      id: null,
      created_at: "2026-05-07 09:08:07",
      updated_at: "2026-05-07 09:08:07",
      memo_date: null,
    });
  });

  it("prefills temporal columns whose database default is current time", () => {
    const now = new Date(2026, 4, 7, 9, 8, 7);
    const columns: ColumnMeta[] = [
      { name: "publish_date", type: "date", nullable: true },
      { name: "start_time", type: "time", nullable: true },
      { name: "seen_at", type: "timestamp", nullable: true },
    ];
    const structureColumns: ColumnInfo[] = [
      columnInfo({ name: "publish_date", type: "date", defaultValue: "CURRENT_DATE" }),
      columnInfo({ name: "start_time", type: "time", defaultValue: "CURRENT_TIME(3)" }),
      columnInfo({ name: "seen_at", type: "timestamp", defaultValue: "now()" }),
    ];

    expect(buildNewTableRow(columns, structureColumns, now)).toEqual({
      publish_date: "2026-05-07",
      start_time: "09:08:07",
      seen_at: "2026-05-07 09:08:07",
    });
  });
});
