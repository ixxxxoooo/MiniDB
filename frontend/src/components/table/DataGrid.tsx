import React, { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type OnChangeFn,
  type SortingState,
} from "@tanstack/react-table";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, ChevronDown } from "lucide-react";
import { useTranslation } from "@/i18n";
import type { ColumnMeta, ColumnInfo } from "@/types/database";

// ====== 列宽计算与缓存 ======

const MIN_COL_WIDTH = 80;
const MAX_COL_WIDTH = 400;
const ROW_NUMBER_COL_MIN_WIDTH = 42;
const ROW_NUMBER_COL_EMPTY_WIDTH = 60;
const HEADER_PADDING = 32;
const CELL_PADDING = 24;
const SAMPLE_ROWS = 50;

// 自动计算的列宽缓存（不含用户手动拖拽）
const autoWidthCache = new Map<string, number>();
// 用户手动拖拽的列宽（优先级最高）
const manualWidthCache = new Map<string, number>();

const NULL_SENTINEL = "__TPAI_NULL__";
const NOW_SENTINEL = "__TPAI_NOW__";
const MAX_CELL_TEXT_RENDER = 512;

type EditorKind = "text" | "date" | "time" | "datetime" | "enum";

interface ResolvedColumnMeta {
  kind: EditorKind;
  nullable: boolean;
  type: string;
  enumOptions: string[];
  defaultValue: string | null;
}

interface EditorDropdownItem {
  label: string;
  value: string;
  action: "set" | "manual" | "null" | "now" | "default";
}

let _measureCanvas: HTMLCanvasElement | null = null;
function getMeasureCanvas(): CanvasRenderingContext2D {
  if (!_measureCanvas) {
    _measureCanvas = document.createElement("canvas");
  }
  return _measureCanvas.getContext("2d")!;
}

function measureTextWidth(text: string, font: string): number {
  const ctx = getMeasureCanvas();
  ctx.font = font;
  return ctx.measureText(text).width;
}

// 根据数据库字段类型返回列宽约束
function getTypeWidthConstraints(colType: string): { min: number; max: number; fixed?: number } {
  const t = colType.toLowerCase();

  // boolean 类型：固定 60px
  if (t.includes("bool") || t === "bit" || t === "bit(1)") {
    return { min: 60, max: 80, fixed: 60 };
  }
  // datetime / timestamp：160~200px
  if (t.includes("datetime") || t.includes("timestamp")) {
    return { min: 160, max: 200 };
  }
  // date（非 datetime）：固定 100
  if (t.includes("date") && !t.includes("datetime")) {
    return { min: 100, max: 120, fixed: 100 };
  }
  // time（非 datetime/timestamp）：固定 90
  if (t.includes("time") && !t.includes("timestamp") && !t.includes("datetime")) {
    return { min: 90, max: 110, fixed: 90 };
  }
  // 整数：80~120
  if (t.includes("int") || t.includes("serial")) {
    return { min: 80, max: 120 };
  }
  // 浮点/精确数值：90~150
  if (t.includes("decimal") || t.includes("numeric") || t.includes("float") || t.includes("double") || t.includes("real")) {
    return { min: 90, max: 150 };
  }
  // JSON / JSONB：300+
  if (t.includes("json")) {
    return { min: 200, max: 400 };
  }
  // 大文本 / blob
  if (t.includes("text") || t.includes("blob") || t.includes("clob") || t.includes("bytea")) {
    return { min: 120, max: 300 };
  }
  // varchar / char：120~300
  if (t.includes("varchar") || t.includes("char") || t.includes("string")) {
    return { min: 120, max: 300 };
  }
  // UUID
  if (t.includes("uuid") || t.includes("guid")) {
    return { min: 260, max: 300 };
  }
  // enum / set
  if (t.includes("enum") || t.includes("set")) {
    return { min: 80, max: 200 };
  }
  return { min: MIN_COL_WIDTH, max: MAX_COL_WIDTH };
}

const HEADER_FONT = "600 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";
const CELL_FONT = "400 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";

// 基于 canvas measureText 计算列宽，采样前 N 行
function computeColumnWidth(
  col: ColumnMeta,
  data: Record<string, unknown>[],
): number {
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
      // 截断超长文本，避免 measureText 开销过大
      const display = text.length > 100 ? text.substring(0, 100) : text;
      maxWidth = Math.max(maxWidth, measureTextWidth(display, CELL_FONT) + CELL_PADDING);
    }
  }

  // 先 clamp 到类型约束，再 clamp 到全局极限
  maxWidth = Math.max(constraints.min, Math.min(constraints.max, maxWidth));
  maxWidth = Math.max(MIN_COL_WIDTH, Math.min(MAX_COL_WIDTH, maxWidth));

  return Math.ceil(maxWidth);
}

// 对单列执行 auto-fit：忽略手动缓存，基于数据重新计算
function autoFitColumnWidth(
  col: ColumnMeta,
  data: Record<string, unknown>[],
  database: string,
  table: string,
): number {
  const cacheKey = getCacheKey(database, table, col.name);
  // 清除手动拖拽缓存，恢复自动计算
  manualWidthCache.delete(cacheKey);
  const w = computeColumnWidth(col, data);
  autoWidthCache.set(cacheKey, w);
  return w;
}

function getCacheKey(database: string, table: string, column: string): string {
  return `${database}:${table}:${column}`;
}

function normalizeType(raw: string | undefined): string {
  return (raw || "").trim().toLowerCase();
}

function parseEnumOptions(colType: string): string[] {
  const match = colType.match(/enum\s*\((.*)\)/i);
  if (!match) return [];
  const inner = match[1];
  const result: string[] = [];
  const re = /'((?:[^'\\]|\\.)*)'/g;
  let m: RegExpExecArray | null = null;
  while ((m = re.exec(inner)) !== null) {
    result.push(m[1].replace(/\\'/g, "'").replace(/\\\\/g, "\\"));
  }
  return result;
}

function resolveEditorKind(type: string, enumOptions: string[]): EditorKind {
  const t = normalizeType(type);
  if (enumOptions.length > 0) return "enum";
  if (t.includes("datetime") || t.includes("timestamp")) return "datetime";
  if (t.includes("date") && !t.includes("datetime") && !t.includes("timestamp")) return "date";
  if (t.includes("time") && !t.includes("datetime") && !t.includes("timestamp")) return "time";
  return "text";
}

function pad2(v: number): string {
  return String(v).padStart(2, "0");
}

function getNowByKind(kind: EditorKind): string {
  const now = new Date();
  const date = `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}`;
  const time = `${pad2(now.getHours())}:${pad2(now.getMinutes())}:${pad2(now.getSeconds())}`;
  if (kind === "date") return date;
  if (kind === "time") return time;
  return `${date}T${time}`;
}

function normalizeDisplayDateValue(value: unknown, kind: EditorKind): string {
  if (value instanceof Date) {
    const date = `${value.getFullYear()}-${pad2(value.getMonth() + 1)}-${pad2(value.getDate())}`;
    if (kind === "date") return date;
    const time = `${pad2(value.getHours())}:${pad2(value.getMinutes())}:${pad2(value.getSeconds())}`;
    if (kind === "time") return time;
    return `${date} ${time}`;
  }
  const raw = String(value || "").trim();
  if (!raw) return "";
  if (kind === "date") {
    const m = raw.match(/(\d{4}-\d{2}-\d{2})/);
    return m ? m[1] : raw;
  }
  if (kind === "time") {
    const m = raw.match(/(\d{2}:\d{2}(?::\d{2})?)/);
    return m ? m[1] : raw;
  }
  const m = raw.match(/(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})(?::(\d{2}))?/);
  if (m) return `${m[1]} ${m[2]}:${m[3] || "00"}`;
  return raw.replace("T", " ");
}

function toEditorInputValue(value: unknown, kind: EditorKind): string {
  if (value === null || value === undefined) return "";
  if (kind !== "date" && kind !== "time" && kind !== "datetime") return String(value);

  const normalized = normalizeDisplayDateValue(value, kind);
  if (!normalized) return "";
  if (kind === "date") return normalized.slice(0, 10);
  if (kind === "time") {
    const m = normalized.match(/^(\d{2}:\d{2}(?::\d{2})?)/);
    return m ? (m[1].length === 5 ? `${m[1]}:00` : m[1]) : "";
  }
  const m = normalized.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2})(?::(\d{2}))?/);
  if (!m) return "";
  return `${m[1]}T${m[2]}:${m[3] || "00"}`;
}

function fromEditorInputValue(raw: string, kind: EditorKind): string | null {
  const val = raw.trim();
  if (!val) return null;
  if (kind === "date") return val;
  if (kind === "time") return val.length === 5 ? `${val}:00` : val;
  const v = val.replace("T", " ");
  return v.length === 16 ? `${v}:00` : v;
}

function normalizeDefaultValue(rawDefault: string | null): string | null {
  if (rawDefault === null || rawDefault === undefined) return null;
  const trimmed = String(rawDefault).trim();
  if (!trimmed) return null;
  const dequoted = trimmed.replace(/^['"]|['"]$/g, "");
  if (/^null$/i.test(dequoted)) return NULL_SENTINEL;
  if (/^(current_timestamp(\(\))?|now\(\))$/i.test(dequoted)) return NOW_SENTINEL;
  return dequoted;
}

function getEditorDisplayValue(raw: string): string {
  if (raw === NULL_SENTINEL) return "NULL";
  return raw;
}

function coerceEditedValue(raw: string, meta: ResolvedColumnMeta): unknown {
  if (raw === NULL_SENTINEL) return null;

  if (meta.kind === "date" || meta.kind === "time" || meta.kind === "datetime") {
    const v = fromEditorInputValue(raw, meta.kind);
    if (v === null && meta.nullable) return null;
    return v ?? "";
  }

  if (meta.kind === "enum" && raw === "" && meta.nullable) {
    return null;
  }

  return raw;
}

function getComparableValue(value: unknown, meta: ResolvedColumnMeta): string {
  if (value === null || value === undefined) return "null";
  if (meta.kind === "date" || meta.kind === "time" || meta.kind === "datetime") {
    const v = fromEditorInputValue(toEditorInputValue(value, meta.kind), meta.kind);
    if (v === null) return "null";
    return `dt:${v}`;
  }
  return `raw:${String(value)}`;
}

function renderCellValue(
  value: unknown,
  colMeta: ResolvedColumnMeta,
  nullText: string,
): React.ReactNode {
  // NULL 灰色斜体占位
  if (value === null || value === undefined) {
    return <span className="text-[var(--fg-muted)] italic opacity-50 select-none">{nullText}</span>;
  }

  if (colMeta.kind === "date" || colMeta.kind === "time" || colMeta.kind === "datetime") {
    const text = normalizeDisplayDateValue(value, colMeta.kind);
    return <span className="truncate block font-mono" title={text}>{text}</span>;
  }

  const rawText = String(value);
  const displayText =
    rawText.length > MAX_CELL_TEXT_RENDER
      ? `${rawText.slice(0, MAX_CELL_TEXT_RENDER)}…`
      : rawText;
  // 超长文本 tooltip 显示完整内容（最多 1000 字符）
  const tooltipText = rawText.length > 60 ? rawText.slice(0, 1000) : undefined;
  return <span className="truncate block" title={tooltipText}>{displayText}</span>;
}

// 批量计算列宽：用户手动拖拽 > 自动缓存 > 实时计算
function computeAndCacheWidths(
  columns: ColumnMeta[],
  data: Record<string, unknown>[],
  database: string,
  table: string,
): Record<string, number> {
  const result: Record<string, number> = {};
  for (const col of columns) {
    const cacheKey = getCacheKey(database, table, col.name);
    // 手动拖拽优先级最高
    const manual = manualWidthCache.get(cacheKey);
    if (manual !== undefined) {
      result[col.name] = manual;
      continue;
    }
    // 自动计算缓存
    const cached = autoWidthCache.get(cacheKey);
    if (cached !== undefined) {
      result[col.name] = cached;
    } else {
      const w = computeColumnWidth(col, data);
      autoWidthCache.set(cacheKey, w);
      result[col.name] = w;
    }
  }
  return result;
}

// ====== DataGrid 组件 ======

interface DataGridProps {
  columns: ColumnMeta[];
  columnInfos?: ColumnInfo[];
  data: Record<string, unknown>[];
  sorting?: SortingState;
  onSortingChange?: (sorting: SortingState) => void;
  manualSorting?: boolean;
  selectedRowIndex: number | null;
  selectedRowIndexes?: Set<number>;
  onSelectRow: (index: number | null) => void;
  onSelectRows?: (indexes: Set<number>) => void;
  onCellDoubleClick?: (rowIndex: number, column: string) => void;
  onContextMenu?: (e: React.MouseEvent, rowIndex: number, columnName?: string) => void;
  editedCells?: Record<string, unknown>;
  onCellEdit?: (rowIndex: number, column: string, value: unknown) => void;
  onAppendRow?: () => void;
  showRowNumbers?: boolean;
  rowNumberOffset?: number;
  database?: string;
  tableName?: string;
  newRowIndexes?: Set<number>;
  pendingDeleteIndexes?: Set<number>;
  stretchToContainer?: boolean;
  /** 外部获取 autoFitAll 回调的 ref */
  autoFitAllRef?: React.MutableRefObject<(() => void) | null>;
}

export function DataGrid({
  columns,
  columnInfos = [],
  data,
  sorting: controlledSorting,
  onSortingChange,
  manualSorting = false,
  selectedRowIndex,
  selectedRowIndexes,
  onSelectRow,
  onSelectRows,
  onCellDoubleClick,
  onContextMenu,
  editedCells = {},
  onCellEdit,
  onAppendRow,
  showRowNumbers = false,
  rowNumberOffset = 0,
  database = "",
  tableName = "",
  newRowIndexes = new Set(),
  pendingDeleteIndexes = new Set(),
  stretchToContainer = true,
  autoFitAllRef,
}: DataGridProps) {
  const [internalSorting, setInternalSorting] = useState<SortingState>([]);
  // 编辑状态独立管理，不进入 useMemo 的 deps
  const [editingCell, setEditingCell] = useState<{ row: number; col: string; meta: ResolvedColumnMeta } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement | HTMLSelectElement | null>(null);
  const setEditInputRef = useCallback((el: HTMLInputElement | null) => {
    editInputRef.current = el;
  }, []);
  const [editorDropdownOpen, setEditorDropdownOpen] = useState(false);
  const [editorHighlightIdx, setEditorHighlightIdx] = useState(-1);
  const [editorDropdownPos, setEditorDropdownPos] = useState({ top: 0, left: 0, width: 160 });
  const editorDropdownRef = useRef<HTMLDivElement>(null);
  const editorAnchorRef = useRef<HTMLDivElement | null>(null);
  const setEditorAnchorRef = useCallback((el: HTMLDivElement | null) => {
    editorAnchorRef.current = el;
  }, []);
  const [colWidths, setColWidths] = useState<Record<string, number>>({});
  const [gridViewportHeight, setGridViewportHeight] = useState(0);
  const [measuredHeaderHeight, setMeasuredHeaderHeight] = useState(30);
  const [measuredRowHeight, setMeasuredRowHeight] = useState(28);
  const [runtimeExtraFillerRows, setRuntimeExtraFillerRows] = useState(0);
  const resizingRef = useRef<{ col: string; startX: number; startWidth: number } | null>(null);
  const isDraggingRef = useRef(false);
  const dragSelectingRef = useRef(false);
  const dragStartRowRef = useRef<number | null>(null);
  const rangeAnchorRowRef = useRef<number | null>(null);
  const resizeRafRef = useRef<number | null>(null);
  const pendingResizeWidthRef = useRef<number | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);
  const widthsInitRef = useRef(false);
  const lastTableKeyRef = useRef("");
  const { t } = useTranslation();
  const resolvedSorting = controlledSorting ?? internalSorting;

  const handleSortingChange = useCallback<OnChangeFn<SortingState>>((updater) => {
    if (controlledSorting !== undefined) {
      const next = typeof updater === "function" ? updater(controlledSorting) : updater;
      onSortingChange?.(next);
      return;
    }
    setInternalSorting((prev) => {
      const next = typeof updater === "function" ? updater(prev) : updater;
      onSortingChange?.(next);
      return next;
    });
  }, [controlledSorting, onSortingChange]);

  // 用 ref 追踪最新的编辑回调和状态，避免闭包陷阱
  const onCellEditRef = useRef(onCellEdit);
  onCellEditRef.current = onCellEdit;
  const editingCellRef = useRef(editingCell);
  editingCellRef.current = editingCell;
  const editValueRef = useRef(editValue);
  editValueRef.current = editValue;

  const columnInfoMap = useMemo(() => {
    const map = new Map<string, ColumnInfo>();
    for (const info of columnInfos) {
      map.set(info.name, info);
    }
    return map;
  }, [columnInfos]);

  const resolvedColumnMetaMap = useMemo(() => {
    const map: Record<string, ResolvedColumnMeta> = {};
    for (const col of columns) {
      const info = columnInfoMap.get(col.name);
      const type = info?.type || col.type || "";
      const enumOptions = parseEnumOptions(type);
      map[col.name] = {
        kind: resolveEditorKind(type, enumOptions),
        nullable: info?.nullable ?? col.nullable ?? true,
        type,
        enumOptions,
        defaultValue: info?.defaultValue ?? null,
      };
    }
    return map;
  }, [columnInfoMap, columns]);

  const updateEditorDropdownPos = useCallback(() => {
    const anchor = editorAnchorRef.current;
    if (!anchor) return;
    const rect = anchor.getBoundingClientRect();
    setEditorDropdownPos({
      top: rect.bottom + 2,
      left: rect.left,
      width: Math.max(140, rect.width),
    });
  }, []);

  const measureGridMetrics = useCallback(() => {
    const root = gridRef.current;
    if (!root) return;

    const viewportHeight = Math.floor(root.clientHeight);
    setGridViewportHeight((prev) => (prev === viewportHeight ? prev : viewportHeight));

    const header = root.querySelector("thead") as HTMLTableSectionElement | null;
    if (header) {
      const h = Math.round(header.getBoundingClientRect().height);
      if (h > 0) {
        setMeasuredHeaderHeight((prev) => (prev === h ? prev : h));
      }
    }

    const rows = root.querySelectorAll("tbody tr");
    if (rows.length >= 2) {
      const first = rows[0] as HTMLTableRowElement;
      const second = rows[1] as HTMLTableRowElement;
      const step = Math.round(second.getBoundingClientRect().top - first.getBoundingClientRect().top);
      if (step > 10) {
        setMeasuredRowHeight((prev) => (prev === step ? prev : step));
      }
      return;
    }

    const row = rows.length === 1 ? (rows[0] as HTMLTableRowElement) : null;
    if (row) {
      const fallbackHeight = Math.round(row.getBoundingClientRect().height);
      if (fallbackHeight > 10) {
        setMeasuredRowHeight((prev) => (prev === fallbackHeight ? prev : fallbackHeight));
      }
    }
  }, []);

  const getEditorDropdownItems = useCallback((meta: ResolvedColumnMeta, currentValue: string): EditorDropdownItem[] => {
    if (meta.kind === "enum") {
      const query = currentValue.trim().toLowerCase();
      const options = meta.enumOptions.filter((opt) => !query || opt.toLowerCase().includes(query));
      const items: EditorDropdownItem[] = [];
      if (meta.nullable) items.push({ label: "NULL", value: NULL_SENTINEL, action: "set" });
      for (const opt of options) {
        items.push({ label: opt, value: opt, action: "set" });
      }
      return items;
    }
    if (meta.kind === "date" || meta.kind === "time" || meta.kind === "datetime") {
      const items: EditorDropdownItem[] = [{ label: "Manual input", value: "", action: "manual" }];
      if (meta.nullable) items.push({ label: "NULL", value: NULL_SENTINEL, action: "null" });
      items.push({ label: "NOW()", value: "", action: "now" });
      if (meta.defaultValue !== null && meta.defaultValue !== "") {
        items.push({ label: "DEFAULT", value: "", action: "default" });
      }
      return items;
    }
    return [];
  }, []);

  useEffect(() => {
    const tableKey = `${database}:${tableName}`;
    const isNewTable = tableKey !== lastTableKeyRef.current;

    if (columns.length === 0) return;

    if (isNewTable) {
      widthsInitRef.current = false;
      lastTableKeyRef.current = tableKey;
    }

    if (!widthsInitRef.current && data.length > 0) {
      const widths = computeAndCacheWidths(columns, data, database, tableName);
      setColWidths(widths);
      widthsInitRef.current = true;
    } else if (!widthsInitRef.current && data.length === 0 && columns.length > 0) {
      const widths = computeAndCacheWidths(columns, [], database, tableName);
      setColWidths(widths);
      widthsInitRef.current = true;
    }
  }, [columns, data, database, tableName]);

  useEffect(() => {
    measureGridMetrics();
  }, [measureGridMetrics, data.length, columns.length, showRowNumbers]);

  useEffect(() => {
    const el = gridRef.current;
    if (!el) return;

    const onWindowResize = () => measureGridMetrics();
    window.addEventListener("resize", onWindowResize);

    if (typeof ResizeObserver === "undefined") {
      return () => window.removeEventListener("resize", onWindowResize);
    }

    const observer = new ResizeObserver(() => {
      measureGridMetrics();
    });
    observer.observe(el);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", onWindowResize);
    };
  }, [measureGridMetrics]);

  // 编辑框获取焦点
  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus();
      if (editInputRef.current instanceof HTMLInputElement && !["date", "time", "datetime-local"].includes(editInputRef.current.type)) {
        editInputRef.current.select();
      }
    }
  }, [editingCell, data.length]);

  useEffect(() => {
    if (!editingCell) {
      setEditorDropdownOpen(false);
      setEditorHighlightIdx(-1);
    }
  }, [editingCell]);

  useEffect(() => {
    if (!editorDropdownOpen) return;
    updateEditorDropdownPos();
    const onDocMouseDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (editorDropdownRef.current?.contains(target)) return;
      if (editorAnchorRef.current?.contains(target)) return;
      setEditorDropdownOpen(false);
      setEditorHighlightIdx(-1);
    };
    const onReposition = () => updateEditorDropdownPos();
    document.addEventListener("mousedown", onDocMouseDown);
    window.addEventListener("resize", onReposition);
    window.addEventListener("scroll", onReposition, true);
    return () => {
      document.removeEventListener("mousedown", onDocMouseDown);
      window.removeEventListener("resize", onReposition);
      window.removeEventListener("scroll", onReposition, true);
    };
  }, [editorDropdownOpen, updateEditorDropdownPos]);

  const handleDoubleClick = useCallback((rowIndex: number, colName: string, currentValue: unknown) => {
    onCellDoubleClick?.(rowIndex, colName);
    if (!onCellEditRef.current) return;
    const meta = resolvedColumnMetaMap[colName] || {
      kind: "text",
      nullable: true,
      type: "",
      enumOptions: [],
      defaultValue: null,
    };
    setEditingCell({ row: rowIndex, col: colName, meta });
    setEditValue(toEditorInputValue(currentValue, meta.kind));
  }, [onCellDoubleClick, resolvedColumnMetaMap]);

  const handleDoubleClickEmpty = useCallback((colName: string) => {
    if (!onAppendRow || !onCellEditRef.current) return;
    const newRowIndex = data.length;
    const meta = resolvedColumnMetaMap[colName] || {
      kind: "text",
      nullable: true,
      type: "",
      enumOptions: [],
      defaultValue: null,
    };
    onAppendRow();
    onSelectRow(newRowIndex);
    if (onSelectRows) onSelectRows(new Set<number>([newRowIndex]));
    rangeAnchorRowRef.current = newRowIndex;
    setEditingCell({ row: newRowIndex, col: colName, meta });
    setEditValue("");
    requestAnimationFrame(() => gridRef.current?.focus());
  }, [data.length, onAppendRow, onSelectRow, onSelectRows, resolvedColumnMetaMap]);

  const handleDoubleClickEmptyRow = useCallback((e: React.MouseEvent<HTMLTableRowElement>) => {
    if (!onAppendRow || columns.length === 0) return;
    const target = e.target as HTMLElement | null;
    const td = target?.closest("td[data-empty-col-id]") as HTMLTableCellElement | null;
    const colName = td?.dataset.emptyColId || columns[0]?.name;
    if (!colName) return;
    handleDoubleClickEmpty(colName);
  }, [columns, handleDoubleClickEmpty, onAppendRow]);

  // 提交编辑：仅当值实际变化时才通知外部
  const commitEditWithValue = useCallback((rawValue?: string) => {
    const cell = editingCellRef.current;
    if (cell) {
      const nextRaw = rawValue ?? editValueRef.current;
      const nextValue = coerceEditedValue(nextRaw, cell.meta);
      const origValue = data[cell.row]?.[cell.col];
      const before = getComparableValue(origValue, cell.meta);
      const after = getComparableValue(nextValue, cell.meta);
      if (before !== after) {
        onCellEditRef.current?.(cell.row, cell.col, nextValue);
      }
      setEditingCell(null);
      setEditorDropdownOpen(false);
      setEditorHighlightIdx(-1);
    }
  }, [data]);

  const commitEdit = useCallback(() => {
    commitEditWithValue();
  }, [commitEditWithValue]);

  const cancelEdit = useCallback(() => {
    setEditingCell(null);
    setEditorDropdownOpen(false);
    setEditorHighlightIdx(-1);
  }, []);

  const applyEditorDropdownItem = useCallback((item: EditorDropdownItem) => {
    const cell = editingCellRef.current;
    if (!cell) return;
    const meta = cell.meta;

    if (item.action === "manual") {
      setEditorDropdownOpen(false);
      requestAnimationFrame(() => editInputRef.current?.focus());
      return;
    }
    if (item.action === "null") {
      commitEditWithValue(NULL_SENTINEL);
      return;
    }
    if (item.action === "now") {
      commitEditWithValue(getNowByKind(meta.kind));
      return;
    }
    if (item.action === "default") {
      const normalizedDefault = normalizeDefaultValue(meta.defaultValue);
      if (normalizedDefault === NULL_SENTINEL) {
        commitEditWithValue(NULL_SENTINEL);
        return;
      }
      if (normalizedDefault === NOW_SENTINEL) {
        commitEditWithValue(getNowByKind(meta.kind));
        return;
      }
      if (normalizedDefault) {
        if (meta.kind === "date" || meta.kind === "time" || meta.kind === "datetime") {
          commitEditWithValue(toEditorInputValue(normalizedDefault, meta.kind));
        } else {
          commitEditWithValue(normalizedDefault);
        }
      }
      return;
    }

    if (item.action === "set") {
      setEditValue(item.value);
      commitEditWithValue(item.value);
    }
  }, [commitEditWithValue]);

  // 双击列边 auto-fit：重新计算该列宽度并清除手动拖拽缓存
  const handleAutoFitColumn = useCallback((colName: string) => {
    const col = columns.find((c) => c.name === colName);
    if (!col) return;
    const w = autoFitColumnWidth(col, data, database, tableName);
    setColWidths((prev) => ({ ...prev, [colName]: w }));
  }, [columns, data, database, tableName]);

  // 全表 auto-fit：对所有列重新计算宽度
  const handleAutoFitAll = useCallback(() => {
    const next: Record<string, number> = {};
    for (const col of columns) {
      next[col.name] = autoFitColumnWidth(col, data, database, tableName);
    }
    setColWidths(next);
  }, [columns, data, database, tableName]);

  // 把 autoFitAll 暴露给外部
  useEffect(() => {
    if (autoFitAllRef) {
      autoFitAllRef.current = handleAutoFitAll;
    }
    return () => {
      if (autoFitAllRef) autoFitAllRef.current = null;
    };
  }, [autoFitAllRef, handleAutoFitAll]);

  const handleResizeStart = useCallback((e: React.MouseEvent, colName: string) => {
    e.preventDefault();
    e.stopPropagation();
    isDraggingRef.current = true;
    const startX = e.clientX;
    const startWidth = colWidths[colName] || MIN_COL_WIDTH;
    resizingRef.current = { col: colName, startX, startWidth };

    const handleMouseMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const diff = ev.clientX - resizingRef.current.startX;
      const newWidth = Math.max(MIN_COL_WIDTH, Math.min(MAX_COL_WIDTH, resizingRef.current.startWidth + diff));
      pendingResizeWidthRef.current = newWidth;
      if (resizeRafRef.current !== null) return;
      resizeRafRef.current = requestAnimationFrame(() => {
        resizeRafRef.current = null;
        if (!resizingRef.current || pendingResizeWidthRef.current === null) return;
        const nextWidth = pendingResizeWidthRef.current;
        setColWidths((prev) => ({ ...prev, [resizingRef.current!.col]: nextWidth }));
      });
    };
    const handleMouseUp = () => {
      if (resizeRafRef.current !== null) {
        cancelAnimationFrame(resizeRafRef.current);
        resizeRafRef.current = null;
      }
      if (resizingRef.current && pendingResizeWidthRef.current !== null) {
        const col = resizingRef.current.col;
        const finalWidth = pendingResizeWidthRef.current;
        setColWidths((prev) => ({ ...prev, [col]: finalWidth }));
      }
      // 用户手动拖拽的列宽写入 manualWidthCache，优先级最高
      if (resizingRef.current) {
        const finalWidth = pendingResizeWidthRef.current ??
          colWidths[resizingRef.current.col] ??
          Math.max(MIN_COL_WIDTH, resizingRef.current.startWidth);
        const cacheKey = getCacheKey(database, tableName, resizingRef.current.col);
        manualWidthCache.set(cacheKey, finalWidth);
      }
      pendingResizeWidthRef.current = null;
      resizingRef.current = null;
      requestAnimationFrame(() => {
        isDraggingRef.current = false;
      });
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
  }, [colWidths, database, tableName]);

  const handleHeaderClick = useCallback((handler: ((e: unknown) => void) | undefined, e: React.MouseEvent) => {
    if (isDraggingRef.current) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    handler?.(e);
  }, []);

  // 列定义不再依赖 editingCell/editValue，消除输入时重建问题
  const tableColumns: ColumnDef<Record<string, unknown>>[] = useMemo(() => columns.map(
    (col) => ({
      accessorKey: col.name,
      header: col.name,
      cell: (info) => {
        const value = info.getValue();
        const colMeta = resolvedColumnMetaMap[col.name] || {
          kind: "text",
          nullable: true,
          type: "",
          enumOptions: [],
          defaultValue: null,
        };
        return renderCellValue(value, colMeta, t("query.null"));
      },
      size: colWidths[col.name] || 150,
    })
  ), [columns, colWidths, resolvedColumnMetaMap, t]);

  const editorDropdownItems = useMemo(() => {
    if (!editingCell) return [];
    return getEditorDropdownItems(editingCell.meta, editValue);
  }, [editValue, editingCell, getEditorDropdownItems]);

  const table = useReactTable({
    data,
    columns: tableColumns,
    state: { sorting: resolvedSorting },
    onSortingChange: handleSortingChange,
    getCoreRowModel: getCoreRowModel(),
    ...(manualSorting ? {} : { getSortedRowModel: getSortedRowModel() }),
    manualSorting,
  });

  const baseFillerRowCount = useMemo(() => {
    if (columns.length === 0) return 0;
    if (gridViewportHeight <= 0) return 0;
    const rowHeight = Math.max(20, measuredRowHeight);
    const headerHeight = Math.max(0, measuredHeaderHeight);
    // +1 用于吸收 border-collapse 的像素取整误差，避免底部偶发露白
    const availableBodyHeight = Math.max(0, gridViewportHeight - headerHeight + 1);
    const targetRows = Math.ceil(availableBodyHeight / rowHeight);
    return Math.max(0, Math.min(400, targetRows - data.length));
  }, [columns.length, data.length, gridViewportHeight, measuredHeaderHeight, measuredRowHeight]);

  useEffect(() => {
    if (columns.length === 0 || gridViewportHeight <= 0) {
      setRuntimeExtraFillerRows((prev) => (prev === 0 ? prev : 0));
      return;
    }
    const root = gridRef.current;
    if (!root) return;

    let rafId: number | null = requestAnimationFrame(() => {
      rafId = null;
      const tbody = root.querySelector("tbody");
      if (!tbody) {
        setRuntimeExtraFillerRows((prev) => (prev === 0 ? prev : 0));
        return;
      }

      const rows = tbody.querySelectorAll("tr");
      if (rows.length === 0) {
        setRuntimeExtraFillerRows((prev) => (prev === 0 ? prev : 0));
        return;
      }

      const lastRow = rows[rows.length - 1] as HTMLTableRowElement;
      const tbodyRect = tbody.getBoundingClientRect();
      const lastRect = lastRow.getBoundingClientRect();
      const availableBodyHeight = Math.max(0, root.clientHeight - measuredHeaderHeight);
      const filledBodyHeight = Math.max(0, Math.round(lastRect.bottom - tbodyRect.top));
      const gap = availableBodyHeight - filledBodyHeight;

      if (gap > 1) {
        const extra = Math.min(16, Math.ceil(gap / Math.max(20, measuredRowHeight)));
        setRuntimeExtraFillerRows((prev) => (prev === extra ? prev : extra));
      } else {
        setRuntimeExtraFillerRows((prev) => (prev === 0 ? prev : 0));
      }
    });

    return () => {
      if (rafId !== null) cancelAnimationFrame(rafId);
    };
  }, [
    baseFillerRowCount,
    columns.length,
    data.length,
    gridViewportHeight,
    measuredHeaderHeight,
    measuredRowHeight,
  ]);

  const fillerRowCount = useMemo(() => {
    return Math.max(0, Math.min(400, baseFillerRowCount + runtimeExtraFillerRows));
  }, [baseFillerRowCount, runtimeExtraFillerRows]);

  const rowNumberColWidth = useMemo(() => {
    if (!showRowNumbers) return ROW_NUMBER_COL_MIN_WIDTH;
    if (data.length === 0) return ROW_NUMBER_COL_EMPTY_WIDTH;
    const maxRowNumber = rowNumberOffset + data.length;
    const digits = String(Math.max(1, maxRowNumber)).length;
    return Math.max(ROW_NUMBER_COL_MIN_WIDTH, 20 + digits * 10);
  }, [showRowNumbers, data.length, rowNumberOffset]);

  const tableContentWidth = useMemo(() => {
    const rowNumWidth = showRowNumbers ? rowNumberColWidth : 0;
    const colsWidth = table.getVisibleLeafColumns().reduce((sum, col) => {
      const w = colWidths[col.id] || 150;
      return sum + w;
    }, 0);
    return rowNumWidth + colsWidth;
  }, [showRowNumbers, rowNumberColWidth, table, colWidths]);

  const effectiveSelectedRows = useMemo(() => {
    if (selectedRowIndexes && selectedRowIndexes.size > 0) return selectedRowIndexes;
    if (selectedRowIndex === null) return new Set<number>();
    return new Set<number>([selectedRowIndex]);
  }, [selectedRowIndex, selectedRowIndexes]);

  const selectRowRange = useCallback((start: number, end: number) => {
    if (!onSelectRows) return;
    const min = Math.max(0, Math.min(start, end));
    const max = Math.min(data.length - 1, Math.max(start, end));
    const next = new Set<number>();
    for (let i = min; i <= max; i += 1) next.add(i);
    onSelectRows(next);
  }, [data.length, onSelectRows]);

  const handleRowContextMenu = useCallback(
    (e: React.MouseEvent, rowIndex: number, columnName?: string) => {
      const row = data[rowIndex];
      const resolvedColumnName = columnName
        ?? columns.find((col) => {
          const value = row?.[col.name];
          return value !== null && value !== undefined && String(value) !== "";
        })?.name
        ?? columns[0]?.name;

      onSelectRow(rowIndex);
      if (onSelectRows) onSelectRows(new Set<number>([rowIndex]));
      rangeAnchorRowRef.current = rowIndex;
      gridRef.current?.focus();
      const selection = window.getSelection();
      if (selection && selection.type === "Range") {
        selection.removeAllRanges();
      }
      onContextMenu?.(e, rowIndex, resolvedColumnName);
    },
    [columns, data, onContextMenu, onSelectRow, onSelectRows]
  );

  useEffect(() => {
    const onPointerUp = () => {
      dragSelectingRef.current = false;
      dragStartRowRef.current = null;
    };
    window.addEventListener("pointerup", onPointerUp);
    return () => window.removeEventListener("pointerup", onPointerUp);
  }, []);

  return (
    <div
      ref={gridRef}
      className="flex-1 min-h-0 overflow-auto relative scroll-always focus:outline-none"
      tabIndex={0}
      role="grid"
      data-grid-root="true"
    >
      <table
        className={cn("border-collapse table-fixed", stretchToContainer ? "w-full" : "")}
        style={stretchToContainer ? { minWidth: "max-content" } : { width: tableContentWidth, minWidth: tableContentWidth }}
      >
        <colgroup>
          {showRowNumbers && (
            <col style={{ width: rowNumberColWidth, minWidth: rowNumberColWidth, maxWidth: rowNumberColWidth }} />
          )}
          {table.getVisibleLeafColumns().map((col) => {
            const w = colWidths[col.id] || 150;
            return <col key={`col_${col.id}`} style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }} />;
          })}
          {stretchToContainer && (
            <col style={{ width: "auto" }} />
          )}
        </colgroup>
        <thead className="sticky top-0 z-20">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {showRowNumbers && (
                <th
                  className={cn("data-grid-header border-r border-b text-center sticky top-0 z-20", "border-[var(--border-color)]")}
                  style={{ width: rowNumberColWidth, minWidth: rowNumberColWidth, maxWidth: rowNumberColWidth }}
                >
                  #
                </th>
              )}
              {headerGroup.headers.map((header) => {
                const w = colWidths[header.column.id] || 150;
                return (
                  <th
                    key={header.id}
                    className={cn("data-grid-header border-r border-b cursor-pointer hover:bg-[var(--row-hover)] relative select-none sticky top-0 z-20", "border-[var(--border-color)]")}
                    style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }}
                    onClick={(e) => handleHeaderClick(header.column.getToggleSortingHandler(), e)}
                  >
                    <div className="flex items-center gap-1 overflow-hidden">
                      <span className="truncate">{flexRender(header.column.columnDef.header, header.getContext())}</span>
                      {header.column.getIsSorted() === "asc" && <ArrowUp className="h-3 w-3 flex-shrink-0" />}
                      {header.column.getIsSorted() === "desc" && <ArrowDown className="h-3 w-3 flex-shrink-0" />}
                    </div>
                    {/* 列宽拖拽手柄，双击触发 auto-fit */}
                    <div
                      className={cn(
                        "absolute right-0 top-0 h-full w-[5px] cursor-col-resize",
                        "after:absolute after:right-[2px] after:top-[25%] after:h-[50%] after:w-[1px]",
                        "after:bg-[var(--border-color)] after:rounded-full after:transition-colors",
                        "hover:after:bg-[var(--accent)] hover:after:w-[2px] hover:after:right-[1.5px]",
                        "active:after:bg-[var(--accent)]"
                      )}
                      onMouseDown={(e) => handleResizeStart(e, header.column.id)}
                      onDoubleClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        handleAutoFitColumn(header.column.id);
                      }}
                    />
                  </th>
                );
              })}
              {stretchToContainer && (
                <th
                  className={cn("data-grid-header border-r border-b sticky top-0 z-20", "border-[var(--border-color)]")}
                />
              )}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row, rowIndex) => {
            const isSelected = effectiveSelectedRows.has(rowIndex);
            const isNewRow = newRowIndexes.has(rowIndex);
            const isPendingDelete = pendingDeleteIndexes.has(rowIndex);
            return (
              <tr
                key={row.id}
                className={cn(
                  "transition-colors cursor-default select-none",
                  isSelected
                    ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                    : rowIndex % 2 === 0
                      ? "bg-[var(--surface)]"
                      : "bg-[var(--row-stripe)]",
                  !isSelected && "hover:bg-[var(--row-hover)]",
                  isNewRow && "bg-[var(--row-new-bg)]",
                  isPendingDelete && "bg-[var(--row-delete-bg)]",
                )}
                onPointerDown={(e) => {
                  if (e.button !== 0) return;
                  const target = e.target as HTMLElement | null;
                  if (target?.closest("[data-grid-editor='true']")) return;

                  // 双击时不处理选择逻辑，交给 onDoubleClick 处理编辑
                  if (e.detail >= 2) return;

                  const isMod = e.metaKey || e.ctrlKey;
                  const isShift = e.shiftKey;

                  if (isShift) {
                    e.preventDefault();
                    const anchor = rangeAnchorRowRef.current ?? selectedRowIndex ?? rowIndex;
                    selectRowRange(anchor, rowIndex);
                    onSelectRow(rowIndex);
                    return;
                  }

                  if (isMod && onSelectRows) {
                    e.preventDefault();
                    const next = new Set(effectiveSelectedRows);
                    if (next.has(rowIndex)) {
                      next.delete(rowIndex);
                    } else {
                      next.add(rowIndex);
                    }
                    onSelectRows(next);
                    onSelectRow(rowIndex);
                    rangeAnchorRowRef.current = rowIndex;
                    return;
                  }

                  onSelectRow(rowIndex);
                  if (onSelectRows) onSelectRows(new Set<number>([rowIndex]));
                  rangeAnchorRowRef.current = rowIndex;
                  dragSelectingRef.current = true;
                  dragStartRowRef.current = rowIndex;
                  gridRef.current?.focus();
                }}
                onPointerEnter={() => {
                  if (!dragSelectingRef.current) return;
                  const dragStart = dragStartRowRef.current;
                  if (dragStart === null) return;
                  selectRowRange(dragStart, rowIndex);
                  onSelectRow(rowIndex);
                }}
                onPointerMove={(e) => {
                  if (!dragSelectingRef.current) return;
                  if ((e.buttons & 1) !== 1) return;
                  const dragStart = dragStartRowRef.current;
                  if (dragStart === null) return;
                  selectRowRange(dragStart, rowIndex);
                  onSelectRow(rowIndex);
                }}
              >
                {showRowNumbers && (
                  <td
                    className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]"
                    style={{ width: rowNumberColWidth, minWidth: rowNumberColWidth, maxWidth: rowNumberColWidth }}
                    onDoubleClick={() => {
                      const firstCell = row.getVisibleCells()[0];
                      if (!firstCell) return;
                      handleDoubleClick(rowIndex, firstCell.column.id, firstCell.getValue());
                    }}
                    onContextMenu={(e) => handleRowContextMenu(e, rowIndex)}
                  >
                    {rowNumberOffset + rowIndex + 1}
                  </td>
                )}
                {row.getVisibleCells().map((cell) => {
                  const cellKey = `${rowIndex}:${cell.column.id}`;
                  const isEdited = cellKey in editedCells;
                  const isEditing = editingCell?.row === rowIndex && editingCell?.col === cell.column.id;
                  const colMeta = resolvedColumnMetaMap[cell.column.id] || {
                    kind: "text",
                    nullable: true,
                    type: "",
                    enumOptions: [],
                    defaultValue: null,
                  };
                  const w = colWidths[cell.column.id] || 150;
                  return (
                    <td
                      key={cell.id}
                      className={cn(
                        "data-grid-cell relative",
                        isEditing ? "overflow-visible" : "overflow-hidden",
                        isEdited && "border-l-2 border-l-[var(--warning)] bg-[var(--cell-edit-bg)]/30",
                        isSelected && "text-[var(--fg)] font-medium"
                      )}
                      style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }}
                      onDoubleClick={(e) => {
                        const target = e.target as HTMLElement | null;
                        if (target?.closest("[data-grid-editor='true']")) return;
                        handleDoubleClick(rowIndex, cell.column.id, cell.getValue());
                      }}
                      onContextMenu={(e) => handleRowContextMenu(e, rowIndex, cell.column.id)}
                    >
                      {isEditing ? (
                        colMeta.kind === "enum" || colMeta.kind === "date" || colMeta.kind === "time" || colMeta.kind === "datetime" ? (
                          <>
                            <div
                              ref={setEditorAnchorRef}
                              data-grid-editor="true"
                              className="absolute inset-[1px] z-20"
                              onMouseDown={(e) => e.stopPropagation()}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <div className="relative flex h-full items-center">
                                <input
                                  ref={setEditInputRef}
                                  type={colMeta.kind === "date" ? "date" : colMeta.kind === "time" ? "time" : colMeta.kind === "datetime" ? "datetime-local" : "text"}
                                  step={colMeta.kind === "datetime" || colMeta.kind === "time" ? 1 : undefined}
                                  className={cn(
                                    "w-full h-full border border-[var(--accent)] outline-none text-[length:var(--size-font-xs)] px-1.5 rounded-[var(--radius-sm)] box-border",
                                    "bg-[var(--surface)] text-[var(--fg)] font-medium pr-5",
                                    (colMeta.kind === "date" || colMeta.kind === "time" || colMeta.kind === "datetime") && "font-mono"
                                  )}
                                  value={getEditorDisplayValue(editValue)}
                                  onChange={(e) => {
                                    const val = e.target.value;
                                    setEditValue(val);
                                    if (colMeta.kind === "enum") {
                                      setEditorDropdownOpen(true);
                                      setEditorHighlightIdx(-1);
                                      requestAnimationFrame(() => updateEditorDropdownPos());
                                    }
                                  }}
                                  onFocus={() => {
                                    if (colMeta.kind === "enum") {
                                      setEditorDropdownOpen(true);
                                      setEditorHighlightIdx(-1);
                                      requestAnimationFrame(() => updateEditorDropdownPos());
                                    }
                                  }}
                                  onBlur={commitEdit}
                                  onKeyDown={(e) => {
                                    if (e.key === "ArrowDown") {
                                      e.preventDefault();
                                      setEditorDropdownOpen(true);
                                      setEditorHighlightIdx((prev) => Math.min(prev + 1, editorDropdownItems.length - 1));
                                      requestAnimationFrame(() => updateEditorDropdownPos());
                                      return;
                                    }
                                    if (e.key === "ArrowUp") {
                                      e.preventDefault();
                                      setEditorHighlightIdx((prev) => Math.max(prev - 1, 0));
                                      return;
                                    }
                                    if (e.key === "Enter") {
                                      e.preventDefault();
                                      if (editorDropdownOpen && editorHighlightIdx >= 0 && editorHighlightIdx < editorDropdownItems.length) {
                                        applyEditorDropdownItem(editorDropdownItems[editorHighlightIdx]);
                                      } else {
                                        commitEdit();
                                      }
                                      return;
                                    }
                                    if (e.key === "Escape") {
                                      e.preventDefault();
                                      cancelEdit();
                                      return;
                                    }
                                    if (e.key === "Tab") {
                                      e.preventDefault();
                                      commitEdit();
                                    }
                                  }}
                                />
                                <button
                                  className="absolute right-0 top-0 bottom-0 w-5 flex items-center justify-center text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                                  tabIndex={-1}
                                  onMouseDown={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    setEditorDropdownOpen((prev) => {
                                      const next = !prev;
                                      if (next) {
                                        setEditorHighlightIdx(-1);
                                        requestAnimationFrame(() => updateEditorDropdownPos());
                                      }
                                      return next;
                                    });
                                  }}
                                >
                                  <ChevronDown className="h-3 w-3" />
                                </button>
                              </div>
                            </div>
                            {editorDropdownOpen && createPortal(
                              <div
                                ref={editorDropdownRef}
                                className="fixed z-[9999] max-h-[240px] overflow-auto rounded-[var(--radius-menu)] border border-[var(--border-color)] bg-[var(--surface)] shadow-lg"
                                style={{ top: editorDropdownPos.top, left: editorDropdownPos.left, width: editorDropdownPos.width }}
                              >
                                {editorDropdownItems.length === 0 ? (
                                  <div className="px-2 py-1.5 text-xs text-[var(--fg-muted)] text-center">
                                    No options
                                  </div>
                                ) : (
                                  editorDropdownItems.map((item, itemIdx) => (
                                    <div
                                      key={`${item.action}:${item.value}:${item.label}:${itemIdx}`}
                                      className={cn(
                                        "px-2 py-[5px] text-xs cursor-pointer transition-colors",
                                        itemIdx === editorHighlightIdx
                                          ? "bg-[var(--accent)] text-white"
                                          : "hover:bg-[var(--row-hover)] text-[var(--fg)]"
                                      )}
                                      onMouseDown={(e) => {
                                        e.preventDefault();
                                        applyEditorDropdownItem(item);
                                      }}
                                      onMouseEnter={() => setEditorHighlightIdx(itemIdx)}
                                    >
                                      {item.label}
                                    </div>
                                  ))
                                )}
                              </div>,
                              document.body
                            )}
                          </>
                        ) : (
                          <input
                            ref={setEditInputRef}
                            data-grid-editor="true"
                            className={cn(
                              "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
                              "bg-[var(--surface)] text-[var(--fg)] font-medium",
                              "absolute inset-0 z-20"
                            )}
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            onBlur={commitEdit}
                            onPointerDown={(e) => e.stopPropagation()}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") { e.preventDefault(); commitEdit(); }
                              if (e.key === "Escape") cancelEdit();
                              if (e.key === "Tab") { e.preventDefault(); commitEdit(); }
                            }}
                            onClick={(e) => e.stopPropagation()}
                          />
                        )
                      ) : (
                        flexRender(cell.column.columnDef.cell, cell.getContext())
                      )}
                    </td>
                  );
                })}
                {stretchToContainer && (
                  <td
                    className="data-grid-cell overflow-hidden"
                    onDoubleClick={() => {
                      const firstCell = row.getVisibleCells()[0];
                      if (!firstCell) return;
                      handleDoubleClick(rowIndex, firstCell.column.id, firstCell.getValue());
                    }}
                    onContextMenu={(e) => handleRowContextMenu(e, rowIndex)}
                  />
                )}
              </tr>
            );
          })}
          {Array.from({ length: fillerRowCount }).map((_, fillerIndex) => {
            const visualRowIndex = data.length + fillerIndex;
            return (
              <tr
                key={`filler_${fillerIndex}`}
                className={cn(
                  "transition-colors select-none",
                  visualRowIndex % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                  onAppendRow && onCellEdit ? "hover:bg-[var(--row-hover)] cursor-default" : "cursor-default"
                )}
                onDoubleClick={handleDoubleClickEmptyRow}
              >
                {showRowNumbers && (
                  <td
                    className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]"
                    style={{ width: rowNumberColWidth, minWidth: rowNumberColWidth, maxWidth: rowNumberColWidth }}
                  >
                    {rowNumberOffset + visualRowIndex + 1}
                  </td>
                )}
                {table.getVisibleLeafColumns().map((col) => {
                  const w = colWidths[col.id] || 150;
                  return (
                    <td
                      key={`filler_${fillerIndex}_${col.id}`}
                      data-empty-col-id={col.id}
                      className="data-grid-cell overflow-hidden"
                      style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }}
                    >
                      <span className="opacity-0 select-none">.</span>
                    </td>
                  );
                })}
                {stretchToContainer && (
                  <td className="data-grid-cell overflow-hidden">
                    <span className="opacity-0 select-none">.</span>
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
