import React, { useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

interface ToolResultTableProps {
  columns?: string[];
  rows?: Record<string, unknown>[];
  truncated?: boolean;
  title?: string;
}

const MAX_TABLE_ROWS = 50;

/**
 * 结构化工具结果的表格渲染（协议 v2 tool.result rows）。
 * - 无数据时折叠为摘要行
 * - 超过 MAX_TABLE_ROWS 显示截断提示
 * - 单元格文本由 React 转义，无 XSS 风险
 */
export function ToolResultTable({ columns, rows, truncated: wasTruncated, title }: ToolResultTableProps) {
  const [expanded, setExpanded] = useState(false);

  const cols = useMemo(() => columns ?? [], [columns]);
  const data = useMemo(() => rows ?? [], [rows]);

  if (cols.length === 0) {
    return (
      <div className="text-2xs text-[var(--fg-muted)] px-0.5 py-0.5">
        {title ?? "tool result"}：0 行
      </div>
    );
  }

  const displayRows = data.slice(0, MAX_TABLE_ROWS);
  const isTruncated = data.length > MAX_TABLE_ROWS || !!wasTruncated;

  return (
    <div className="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] overflow-hidden">
      <button
        type="button"
        className="w-full px-2 py-1 flex items-center justify-between gap-2 hover:bg-[var(--surface-secondary)]/80 transition-colors text-left"
        onClick={() => setExpanded((v) => !v)}
      >
        <span className="text-2xs font-medium text-[var(--fg-muted)]">
          {title ?? "tool result"}（{data.length} 行
          {isTruncated ? `，展示前 ${MAX_TABLE_ROWS} 行` : ""}）
        </span>
        {expanded ? <ChevronDown className="h-3 w-3 text-[var(--fg-muted)]" /> : <ChevronRight className="h-3 w-3 text-[var(--fg-muted)]" />}
      </button>

      {expanded && (
        <div className="overflow-x-auto max-h-72">
          <table className="min-w-max w-full text-2xs border-collapse">
            <thead>
              <tr>
                {cols.map((col) => (
                  <th key={col} className="px-2 py-1 bg-[var(--surface)] border-b border-[var(--border-subtle)] whitespace-nowrap text-left font-medium">
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {displayRows.map((row, idx) => (
                <tr key={idx} className={cn(idx % 2 === 1 && "bg-[var(--surface)]/60")}>
                  {cols.map((col) => {
                    const value = row[col];
                    let text = "NULL";
                    if (value !== null && value !== undefined) {
                      text = String(value);
                      if (text.length > 160) text = `${text.slice(0, 160)}…`;
                    }
                    return (
                      <td key={col} className="px-2 py-1 border-b border-[var(--border-subtle)] whitespace-nowrap font-mono">
                        {text}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}