import React, { useState, useRef, useEffect } from "react";
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
  onContextMenu?: (e: React.MouseEvent, rowIndex: number) => void;
  editedCells?: Record<string, unknown>;
  onCellEdit?: (rowIndex: number, column: string, value: unknown) => void;
}

const MAX_COL_WIDTH = 280;

export function DataGrid({
  columns,
  data,
  selectedRowIndex,
  onSelectRow,
  onContextMenu,
  editedCells = {},
  onCellEdit,
}: DataGridProps) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [editingCell, setEditingCell] = useState<{ row: number; col: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement>(null);

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
              className="w-full h-full bg-transparent border-none outline-none text-xs px-0"
              style={{ background: "var(--cell-edit-bg)" }}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={commitEdit}
              onKeyDown={(e) => {
                if (e.key === "Enter") commitEdit();
                if (e.key === "Escape") cancelEdit();
                if (e.key === "Tab") { e.preventDefault(); commitEdit(); }
              }}
            />
          );
        }

        if (value === null || value === undefined) {
          return <span className="text-[var(--fg-muted)] italic">NULL</span>;
        }
        return <span className="truncate block" style={{ maxWidth: MAX_COL_WIDTH }}>{String(value)}</span>;
      },
      size: 150,
      maxSize: MAX_COL_WIDTH,
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
      <table className="w-full border-collapse">
        <thead className="sticky top-0 z-10">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              <th className="data-grid-header w-10 text-center border-r border-b border-[var(--border-color)]">#</th>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className={cn("data-grid-header border-r border-b cursor-pointer hover:bg-[var(--row-hover)]", "border-[var(--border-color)]")}
                  style={{ width: header.getSize(), maxWidth: MAX_COL_WIDTH }}
                  onClick={header.column.getToggleSortingHandler()}
                >
                  <div className="flex items-center gap-1">
                    <span className="truncate">{flexRender(header.column.columnDef.header, header.getContext())}</span>
                    {header.column.getIsSorted() === "asc" && <ArrowUp className="h-3 w-3 flex-shrink-0" />}
                    {header.column.getIsSorted() === "desc" && <ArrowDown className="h-3 w-3 flex-shrink-0" />}
                  </div>
                </th>
              ))}
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
                onContextMenu={(e) => { onSelectRow(rowIndex); onContextMenu?.(e, rowIndex); }}
              >
                <td className={cn(
                  "data-grid-cell w-10 text-center text-2xs border-r border-[var(--border-subtle)]",
                  isSelected ? "text-white/70" : "text-[var(--fg-muted)]"
                )}>
                  {rowIndex + 1}
                </td>
                {row.getVisibleCells().map((cell) => {
                  const cellKey = `${rowIndex}:${cell.column.id}`;
                  const isEdited = cellKey in editedCells;
                  return (
                    <td
                      key={cell.id}
                      className={cn(
                        "data-grid-cell",
                        isEdited && !isSelected && "border-l-2 border-l-[var(--warning)]"
                      )}
                      style={{ maxWidth: MAX_COL_WIDTH }}
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
              <td colSpan={columns.length + 1} className="text-center py-12 text-[var(--fg-muted)] text-sm">暂无数据</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
