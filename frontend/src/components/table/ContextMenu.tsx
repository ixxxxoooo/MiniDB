import React from "react";
import {
  Copy,
  ClipboardCopy,
  FileCode,
  Trash2,
  RefreshCw,
  Eye,
} from "lucide-react";
import { cn } from "@/lib/utils";

export interface ContextMenuPosition {
  x: number;
  y: number;
}

interface RowContextMenuProps {
  position: ContextMenuPosition | null;
  onClose: () => void;
  onCopyCell: () => void;
  onCopyRow: () => void;
  onCopyAsInsert: () => void;
  onDeleteRow: () => void;
  onRefresh: () => void;
  onPreview: () => void;
}

export function RowContextMenu({
  position,
  onClose,
  onCopyCell,
  onCopyRow,
  onCopyAsInsert,
  onDeleteRow,
  onRefresh,
  onPreview,
}: RowContextMenuProps) {
  if (!position) return null;

  return (
    <>
      {/* 遮罩层 */}
      <div className="fixed inset-0 z-40" onClick={onClose} />

      {/* 菜单 */}
      <div
        className={cn(
          "fixed z-50 min-w-[180px] rounded-lg py-1 shadow-lg border animate-fade-in",
          "bg-[var(--surface-elevated)] border-[var(--border-color)]"
        )}
        style={{ left: position.x, top: position.y }}
      >
        <MenuItem icon={Eye} label="预览行数据" shortcut="Space" onClick={onPreview} />
        <Separator />
        <MenuItem icon={Copy} label="复制单元格" shortcut="⌘C" onClick={onCopyCell} />
        <MenuItem icon={ClipboardCopy} label="复制整行" onClick={onCopyRow} />
        <MenuItem icon={FileCode} label="复制为 INSERT" onClick={onCopyAsInsert} />
        <Separator />
        <MenuItem icon={RefreshCw} label="刷新" shortcut="⌘R" onClick={onRefresh} />
        <Separator />
        <MenuItem
          icon={Trash2}
          label="删除行"
          onClick={onDeleteRow}
          danger
        />
      </div>
    </>
  );
}

function MenuItem({
  icon: Icon,
  label,
  shortcut,
  onClick,
  danger,
}: {
  icon: React.ElementType;
  label: string;
  shortcut?: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      className={cn(
        "w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors",
        danger
          ? "text-[var(--danger)] hover:bg-red-50 dark:hover:bg-red-900/20"
          : "text-[var(--fg)] hover:bg-[var(--row-hover)]"
      )}
      onClick={onClick}
    >
      <Icon className="h-3.5 w-3.5" />
      <span className="flex-1 text-left">{label}</span>
      {shortcut && (
        <span className="text-2xs text-[var(--fg-muted)]">{shortcut}</span>
      )}
    </button>
  );
}

function Separator() {
  return <div className="my-1 h-px bg-[var(--border-subtle)]" />;
}
