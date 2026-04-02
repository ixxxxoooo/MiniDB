import React, { useState, useRef, useEffect } from "react";
import {
  X,
  Plus,
  Minus,
  ChevronDown,
  Check,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";
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

type FilterMode = "column" | "rawsql";

/**
 * 单行筛选工具栏 — 参考 TablePlus 风格
 * 左侧下拉选择列名或 Raw SQL，右侧输入条件，回车执行
 */
export function DataGridToolbar({
  columns = [],
  onFiltersChange,
  rawSqlFilter = "",
  onRawSqlChange,
  onRawSqlExecute,
}: DataGridToolbarProps) {
  // 当前选中的筛选模式：具体列名 或 Raw SQL
  const [selectedColumn, setSelectedColumn] = useState<string>("__rawsql");
  const [operator, setOperator] = useState("=");
  const [filterValue, setFilterValue] = useState("");
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [operatorDropdownOpen, setOperatorDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const operatorDropdownRef = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();

  const isRawSQL = selectedColumn === "__rawsql";

  // 关闭下拉菜单
  useEffect(() => {
    if (!dropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [dropdownOpen]);

  useEffect(() => {
    if (!operatorDropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (operatorDropdownRef.current && !operatorDropdownRef.current.contains(e.target as Node)) {
        setOperatorDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [operatorDropdownOpen]);

  // 获取当前列显示名
  const displayLabel = isRawSQL
    ? t("datagrid.rawSQL")
    : selectedColumn === "__any"
    ? t("datagrid.anyColumn")
    : selectedColumn;

  const handleApply = () => {
    if (isRawSQL) {
      onRawSqlExecute?.();
    } else if (selectedColumn === "__any") {
      if (filterValue.trim()) {
        onFiltersChange([{ column: "__any", operator, value: filterValue }]);
      }
    } else if (filterValue.trim() || operator === "IS NULL" || operator === "IS NOT NULL") {
      onFiltersChange([{ column: selectedColumn, operator, value: filterValue }]);
    }
  };

  const handleClear = () => {
    setFilterValue("");
    onRawSqlChange?.("");
    onFiltersChange([]);
  };

  return (
    <div className="flex-shrink-0 border-b border-[var(--border-color)] bg-[var(--surface)]">
      <div className="flex items-center gap-2 px-3 py-1.5 min-h-[36px]">
        {/* 列选择下拉 */}
        <div className="relative" ref={dropdownRef}>
          <button
            className={cn(
              "flex items-center gap-[var(--size-gap-sm)] h-[var(--size-btn-sm)] px-1.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] transition-colors",
              "border border-[var(--border-color)] bg-[var(--surface)] text-[var(--fg)]",
              "hover:bg-[var(--surface-secondary)]"
            )}
            onClick={() => setDropdownOpen(!dropdownOpen)}
          >
            <span className="max-w-[80px] truncate">{displayLabel}</span>
            <ChevronDown className="h-2.5 w-2.5 text-[var(--fg-muted)] flex-shrink-0" />
          </button>

          {dropdownOpen && (
            <div
              className={cn(
                "absolute left-0 top-full z-[100] mt-0.5 min-w-[140px] max-h-[280px] overflow-y-auto",
                "py-0.5 rounded-[var(--radius-menu)] shadow-lg border",
                "bg-[var(--surface-elevated)] border-[var(--border-color)]"
              )}
            >
              {/* 列名列表 */}
              {columns.map((col) => (
                <button
                  key={col.name}
                  className={cn(
                    "w-full flex items-center gap-1.5 px-2 py-1 text-[length:var(--size-font-xs)] text-left transition-colors",
                    selectedColumn === col.name
                      ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                      : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                  )}
                  onClick={() => {
                    setSelectedColumn(col.name);
                    setDropdownOpen(false);
                  }}
                >
                  {selectedColumn === col.name && <Check className="h-2.5 w-2.5 flex-shrink-0" />}
                  <span className={selectedColumn !== col.name ? "pl-4" : ""}>{col.name}</span>
                </button>
              ))}

              <div className="h-px bg-[var(--border-subtle)] my-0.5" />

              {/* Any column */}
              <button
                className={cn(
                  "w-full flex items-center gap-1.5 px-2 py-1 text-[length:var(--size-font-xs)] text-left transition-colors",
                  selectedColumn === "__any"
                    ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                    : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={() => {
                  setSelectedColumn("__any");
                  setDropdownOpen(false);
                }}
              >
                {selectedColumn === "__any" && <Check className="h-2.5 w-2.5 flex-shrink-0" />}
                <span className={selectedColumn !== "__any" ? "pl-4" : ""}>{t("datagrid.anyColumn")}</span>
              </button>

              {/* Raw SQL */}
              <button
                className={cn(
                  "w-full flex items-center gap-1.5 px-2 py-1 text-[length:var(--size-font-xs)] text-left transition-colors",
                  isRawSQL
                    ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                    : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={() => {
                  setSelectedColumn("__rawsql");
                  setDropdownOpen(false);
                }}
              >
                {isRawSQL && <Check className="h-2.5 w-2.5 flex-shrink-0" />}
                <span className={!isRawSQL ? "pl-4" : ""}>{t("datagrid.rawSQL")}</span>
              </button>
            </div>
          )}
        </div>

        {/* 非 Raw SQL 模式：操作符下拉 */}
        {!isRawSQL && (
          <div className="relative" ref={operatorDropdownRef}>
            <button
              className={cn(
                "flex items-center gap-[var(--size-gap-sm)] h-[var(--size-btn-sm)] px-1.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] transition-colors",
                "border border-[var(--border-color)] bg-[var(--surface)] text-[var(--fg)]",
                "hover:bg-[var(--surface-secondary)]"
              )}
              onClick={() => setOperatorDropdownOpen(!operatorDropdownOpen)}
            >
              <span>{operator}</span>
              <ChevronDown className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
            </button>

            {operatorDropdownOpen && (
              <div
                className={cn(
                  "absolute left-0 top-full z-[100] mt-0.5 min-w-[100px] max-h-[200px] overflow-y-auto",
                  "py-0.5 rounded-[var(--radius-menu)] shadow-lg border",
                  "bg-[var(--surface-elevated)] border-[var(--border-color)]"
                )}
              >
                {OPERATORS.map((op) => (
                  <button
                    key={op.value}
                    className={cn(
                      "w-full px-2 py-1 text-[length:var(--size-font-xs)] text-left transition-colors",
                      operator === op.value
                        ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                        : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={() => {
                      setOperator(op.value);
                      setOperatorDropdownOpen(false);
                    }}
                  >
                    {op.label}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {/* 输入框 */}
        {isRawSQL ? (
          <Input
            className="h-[var(--size-btn-sm)] text-[length:var(--size-font-2xs)] flex-1 min-w-[120px] rounded-[var(--radius-input)] font-mono"
            placeholder={t("datagrid.rawSQLExample")}
            value={rawSqlFilter}
            onChange={(e) => onRawSqlChange?.(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") onRawSqlExecute?.(); }}
          />
        ) : operator !== "IS NULL" && operator !== "IS NOT NULL" ? (
          <Input
            className="h-[var(--size-btn-sm)] text-[length:var(--size-font-2xs)] flex-1 min-w-[120px] rounded-[var(--radius-input)]"
            placeholder={t("datagrid.value")}
            value={filterValue}
            onChange={(e) => setFilterValue(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleApply(); }}
          />
        ) : (
          <div className="flex-1" />
        )}

        {/* 应用按钮 */}
        <button
          className="px-2 h-[var(--size-btn-sm)] rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-[var(--accent-fg)] bg-[var(--accent)] hover:opacity-90 transition-opacity flex-shrink-0"
          onClick={handleApply}
        >
          {t("common.apply")}
        </button>

        {/* 删除当前筛选条件 */}
        <button
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] flex-shrink-0 border border-[var(--border-color)]"
          onClick={handleClear}
          title={t("common.clear")}
        >
          <Minus className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
        </button>

        {/* 添加新筛选条件（预留） */}
        <button
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] flex-shrink-0 border border-[var(--border-color)]"
          onClick={() => {}}
          title={t("datagrid.addCondition")}
        >
          <Plus className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
        </button>
      </div>
    </div>
  );
}
