import React, { useState, useRef, useEffect, useCallback, useMemo } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown } from "lucide-react";
import { useTranslation } from "@/i18n";
import type { ColumnMeta } from "@/types/database";

// ====== 列宽计算与缓存 ======

const MIN_COL_WIDTH = 60;
const MAX_COL_WIDTH = 400;
const HEADER_PADDING = 32;
const CELL_PADDING = 24;
const SAMPLE_ROWS = 50;

const colWidthCache = new Map<string, number>();

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

  if (t.includes("bool") || t.includes("tinyint(1)")) {
    return { min: 60, max: 80, fixed: 60 };
  }
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
}

export function DataGrid({
  columns,
  data,
  selectedRowIndex,
  onSelectRow,
  onContextMenu,
  editedCells = {},
  onCellEdit,
  showRowNumbers = false,
  rowNumberOffset = 0,
  database = "",
  tableName = "",
}: DataGridProps) {
  const [sorting, setSorting] = useState<SortingState>([]);
  // 编辑状态独立管理，不进入 useMemo 的 deps
  const [editingCell, setEditingCell] = useState<{ row: number; col: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement>(null);
  const [colWidths, setColWidths] = useState<Record<string, number>>({});
  const resizingRef = useRef<{ col: string; startX: number; startWidth: number } | null>(null);
  const isDraggingRef = useRef(false);
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
      editInputRef.current.select();
    }
  }, [editingCell]);

  const handleDoubleClick = useCallback((rowIndex: number, colName: string, currentValue: unknown) => {
    setEditingCell({ row: rowIndex, col: colName });
    setEditValue(currentValue === null || currentValue === undefined ? "" : String(currentValue));
  }, []);

  // 提交编辑：仅当值实际变化时才通知外部
  const commitEdit = useCallback(() => {
    const cell = editingCellRef.current;
    if (cell) {
      const origValue = data[cell.row]?.[cell.col];
      const origStr = origValue === null || origValue === undefined ? "" : String(origValue);
      if (editValueRef.current !== origStr) {
        onCellEditRef.current?.(cell.row, cell.col, editValueRef.current);
      }
      setEditingCell(null);
    }
  }, [data]);

  const cancelEdit = useCallback(() => setEditingCell(null), []);

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
      setColWidths((prev) => ({ ...prev, [resizingRef.current!.col]: newWidth }));
    };
    const handleMouseUp = () => {
      if (resizingRef.current) {
        const finalWidth = colWidths[resizingRef.current.col] ??
          Math.max(MIN_COL_WIDTH, resizingRef.current.startWidth);
        const cacheKey = getCacheKey(database, tableName, resizingRef.current.col);
        colWidthCache.set(cacheKey, finalWidth);
      }
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
        if (value === null || value === undefined) {
          return <span className="text-[var(--fg-muted)] italic opacity-70">NULL</span>;
        }
        return <span className="truncate block">{String(value)}</span>;
      },
      size: colWidths[col.name] || 150,
    })
  ), [columns, colWidths]);

  const table = useReactTable({
    data,
    columns: tableColumns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="flex-1 overflow-auto relative">
      <table className="w-full border-collapse" style={{ minWidth: "max-content" }}>
        <thead className="sticky top-0 z-10">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {showRowNumbers && (
                <th
                  className={cn("data-grid-header border-r border-b text-center", "border-[var(--border-color)]")}
                  style={{ width: 50, minWidth: 50 }}
                >
                  #
                </th>
              )}
              {headerGroup.headers.map((header) => {
                const w = colWidths[header.column.id] || 150;
                return (
                  <th
                    key={header.id}
                    className={cn("data-grid-header border-r border-b cursor-pointer hover:bg-[var(--row-hover)] relative select-none", "border-[var(--border-color)]")}
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
            return (
              <tr
                key={row.id}
                className={cn(
                  "transition-colors cursor-default",
                  isSelected
                    ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                    : rowIndex % 2 === 0
                      ? "bg-[var(--surface)]"
                      : "bg-[var(--row-stripe)]",
                  !isSelected && "hover:bg-[var(--row-hover)]"
                )}
                onClick={() => onSelectRow(rowIndex)}
                onContextMenu={(e) => {
                  onSelectRow(rowIndex);
                  const target = e.target as HTMLElement;
                  const td = target.closest("td");
                  const cellIdx = td ? Array.from(td.parentElement?.children || []).indexOf(td) - (showRowNumbers ? 1 : 0) : -1;
                  const colName = cellIdx >= 0 ? columns[cellIdx]?.name : undefined;
                  onContextMenu?.(e, rowIndex, colName);
                }}
              >
                {showRowNumbers && (
                  <td
                    className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]"
                    style={{ width: 50, minWidth: 50 }}
                  >
                    {rowNumberOffset + rowIndex + 1}
                  </td>
                )}
                {row.getVisibleCells().map((cell) => {
                  const cellKey = `${rowIndex}:${cell.column.id}`;
                  const isEdited = cellKey in editedCells;
                  const isEditing = editingCell?.row === rowIndex && editingCell?.col === cell.column.id;
                  const w = colWidths[cell.column.id] || 150;
                  return (
                    <td
                      key={cell.id}
                      className={cn(
                        "data-grid-cell overflow-hidden relative",
                        isEdited && "border-l-2 border-l-[var(--warning)] bg-[var(--cell-edit-bg)]/30",
                        isSelected && "text-[var(--fg)] font-medium"
                      )}
                      style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }}
                      onDoubleClick={() => handleDoubleClick(rowIndex, cell.column.id, cell.getValue())}
                    >
                      {isEditing ? (
                        <input
                          ref={editInputRef}
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
              <td colSpan={columns.length + (showRowNumbers ? 1 : 0)} className="text-center py-12 text-[var(--fg-muted)] text-sm">{t("common.noData")}</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
