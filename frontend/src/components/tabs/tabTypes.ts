import type { ColumnInfo } from "@/types/database";
import type { DatabaseDriver } from "@/types/connection";
import { GetColumnTypes } from "@/lib/wails/services/DatabaseService";

export type TableSubView = "data" | "structure" | "info" | "doc";

export interface EditingStructureCol extends ColumnInfo {
  __status?: "new" | "modified" | "deleted";
  __uid: string;
}

export interface EditingIndexRow {
  __uid: string;
  __status?: "new" | "deleted";
  name: string;
  type: string;
  isUnique: boolean;
  columns: string[];
  isPrimary?: boolean;
}

export interface StructureColDef {
  key: string;
  label: string;
  editable: boolean;
  minWidth: number;
  isTypeSelect?: boolean;
  isCheckbox?: boolean;
}

// 从后端获取指定连接支持的字段类型列表
export async function fetchColumnTypes(connID: string): Promise<string[]> {
  try {
    return await GetColumnTypes(connID);
  } catch {
    return [];
  }
}

// 兼容旧调用的同步版本（作为 fetchColumnTypes 未就绪时的回退默认值）
const FALLBACK_TYPES: string[] = [
  "int", "bigint", "varchar", "text", "datetime", "timestamp",
  "decimal", "float", "double", "boolean", "json", "blob",
];

export function getDataTypesFallback(): string[] {
  return FALLBACK_TYPES;
}

export const STRUCTURE_COL_DEFS: StructureColDef[] = [
  { key: "name", label: "column_name", editable: true, minWidth: 140 },
  { key: "type", label: "data_type", editable: true, minWidth: 120, isTypeSelect: true },
  { key: "characterSet", label: "character_set", editable: false, minWidth: 100 },
  { key: "collation", label: "collation", editable: false, minWidth: 130 },
  { key: "nullable", label: "is_nullable", editable: true, minWidth: 80, isCheckbox: true },
  { key: "defaultValue", label: "column_default", editable: true, minWidth: 110 },
  { key: "extra", label: "extra", editable: false, minWidth: 100 },
  { key: "foreignKey", label: "foreign_key", editable: false, minWidth: 110 },
  { key: "comment", label: "comment", editable: true, minWidth: 140 },
];

export const INDEX_COL_DEFS = [
  { key: "name", label: "index_name", minWidth: 160 },
  { key: "type", label: "index_algorithm", minWidth: 120 },
  { key: "isUnique", label: "is_unique", minWidth: 90 },
  { key: "columns", label: "column_name", minWidth: 220 },
] as const;
