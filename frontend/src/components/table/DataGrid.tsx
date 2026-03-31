import React, { useState, useRef, useEffect, useCallback } from "react";
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
import type { ColumnMeta } from "@/types/database";

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
}

const DEFAULT_COL_WIDTH = 150;
const MIN_COL_WIDTH = 60;

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
}: DataGridProps) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [editingCell, setEditingCell] = useState<{ row: number; col: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement>(null);
  const [colWidths, setColWidths] = useState<Record<string, number>>({});
  const resizingRef = useRef<{ col: string; startX: number; startWidth: number } | null>(null);

  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus();
      editInputRef.current.select();
    }
  }, [editingCell]);

  const handleDoubleClick = (rowIndex: number, colName: string, currentValue: unknown) => {
    setEditingCell({ row: rowIndex, col: colName });
    setEditValue(currentValue === null || currentValue === undefined ? "" : String(currentValue));
  };

  const commitEdit = () => {
    if (editingCell) {
      onCellEdit?.(editingCell.row, editingCell.col, editValue);
      setEditingCell(null);
    }
  };

  const cancelEdit = () => setEditingCell(null);

  // 列宽拖拽
  const handleResizeStart = useCallback((e: React.MouseEvent, colName: string) => {
    e.preventDefault();
    e.stopPropagation();
    const startX = e.clientX;
    const startWidth = colWidths[colName] || DEFAULT_COL_WIDTH;
    resizingRef.current = { col: colName, startX, startWidth };

    const handleMouseMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const diff = ev.clientX - resizingRef.current.startX;
      const newWidth = Math.max(MIN_COL_WIDTH, resizingRef.current.startWidth + diff);
      setColWidths((prev) => ({ ...prev, [resizingRef.current!.col]: newWidth }));
    };
    const handleMouseUp = () => {
      resizingRef.current = null;
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
  }, [colWidths]);

  const tableColumns: ColumnDef<Record<string, unknown>>[] = columns.map(
    (col) => ({
      accessorKey: col.name,
      header: col.name,
      cell: (info) => {
        const value = info.getValue();
        const rowIdx = info.row.index;
        const isEditing = editingCell?.row === rowIdx && editingCell?.col === col.name;

        if (isEditing) {
          return (
            <input
              ref={editInputRef}
              className={cn(
                "w-full h-full bg-[var(--cell-edit-bg)] border-none outline-none text-xs px-1",
                "text-[var(--fg)] font-medium"
              )}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={commitEdit}
              onKeyDown={(e) => {
                if (e.key === "Enter") { e.preventDefault(); commitEdit(); }
                if (e.key === "Escape") cancelEdit();
                if (e.key === "Tab") { e.preventDefault(); commitEdit(); }
              }}
            />
          );
        }

        if (value === null || value === undefined) {
          return <span className="text-[var(--fg-muted)] italic">NULL</span>;
        }
        return <span className="truncate block">{String(value)}</span>;
      },
      size: colWidths[col.name] || DEFAULT_COL_WIDTH,
    })
  );

  const table = useReactTable({
    data,
    columns: tableColumns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="flex-1 overflow-auto">
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
                const w = colWidths[header.column.id] || DEFAULT_COL_WIDTH;
                return (
                  <th
                    key={header.id}
                    className={cn("data-grid-header border-r border-b cursor-pointer hover:bg-[var(--row-hover)] relative", "border-[var(--border-color)]")}
                    style={{ width: w, minWidth: MIN_COL_WIDTH }}
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    <div className="flex items-center gap-1">
                      <span className="truncate">{flexRender(header.column.columnDef.header, header.getContext())}</span>
                      {header.column.getIsSorted() === "asc" && <ArrowUp className="h-3 w-3 flex-shrink-0" />}
                      {header.column.getIsSorted() === "desc" && <ArrowDown className="h-3 w-3 flex-shrink-0" />}
                    </div>
                    {/* 拖拽手柄 */}
                    <div
                      className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize hover:bg-[var(--accent)] opacity-0 hover:opacity-50 transition-opacity"
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
                    ? "bg-[var(--accent)] text-white"
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
                  const w = colWidths[cell.column.id] || DEFAULT_COL_WIDTH;
                  return (
                    <td
                      key={cell.id}
                      className={cn(
                        "data-grid-cell",
                        isEdited && !isSelected && "border-l-2 border-l-[var(--warning)] bg-[var(--cell-edit-bg)]/30"
                      )}
                      style={{ width: w, minWidth: MIN_COL_WIDTH, maxWidth: w }}
                      onDoubleClick={() => handleDoubleClick(rowIndex, cell.column.id, cell.getValue())}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  );
                })}
              </tr>
            );
          })}
          {data.length === 0 && (
            <tr>
              <td colSpan={columns.length + (showRowNumbers ? 1 : 0)} className="text-center py-12 text-[var(--fg-muted)] text-sm">暂无数据</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
