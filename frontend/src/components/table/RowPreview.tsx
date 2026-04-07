import React, { useEffect, useMemo, useState } from "react";
import { X, Copy, FileCode, Search } from "lucide-react";
import { cn, copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useTranslation } from "@/i18n";
import type { ColumnMeta } from "@/types/database";

interface RowPreviewProps {
  row: Record<string, unknown> | null;
  columns?: ColumnMeta[];
  tableName: string;
  onClose: () => void;
  rowKey?: string | number;
  onEdit?: (column: string, value: unknown) => void;
}

const LONG_TEXT_THRESHOLD = 80;

function toEditableString(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function RowPreview({ row, columns = [], tableName, onClose, rowKey, onEdit }: RowPreviewProps) {
  const [search, setSearch] = useState("");
  const [draftValues, setDraftValues] = useState<Record<string, string>>({});
  const [baselineValues, setBaselineValues] = useState<Record<string, string>>({});
  const { t } = useTranslation();

  if (!row) return null;

  // 字段顺序与列定义保持一致
  const orderedEntries: [string, unknown][] = columns.length > 0
    ? columns.map((col) => [col.name, row[col.name]] as [string, unknown])
    : Object.entries(row);
  const filteredEntries = search
    ? orderedEntries.filter(([key]) => key.toLowerCase().includes(search.toLowerCase()))
    : orderedEntries;
  const orderedKeys = useMemo(() => orderedEntries.map(([key]) => key), [orderedEntries]);

  const getColumnType = (name: string) => {
    const col = columns.find((c) => c.name === name);
    return col?.type || "";
  };

  const isLongText = (value: string): boolean => {
    const str = value || "";
    return str.length > LONG_TEXT_THRESHOLD || str.includes("\n");
  };

  useEffect(() => {
    const next: Record<string, string> = {};
    for (const [key, value] of orderedEntries) {
      next[key] = toEditableString(value);
    }
    setDraftValues(next);
    setBaselineValues(next);
  // 只在切换到另一行时重置基线，避免编辑过程中状态闪断
  }, [rowKey]);

  const modifiedSet = useMemo(() => {
    const set = new Set<string>();
    for (const key of orderedKeys) {
      if ((draftValues[key] || "") !== (baselineValues[key] || "")) {
        set.add(key);
      }
    }
    return set;
  }, [baselineValues, draftValues, orderedKeys]);

  const editable = !!onEdit;

  return (
    <div
      className={cn(
        "flex flex-col border-l h-full",
        "bg-[var(--surface)] border-[var(--border-color)]"
      )}
      style={{ width: 300, minWidth: 260 }}
    >
      {/* 头部 */}
      <div className="flex items-center gap-1.5 px-3 py-2 border-b border-[var(--border-color)]">
        <div className="flex-1 flex items-center gap-1">
          <Search className="h-3 w-3 text-[var(--fg-muted)]" />
          <input
            className="flex-1 text-xs bg-transparent outline-none text-[var(--fg)] placeholder-[var(--fg-muted)]"
            placeholder={t("common.search") + "..."}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Tooltip delayDuration={300}>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-5 w-5"
              onClick={() => copyToClipboard(JSON.stringify(row, null, 2))}
            >
              <Copy className="h-3 w-3" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{t("rowPreview.copyJSON")}</TooltipContent>
        </Tooltip>
        <Tooltip delayDuration={300}>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-5 w-5"
              onClick={() => copyToClipboard(rowToInsertSQL(tableName, row))}
            >
              <FileCode className="h-3 w-3" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{t("rowPreview.copyInsert")}</TooltipContent>
        </Tooltip>
        <Tooltip delayDuration={300}>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="h-5 w-5" onClick={onClose}>
              <X className="h-3 w-3" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">{t("common.close")}</TooltipContent>
        </Tooltip>
      </div>

      {/* 字段列表 */}
      <div className="flex-1 overflow-y-auto">
        {filteredEntries.map(([key, value]) => {
          const colType = getColumnType(key);
          const currentValue = draftValues[key] ?? toEditableString(value);
          const longText = isLongText(currentValue);
          const isModified = modifiedSet.has(key);

          return (
            <div
              key={key}
              className="px-3 py-1.5 border-b border-[var(--border-subtle)] hover:bg-[var(--row-hover)] transition-colors"
            >
              <div className="flex items-center gap-1.5 mb-0.5">
                <span className="text-xs font-medium text-[var(--fg)]">{key}</span>
                {colType && (
                  <span className="text-2xs text-[var(--fg-muted)] bg-[var(--surface-secondary)] px-1 py-px rounded">
                    {colType}
                  </span>
                )}
                {editable && isModified && (
                  <span className="text-2xs text-[var(--warning)]">{t("rowPreview.modified")}</span>
                )}
              </div>
              {longText ? (
                <textarea
                  className={cn(
                    "w-full min-h-[80px] max-h-[200px] text-xs rounded border p-1.5 resize-y",
                    "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]",
                    "focus:outline-none focus:ring-1 focus:ring-[var(--accent)]",
                    !editable && "opacity-80 cursor-default"
                  )}
                  value={currentValue}
                  placeholder="NULL"
                  disabled={!editable}
                  onChange={(e) => {
                    const nextValue = e.target.value;
                    setDraftValues((prev) => ({ ...prev, [key]: nextValue }));
                    onEdit?.(key, nextValue);
                  }}
                />
              ) : (
                <input
                  className={cn(
                    "w-full h-7 text-xs rounded border px-1.5",
                    "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]",
                    "focus:outline-none focus:ring-1 focus:ring-[var(--accent)]",
                    !editable && "opacity-80 cursor-default"
                  )}
                  value={currentValue}
                  placeholder="NULL"
                  disabled={!editable}
                  onChange={(e) => {
                    const nextValue = e.target.value;
                    setDraftValues((prev) => ({ ...prev, [key]: nextValue }));
                    onEdit?.(key, nextValue);
                  }}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
