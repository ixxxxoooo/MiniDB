import React, { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, ChevronDown } from "lucide-react";
import { useTranslation } from "@/i18n";
import type { ColumnMeta, ColumnInfo } from "@/types/database";

// ====== 列宽计算与缓存 ======

const MIN_COL_WIDTH = 60;
const MAX_COL_WIDTH = 400;
const ROW_NUMBER_COL_WIDTH = 42;
const HEADER_PADDING = 32;
const CELL_PADDING = 24;
const SAMPLE_ROWS = 50;

const colWidthCache = new Map<string, number>();
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

function getTypeWidthConstraints(colType: string): { min: number; max: number; fixed?: number } {
  const t = colType.toLowerCase();

  if (t.includes("datetime") || t.includes("timestamp")) {
    return { min: 160, max: 180, fixed: 160 };
  }
  if (t.includes("date") && !t.includes("datetime")) {
    return { min: 100, max: 120, fixed: 100 };
  }
  if (t.includes("time") && !t.includes("timestamp") && !t.includes("datetime")) {
    return { min: 90, max: 110, fixed: 90 };
  }
  if (t.includes("int") || t.includes("serial")) {
    return { min: 80, max: 150 };
  }
  if (t.includes("decimal") || t.includes("numeric") || t.includes("float") || t.includes("double") || t.includes("real")) {
    return { min: 90, max: 180 };
  }
  if (t.includes("text") || t.includes("json") || t.includes("blob") || t.includes("clob") || t.includes("bytea")) {
    return { min: 100, max: 300 };
  }
  if (t.includes("uuid") || t.includes("guid")) {
    return { min: 260, max: 300 };
  }
  if (t.includes("enum") || t.includes("set")) {
    return { min: 80, max: 200 };
  }
  return { min: MIN_COL_WIDTH, max: MAX_COL_WIDTH };
}

function computeColumnWidth(
  col: ColumnMeta,
  data: Record<string, unknown>[],
): number {
  const headerFont = "600 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";
  const cellFont = "400 12px -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";

  const constraints = getTypeWidthConstraints(col.type);
  if (constraints.fixed) return constraints.fixed;

  let maxWidth = measureTextWidth(col.name, headerFont) + HEADER_PADDING;

  const sampleData = data.slice(0, SAMPLE_ROWS);
  for (const row of sampleData) {
    const val = row[col.name];
    if (val === null || val === undefined) {
      maxWidth = Math.max(maxWidth, measureTextWidth("NULL", cellFont) + CELL_PADDING);
    } else {
      const text = String(val);
      const display = text.length > 80 ? text.substring(0, 80) : text;
      maxWidth = Math.max(maxWidth, measureTextWidth(display, cellFont) + CELL_PADDING);
    }
  }

  maxWidth = Math.max(constraints.min, Math.min(constraints.max, maxWidth));
  maxWidth = Math.max(MIN_COL_WIDTH, Math.min(MAX_COL_WIDTH, maxWidth));

  return Math.ceil(maxWidth);
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
  if (value === null || value === undefined) {
    return <span className="text-[var(--fg-muted)] italic opacity-70">{nullText}</span>;
  }

  if (colMeta.kind === "date" || colMeta.kind === "time" || colMeta.kind === "datetime") {
    return <span className="truncate block font-mono">{normalizeDisplayDateValue(value, colMeta.kind)}</span>;
  }

  const rawText = String(value);
  const displayText =
    rawText.length > MAX_CELL_TEXT_RENDER
      ? `${rawText.slice(0, MAX_CELL_TEXT_RENDER)}…`
      : rawText;
  return <span className="truncate block">{displayText}</span>;
}

function computeAndCacheWidths(
  columns: ColumnMeta[],
  data: Record<string, unknown>[],
  database: string,
  table: string,
): Record<string, number> {
  const result: Record<string, number> = {};
  for (const col of columns) {
    const cacheKey = getCacheKey(database, table, col.name);
    const cached = colWidthCache.get(cacheKey);
    if (cached !== undefined) {
      result[col.name] = cached;
    } else {
      const w = computeColumnWidth(col, data);
      colWidthCache.set(cacheKey, w);
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
  selectedRowIndex: number | null;
  onSelectRow: (index: number | null) => void;
  onCellDoubleClick?: (rowIndex: number, column: string) => void;
  onContextMenu?: (e: React.MouseEvent, rowIndex: number, columnName?: string) => void;
  editedCells?: Record<string, unknown>;
  onCellEdit?: (rowIndex: number, column: string, value: unknown) => void;
  showRowNumbers?: boolean;
  rowNumberOffset?: number;
  database?: string;
  tableName?: string;
  newRowIndexes?: Set<number>;
  pendingDeleteIndexes?: Set<number>;
}

export function DataGrid({
  columns,
  columnInfos = [],
  data,
  selectedRowIndex,
  onSelectRow,
  onCellDoubleClick,
  onContextMenu,
  editedCells = {},
  onCellEdit,
  showRowNumbers = false,
  rowNumberOffset = 0,
  database = "",
  tableName = "",
  newRowIndexes = new Set(),
  pendingDeleteIndexes = new Set(),
}: DataGridProps) {
  const [sorting, setSorting] = useState<SortingState>([]);
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
  const resizingRef = useRef<{ col: string; startX: number; startWidth: number } | null>(null);
  const isDraggingRef = useRef(false);
  const resizeRafRef = useRef<number | null>(null);
  const pendingResizeWidthRef = useRef<number | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);
  const widthsInitRef = useRef(false);
  const lastTableKeyRef = useRef("");
  const { t } = useTranslation();

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

  // 编辑框获取焦点
  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus();
      if (editInputRef.current instanceof HTMLInputElement && !["date", "time", "datetime-local"].includes(editInputRef.current.type)) {
        editInputRef.current.select();
      }
    }
  }, [editingCell]);

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
      if (resizingRef.current) {
        const finalWidth = pendingResizeWidthRef.current ??
          colWidths[resizingRef.current.col] ??
          Math.max(MIN_COL_WIDTH, resizingRef.current.startWidth);
        const cacheKey = getCacheKey(database, tableName, resizingRef.current.col);
        colWidthCache.set(cacheKey, finalWidth);
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
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  const handleRowContextMenu = useCallback(
    (e: React.MouseEvent, rowIndex: number, columnName?: string) => {
      onSelectRow(rowIndex);
      gridRef.current?.focus();
      const selection = window.getSelection();
      if (selection && selection.type === "Range") {
        selection.removeAllRanges();
      }
      onContextMenu?.(e, rowIndex, columnName);
    },
    [onContextMenu, onSelectRow]
  );

  return (
    <div
      ref={gridRef}
      className="flex-1 min-h-0 overflow-auto relative scroll-always focus:outline-none"
      tabIndex={0}
      role="grid"
    >
      <table className="w-full border-collapse table-fixed" style={{ minWidth: "max-content" }}>
        <thead className="sticky top-0 z-20">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {showRowNumbers && (
                <th
                  className={cn("data-grid-header border-r border-b text-center sticky top-0 z-20", "border-[var(--border-color)]")}
                  style={{ width: ROW_NUMBER_COL_WIDTH, minWidth: ROW_NUMBER_COL_WIDTH }}
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
                    {/* 列宽拖拽手柄 */}
                    <div
                      className={cn(
                        "absolute right-0 top-0 h-full w-[5px] cursor-col-resize",
                        "after:absolute after:right-[2px] after:top-[25%] after:h-[50%] after:w-[1px]",
                        "after:bg-[var(--border-color)] after:rounded-full after:transition-colors",
                        "hover:after:bg-[var(--accent)] hover:after:w-[2px] hover:after:right-[1.5px]",
                        "active:after:bg-[var(--accent)]"
                      )}
                      onMouseDown={(e) => handleResizeStart(e, header.column.id)}
                    />
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row, rowIndex) => {
            const isSelected = selectedRowIndex === rowIndex;
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
                  isNewRow && !isSelected && "bg-[var(--success)]/8",
                  isPendingDelete && "opacity-40 line-through",
                )}
                onMouseDown={(e) => {
                  if (e.button === 2) {
                    e.preventDefault();
                  }
                }}
                onPointerDown={(e) => {
                  if (e.button !== 0) return;
                  onSelectRow(rowIndex);
                  gridRef.current?.focus();
                }}
              >
                {showRowNumbers && (
                  <td
                    className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]"
                    style={{ width: ROW_NUMBER_COL_WIDTH, minWidth: ROW_NUMBER_COL_WIDTH }}
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
                      onDoubleClick={() => handleDoubleClick(rowIndex, cell.column.id, cell.getValue())}
                      onContextMenu={(e) => handleRowContextMenu(e, rowIndex, cell.column.id)}
                    >
                      {isEditing ? (
                        colMeta.kind === "enum" || colMeta.kind === "date" || colMeta.kind === "time" || colMeta.kind === "datetime" ? (
                          <>
                            <div
                              ref={setEditorAnchorRef}
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
                            className={cn(
                              "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
                              "bg-[var(--surface)] text-[var(--fg)] font-medium",
                              "absolute inset-0 z-20"
                            )}
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            onBlur={commitEdit}
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
              </tr>
            );
          })}
          {data.length === 0 && (
            <tr>
              <td colSpan={columns.length + (showRowNumbers ? 1 : 0)}>
                <div className="absolute inset-0 flex items-center justify-center text-[var(--fg-muted)] text-sm pointer-events-none">
                  {t("common.noData")}
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
