import React, { useState } from "react";
import {
  X,
  Plus,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { ColumnMeta } from "@/types/database";

/** 筛选条件 */
export interface FilterCondition {
  column: string;
  operator: string;
  value: string;
}

const OPERATORS = [
  { label: "=", value: "=" },
  { label: "!=", value: "!=" },
  { label: ">", value: ">" },
  { label: "<", value: "<" },
  { label: ">=", value: ">=" },
  { label: "<=", value: "<=" },
  { label: "LIKE", value: "LIKE" },
  { label: "NOT LIKE", value: "NOT LIKE" },
  { label: "IS NULL", value: "IS NULL" },
  { label: "IS NOT NULL", value: "IS NOT NULL" },
  { label: "IN", value: "IN" },
];

interface DataGridToolbarProps {
  tableName: string;
  totalRows: number;
  page: number;
  pageSize: number;
  columns?: ColumnMeta[];
  onPageChange: (page: number) => void;
  onRefresh: () => void;
  onOpenQuery: () => void;
  onExport: () => void;
  onFiltersChange: (filters: FilterCondition[]) => void;
  rawSqlFilter?: string;
  onRawSqlChange?: (sql: string) => void;
  onRawSqlExecute?: () => void;
}

export function DataGridToolbar({
  columns = [],
  onRefresh,
  onOpenQuery,
  onExport,
  onFiltersChange,
  rawSqlFilter = "",
  onRawSqlChange,
  onRawSqlExecute,
}: DataGridToolbarProps) {
  const [mode, setMode] = useState<"column" | "rawsql">("column");
  const [filters, setFilters] = useState<FilterCondition[]>([]);

  const addFilter = () => {
    const firstCol = columns[0]?.name || "";
    setFilters((prev) => [...prev, { column: firstCol, operator: "=", value: "" }]);
  };

  const updateFilter = (idx: number, field: keyof FilterCondition, val: string) => {
    setFilters((prev) => {
      const next = [...prev];
      next[idx] = { ...next[idx], [field]: val };
      return next;
    });
  };

  const removeFilter = (idx: number) => {
    const next = filters.filter((_, i) => i !== idx);
    setFilters(next);
    onFiltersChange(next.filter((f) => f.value || f.operator === "IS NULL" || f.operator === "IS NOT NULL"));
  };

  const applyFilters = () => {
    onFiltersChange(filters.filter((f) => f.value || f.operator === "IS NULL" || f.operator === "IS NOT NULL"));
  };

  const clearFilters = () => {
    setFilters([]);
    onFiltersChange([]);
  };

  return (
    <div className="flex-shrink-0 border-b border-[var(--border-color)] bg-[var(--surface)]">
      {/* 模式切换行 */}
      <div className="flex items-center gap-2 px-3 py-1.5">
        <div className="flex items-center gap-0.5 bg-[var(--surface-secondary)] rounded p-0.5">
          <button
            className={cn(
              "px-2 py-0.5 rounded text-xs transition-colors",
              mode === "column" ? "bg-[var(--surface)] shadow-sm text-[var(--fg)] font-medium" : "text-[var(--fg-secondary)]"
            )}
            onClick={() => setMode("column")}
          >
            字段
          </button>
          <button
            className={cn(
              "px-2 py-0.5 rounded text-xs transition-colors",
              mode === "rawsql" ? "bg-[var(--surface)] shadow-sm text-[var(--fg)] font-medium" : "text-[var(--fg-secondary)]"
            )}
            onClick={() => setMode("rawsql")}
          >
            Raw SQL
          </button>
        </div>
        <div className="flex-1" />
      </div>

      {/* 列筛选模式 */}
      {mode === "column" && (
        <div className="px-3 py-1.5 space-y-1.5">
          {filters.map((filter, idx) => (
            <div key={idx} className="flex items-center gap-1.5">
              <select
                className="h-7 text-xs rounded border border-[var(--border-color)] bg-[var(--surface)] text-[var(--fg)] px-2 min-w-[120px]"
                value={filter.column}
                onChange={(e) => updateFilter(idx, "column", e.target.value)}
              >
                {columns.map((col) => <option key={col.name} value={col.name}>{col.name}</option>)}
                <option value="__any">Any column</option>
              </select>
              <select
                className="h-7 text-xs rounded border border-[var(--border-color)] bg-[var(--surface)] text-[var(--fg)] px-2 min-w-[80px]"
                value={filter.operator}
                onChange={(e) => updateFilter(idx, "operator", e.target.value)}
              >
                {OPERATORS.map((op) => <option key={op.value} value={op.value}>{op.label}</option>)}
              </select>
              {filter.operator !== "IS NULL" && filter.operator !== "IS NOT NULL" && (
                <Input
                  className="h-7 text-xs flex-1 min-w-[120px]"
                  placeholder="值..."
                  value={filter.value}
                  onChange={(e) => updateFilter(idx, "value", e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") applyFilters(); }}
                />
              )}
              <Button variant="ghost" size="icon" className="h-7 w-7 flex-shrink-0" onClick={() => removeFilter(idx)}>
                <X className="h-3 w-3" />
              </Button>
            </div>
          ))}
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={addFilter}>
              <Plus className="h-3 w-3 mr-1" /> 添加条件
            </Button>
            <div className="flex-1" />
            {filters.length > 0 && (
              <>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={clearFilters}>清除</Button>
                <Button size="sm" className="h-7 text-xs" onClick={applyFilters}>应用</Button>
              </>
            )}
          </div>
        </div>
      )}

      {/* Raw SQL 模式 */}
      {mode === "rawsql" && (
        <div className="px-3 py-1.5 flex items-center gap-2">
          <Input
            className="h-7 text-xs flex-1 font-mono"
            placeholder="id = 232 或 status = 'active' AND age > 18"
            value={rawSqlFilter}
            onChange={(e) => onRawSqlChange?.(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") onRawSqlExecute?.(); }}
          />
          <Button size="sm" className="h-7 text-xs" onClick={onRawSqlExecute}>执行</Button>
        </div>
      )}
    </div>
  );
}
