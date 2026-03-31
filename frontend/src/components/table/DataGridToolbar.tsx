import React, { useState } from "react";
import {
  Filter,
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  Code,
  Download,
  Search,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface DataGridToolbarProps {
  tableName: string;
  totalRows: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onRefresh: () => void;
  onOpenQuery: () => void;
  onExport: () => void;
  onFilterChange: (filter: string) => void;
}

export function DataGridToolbar({
  tableName,
  totalRows,
  page,
  pageSize,
  onPageChange,
  onRefresh,
  onOpenQuery,
  onExport,
  onFilterChange,
}: DataGridToolbarProps) {
  const [filterText, setFilterText] = useState("");
  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));

  return (
    <div
      className={cn(
        "flex items-center gap-2 px-3 py-1.5 border-b",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}
    >
      <span className="text-xs font-medium text-[var(--fg-secondary)] truncate">
        {tableName}
      </span>

      <div className="flex-1" />

      {/* 筛选输入 */}
      <div className="relative w-48">
        <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-[var(--fg-muted)]" />
        <Input
          className="h-7 pl-7 text-xs"
          placeholder="筛选..."
          value={filterText}
          onChange={(e) => {
            setFilterText(e.target.value);
            onFilterChange(e.target.value);
          }}
        />
      </div>

      {/* 操作按钮 */}
      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onOpenQuery} title="SQL 查询">
        <Code className="h-3.5 w-3.5" />
      </Button>
      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onExport} title="导出">
        <Download className="h-3.5 w-3.5" />
      </Button>
      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onRefresh} title="刷新">
        <RefreshCw className="h-3.5 w-3.5" />
      </Button>

      {/* 分页 */}
      <div className="flex items-center gap-1 text-xs text-[var(--fg-secondary)]">
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft className="h-3 w-3" />
        </Button>
        <span>
          {page} / {totalPages}
        </span>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          <ChevronRight className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}
