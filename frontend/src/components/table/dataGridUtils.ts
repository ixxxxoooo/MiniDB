import type { ColumnMeta, ColumnInfo } from "@/types/database";

// ====== 常量 ======

export const MIN_COL_WIDTH = 80;
export const MAX_COL_WIDTH = 400;
export const ROW_NUMBER_COL_MIN_WIDTH = 42;
export const ROW_NUMBER_COL_EMPTY_WIDTH = 60;
export const HEADER_PADDING = 32;
export const CELL_PADDING = 24;
export const SAMPLE_ROWS = 50;

export const NULL_SENTINEL = "__TPAI_NULL__";
export const NOW_SENTINEL = "__TPAI_NOW__";
export const MAX_CELL_TEXT_RENDER = 512;

// ====== 类型定义 ======

export type EditorKind = "text" | "date" | "time" | "datetime" | "enum";

export interface ResolvedColumnMeta {
  kind: EditorKind;
  nullable: boolean;
  type: string;
  enumOptions: string[];
  defaultValue: string | null;
}

export interface EditorDropdownItem {
  label: string;
  value: string;
  action: "set" | "manual" | "null" | "now" | "default";
}

// ====== 列宽计算与缓存 ======

// 自动计算的列宽缓存（不含用户手动拖拽）
export const autoWidthCache = new Map<string, number>();
// 用户手动拖拽的列宽（优先级最高）
export const manualWidthCache = new Map<string, number>();

const HEADER_FONT = "600 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";
const CELL_FONT = "400 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";

let _measureCanvas: HTMLCanvasElement | null = null;
function getMeasureCanvas(): CanvasRenderingContext2D {
  if (!_measureCanvas) {
    _measureCanvas = document.createElement("canvas");
  }
  return _measureCanvas.getContext("2d")!;
}

export function measureTextWidth(text: string, font: string): number {
  const ctx = getMeasureCanvas();
  ctx.font = font;
  return ctx.measureText(text).width;
}

export function getTypeWidthConstraints(colType: string): { min: number; max: number; fixed?: number } {
  const t = colType.toLowerCase();

  if (t.includes("bool") || t === "bit" || t === "bit(1)") {
    return { min: 60, max: 80, fixed: 60 };
  }
  if (t.includes("datetime") || t.includes("timestamp")) {
    return { min: 160, max: 200 };
  }
  if (t.includes("date") && !t.includes("datetime")) {
    return { min: 100, max: 120, fixed: 100 };
  }
  if (t.includes("time") && !t.includes("timestamp") && !t.includes("datetime")) {
    return { min: 90, max: 110, fixed: 90 };
  }
  if (t.includes("int") || t.includes("serial")) {
    return { min: 80, max: 120 };
  }
  if (t.includes("decimal") || t.includes("numeric") || t.includes("float") || t.includes("double") || t.includes("real")) {
    return { min: 90, max: 150 };
  }
  if (t.includes("json")) {
    return { min: 200, max: 400 };
  }
  if (t.includes("text") || t.includes("blob") || t.includes("clob") || t.includes("bytea")) {
    return { min: 120, max: 300 };
  }
  if (t.includes("varchar") || t.includes("char") || t.includes("string")) {
    return { min: 120, max: 300 };
  }
  if (t.includes("uuid") || t.includes("guid")) {
    return { min: 260, max: 300 };
  }
  if (t.includes("enum") || t.includes("set")) {
    return { min: 80, max: 200 };
  }
  return { min: MIN_COL_WIDTH, max: MAX_COL_WIDTH };
}

export function getCacheKey(database: string, table: string, column: string): string {
  return `${database}:${table}:${column}`;
}

export function computeColumnWidth(col: ColumnMeta, data: Record<string, unknown>[]): number {
  const constraints = getTypeWidthConstraints(col.type);
  if (constraints.fixed) return constraints.fixed;

  let maxWidth = measureTextWidth(col.name, HEADER_FONT) + HEADER_PADDING;

  const sampleData = data.slice(0, SAMPLE_ROWS);
  for (const row of sampleData) {
    const val = row[col.name];
    if (val === null || val === undefined) {
      maxWidth = Math.max(maxWidth, measureTextWidth("NULL", CELL_FONT) + CELL_PADDING);
    } else {
      const text = String(val);
      const display = text.length > 100 ? text.substring(0, 100) : text;
      maxWidth = Math.max(maxWidth, measureTextWidth(display, CELL_FONT) + CELL_PADDING);
    }
  }

  maxWidth = Math.max(constraints.min, Math.min(constraints.max, maxWidth));
  maxWidth = Math.max(MIN_COL_WIDTH, Math.min(MAX_COL_WIDTH, maxWidth));

  return Math.ceil(maxWidth);
}

export function autoFitColumnWidth(
  col: ColumnMeta,
  data: Record<string, unknown>[],
  database: string,
  table: string,
): number {
  const cacheKey = getCacheKey(database, table, col.name);
  manualWidthCache.delete(cacheKey);
  const w = computeColumnWidth(col, data);
  autoWidthCache.set(cacheKey, w);
  return w;
}

// ====== 编辑器类型推断 ======

export function normalizeType(raw: string | undefined): string {
  return (raw || "").trim().toLowerCase();
}

export function resolveEditorKind(type: string, enumOptions: string[]): EditorKind {
  const t = normalizeType(type);
  if (enumOptions.length > 0) return "enum";
  if (t.includes("datetime") || t.includes("timestamp")) return "datetime";
  if (t.includes("date") && !t.includes("datetime") && !t.includes("timestamp")) return "date";
  if (t.includes("time") && !t.includes("datetime") && !t.includes("timestamp")) return "time";
  return "text";
}

// ====== 时间格式化 ======

function pad2(v: number): string {
  return String(v).padStart(2, "0");
}

export function getNowByKind(kind: EditorKind): string {
  const now = new Date();
  const date = `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}`;
  const time = `${pad2(now.getHours())}:${pad2(now.getMinutes())}:${pad2(now.getSeconds())}`;
  if (kind === "date") return date;
  if (kind === "time") return time;
  return `${date}T${time}`;
}

export function normalizeDisplayDateValue(value: unknown, kind: EditorKind): string {
  if (value === null || value === undefined || value === "") return "";
  const s = String(value).trim();
  if (kind === "datetime" || kind === "date") {
    if (/^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}/.test(s)) {
      return s.slice(0, 19).replace(" ", "T");
    }
    if (/^\d{4}-\d{2}-\d{2}$/.test(s)) {
      return s;
    }
  }
  if (kind === "time") {
    if (/^\d{2}:\d{2}(:\d{2})?$/.test(s)) return s;
  }
  return s;
}

export function normalizeDefaultValue(rawDefault: string | null): string | null {
  if (rawDefault === null || rawDefault === undefined) return null;
  const trimmed = String(rawDefault).trim();
  if (!trimmed) return null;
  const dequoted = trimmed.replace(/^['"]|['"]$/g, "");
  if (/^null$/i.test(dequoted)) return NULL_SENTINEL;
  if (/^(current_(timestamp|date|time)(\(\d*\))?|local(timestamp|time)(\(\d*\))?|now\(\))$/i.test(dequoted)) return NOW_SENTINEL;
  return dequoted;
}

// ====== 列元数据解析 ======

export function resolveColumnMetaMap(
  columns: ColumnMeta[],
  columnInfoMap: Map<string, ColumnInfo>,
): Record<string, ResolvedColumnMeta> {
  const map: Record<string, ResolvedColumnMeta> = {};
  for (const col of columns) {
    const info = columnInfoMap.get(col.name);
    const type = info?.type || col.type || "";
    const enumOptions = info?.enumOptions || [];
    map[col.name] = {
      kind: resolveEditorKind(type, enumOptions),
      nullable: info?.nullable ?? col.nullable ?? true,
      type,
      enumOptions,
      defaultValue: info?.defaultValue ?? null,
    };
  }
  return map;
}
