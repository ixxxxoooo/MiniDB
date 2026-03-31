import React, { useState, useCallback, useRef, useEffect } from "react";
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
}

export function DataGrid({
  columns,
  data,
  selectedRowIndex,
  onSelectRow,
  onCellDoubleClick,
  onContextMenu,
}: DataGridProps) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const tableRef = useRef<HTMLDivElement>(null);

  const tableColumns: ColumnDef<Record<string, unknown>>[] = columns.map(
    (col) => ({
      accessorKey: col.name,
      header: col.name,
      cell: (info) => {
        const value = info.getValue();
        if (value === null || value === undefined) {
          return <span className="text-[var(--fg-muted)] italic">NULL</span>;
        }
        return <span>{String(value)}</span>;
      },
      size: 150,
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
    <div ref={tableRef} className="flex-1 overflow-auto">
      <table className="w-full border-collapse">
        <thead className="sticky top-0 z-10">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {/* 行号列 */}
              <th className="data-grid-header w-12 text-center border-r border-b border-[var(--border-color)]">
                #
              </th>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className={cn(
                    "data-grid-header border-r border-b cursor-pointer hover:bg-[var(--row-hover)]",
                    "border-[var(--border-color)]"
                  )}
                  style={{ width: header.getSize() }}
                  onClick={header.column.getToggleSortingHandler()}
                >
                  <div className="flex items-center gap-1">
                    <span className="truncate">
                      {flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                    </span>
                    {header.column.getIsSorted() === "asc" && (
                      <ArrowUp className="h-3 w-3 flex-shrink-0" />
                    )}
                    {header.column.getIsSorted() === "desc" && (
                      <ArrowDown className="h-3 w-3 flex-shrink-0" />
                    )}
                  </div>
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row, rowIndex) => (
            <tr
              key={row.id}
              className={cn(
                "transition-colors cursor-default",
                selectedRowIndex === rowIndex
                  ? "bg-[var(--row-selected)]"
                  : rowIndex % 2 === 0
                  ? "bg-[var(--surface)]"
                  : "bg-[var(--row-stripe)]",
                selectedRowIndex !== rowIndex && "hover:bg-[var(--row-hover)]"
              )}
              onClick={() => onSelectRow(rowIndex)}
              onContextMenu={(e) => {
                onSelectRow(rowIndex);
                onContextMenu?.(e, rowIndex);
              }}
            >
              {/* 行号 */}
              <td className="data-grid-cell w-12 text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]">
                {rowIndex + 1}
              </td>
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  className="data-grid-cell"
                  onDoubleClick={() =>
                    onCellDoubleClick?.(rowIndex, cell.column.id)
                  }
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
          {data.length === 0 && (
            <tr>
              <td
                colSpan={columns.length + 1}
                className="text-center py-12 text-[var(--fg-muted)] text-sm"
              >
                暂无数据
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
