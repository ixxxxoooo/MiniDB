import React, { useState } from "react";
import { X, Copy, FileCode, Search, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import type { ColumnMeta } from "@/types/database";

interface RowPreviewProps {
  row: Record<string, unknown> | null;
  columns?: ColumnMeta[];
  tableName: string;
  onClose: () => void;
  onEdit?: (column: string, value: unknown) => void;
}

const LONG_TEXT_THRESHOLD = 80;

export function RowPreview({ row, columns = [], tableName, onClose, onEdit }: RowPreviewProps) {
  const [search, setSearch] = useState("");
  const [editingField, setEditingField] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");

  if (!row) return null;

  // 字段顺序与列定义保持一致
  const orderedEntries: [string, unknown][] = columns.length > 0
    ? columns.map((col) => [col.name, row[col.name]] as [string, unknown])
    : Object.entries(row);
  const filteredEntries = search
    ? orderedEntries.filter(([key]) => key.toLowerCase().includes(search.toLowerCase()))
    : orderedEntries;

  const getColumnType = (name: string) => {
    const col = columns.find((c) => c.name === name);
    return col?.type || "";
  };

  const isLongText = (value: unknown): boolean => {
    if (value === null || value === undefined) return false;
    const str = String(value);
    return str.length > LONG_TEXT_THRESHOLD || str.includes("\n");
  };

  const handleStartEdit = (key: string, value: unknown) => {
    setEditingField(key);
    setEditValue(value === null || value === undefined ? "" : String(value));
  };

  const handleConfirmEdit = (key: string) => {
    onEdit?.(key, editValue);
    setEditingField(null);
  };

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
            placeholder="搜索字段..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Button variant="ghost" size="icon" className="h-5 w-5"
          onClick={() => copyToClipboard(JSON.stringify(row, null, 2))} title="复制为 JSON">
          <Copy className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-5 w-5"
          onClick={() => copyToClipboard(rowToInsertSQL(tableName, row))} title="复制为 INSERT">
          <FileCode className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={onClose}>
          <X className="h-3 w-3" />
        </Button>
      </div>

      {/* 字段列表 */}
      <div className="flex-1 overflow-y-auto">
        {filteredEntries.map(([key, value]) => {
          const colType = getColumnType(key);
          const isNull = value === null || value === undefined;
          const displayValue = isNull
            ? "NULL"
            : typeof value === "object"
              ? JSON.stringify(value)
              : String(value);
          const isEditing = editingField === key;
          const longText = isLongText(value);

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
              </div>
              {isEditing ? (
                <div className="flex flex-col gap-1">
                  {longText || editValue.length > LONG_TEXT_THRESHOLD ? (
                    <textarea
                      className={cn(
                        "w-full min-h-[80px] max-h-[200px] text-xs rounded border p-1.5 resize-y",
                        "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]",
                        "focus:outline-none focus:ring-1 focus:ring-[var(--accent)]"
                      )}
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Escape") setEditingField(null);
                        if ((e.metaKey || e.ctrlKey) && e.key === "Enter") handleConfirmEdit(key);
                      }}
                      autoFocus
                    />
                  ) : (
                    <input
                      className={cn(
                        "w-full h-7 text-xs rounded border px-1.5",
                        "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]",
                        "focus:outline-none focus:ring-1 focus:ring-[var(--accent)]"
                      )}
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleConfirmEdit(key);
                        if (e.key === "Escape") setEditingField(null);
                      }}
                      autoFocus
                    />
                  )}
                  <div className="flex items-center gap-1">
                    <Button
                      size="sm"
                      className="h-5 text-2xs px-2"
                      onClick={() => handleConfirmEdit(key)}
                    >
                      <Check className="h-2.5 w-2.5 mr-0.5" />
                      确认
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-5 text-2xs px-2"
                      onClick={() => setEditingField(null)}
                    >
                      取消
                    </Button>
                  </div>
                </div>
              ) : (
                <div
                  className={cn(
                    "text-xs break-all cursor-text select-text leading-relaxed",
                    isNull ? "text-[var(--fg-muted)] italic" : "text-[var(--fg-secondary)]"
                  )}
                  onDoubleClick={() => onEdit && handleStartEdit(key, value)}
                >
                  {displayValue}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
