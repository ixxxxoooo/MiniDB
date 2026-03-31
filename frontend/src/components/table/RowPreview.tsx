import React from "react";
import { X, Copy, FileCode } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { copyToClipboard, rowToInsertSQL } from "@/lib/utils";

interface RowPreviewProps {
  row: Record<string, unknown> | null;
  tableName: string;
  onClose: () => void;
}

export function RowPreview({ row, tableName, onClose }: RowPreviewProps) {
  if (!row) return null;

  const entries = Object.entries(row);

  const handleCopyAsInsert = () => {
    const sql = rowToInsertSQL(tableName, row);
    copyToClipboard(sql);
  };

  const handleCopyAsJSON = () => {
    copyToClipboard(JSON.stringify(row, null, 2));
  };

  return (
    <div
      className={cn(
        "flex flex-col border-l h-full animate-slide-in-right",
        "bg-[var(--surface)] border-[var(--border-color)]"
      )}
      style={{ width: 320 }}
    >
      {/* 头部 */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)]">
        <span className="text-sm font-medium text-[var(--fg)]">行详情</span>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={handleCopyAsJSON}
            title="复制为 JSON"
          >
            <Copy className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={handleCopyAsInsert}
            title="复制为 INSERT"
          >
            <FileCode className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={onClose}
          >
            <X className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {/* 字段列表 */}
      <div className="flex-1 overflow-y-auto">
        {entries.map(([key, value]) => (
          <div
            key={key}
            className="px-3 py-2 border-b border-[var(--border-subtle)] hover:bg-[var(--row-hover)] transition-colors"
          >
            <div className="text-2xs font-medium text-[var(--fg-secondary)] uppercase tracking-wider mb-0.5">
              {key}
            </div>
            <div
              className={cn(
                "text-sm break-all cursor-text select-text",
                value === null || value === undefined
                  ? "text-[var(--fg-muted)] italic"
                  : "text-[var(--fg)]"
              )}
            >
              {value === null || value === undefined
                ? "NULL"
                : typeof value === "object"
                ? JSON.stringify(value)
                : String(value)}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
