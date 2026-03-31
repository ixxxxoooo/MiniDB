import React, { useState, useCallback } from "react";
import { X, Copy, FileCode, Search, ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import type { ColumnMeta } from "@/types/database";

interface RowPreviewProps {
  row: Record<string, unknown> | null;
  columns?: ColumnMeta[];
  tableName: string;
  onClose: () => void;
  onEdit?: (column: string, value: unknown) => void;
}

export function RowPreview({ row, columns = [], tableName, onClose, onEdit }: RowPreviewProps) {
  const [search, setSearch] = useState("");
  const [editingField, setEditingField] = useState<string | null>(null);
  const [editValue, setEditValue] = useState("");

  if (!row) return null;

  const entries = Object.entries(row);
  const filteredEntries = search
    ? entries.filter(([key]) => key.toLowerCase().includes(search.toLowerCase()))
    : entries;

  const getColumnType = (name: string) => {
    const col = columns.find((c) => c.name === name);
    return col?.type || "";
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
            placeholder="Search for field..."
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

      {/* 字段列表（参考 TablePlus 样式） */}
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

          return (
            <div
              key={key}
              className="px-3 py-1.5 border-b border-[var(--border-subtle)] hover:bg-[var(--row-hover)] transition-colors"
            >
              {/* 字段名和类型 */}
              <div className="flex items-center gap-1.5 mb-0.5">
                <span className="text-xs font-medium text-[var(--fg)]">{key}</span>
                {colType && (
                  <span className="text-2xs text-[var(--fg-muted)] bg-[var(--surface-secondary)] px-1 py-px rounded">
                    {colType}
                  </span>
                )}
              </div>
              {/* 值 */}
              {isEditing ? (
                <div className="flex items-center gap-1">
                  <Input
                    className="h-6 text-xs flex-1"
                    value={editValue}
                    onChange={(e) => setEditValue(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") handleConfirmEdit(key); if (e.key === "Escape") setEditingField(null); }}
                    autoFocus
                  />
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

      {/* 底部导航 */}
      <div className="flex items-center justify-center gap-2 px-3 py-1.5 border-t border-[var(--border-color)]">
        <Button variant="ghost" size="icon" className="h-5 w-5">
          <ChevronLeft className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-5 w-5">
          <ChevronRight className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}
