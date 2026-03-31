import React from "react";
import { createPortal } from "react-dom";
import {
  Copy,
  ClipboardCopy,
  FileCode,
  Trash2,
  RefreshCw,
  Eye,
  Download,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";

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
  onDownloadPage?: () => void;
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
  onDownloadPage,
}: RowContextMenuProps) {
  const { t } = useTranslation();

  if (!position) return null;

  return createPortal(
    <>
      <div className="fixed inset-0 z-[100]" onClick={onClose} />
      <div
        className={cn(
          "fixed z-[101] min-w-[180px] rounded-[var(--radius-menu)] py-1 shadow-lg border animate-fade-in",
          "bg-[var(--surface-elevated)] border-[var(--border-color)]"
        )}
        style={{ left: position.x, top: position.y }}
      >
        <MenuItem icon={Eye} label={t("contextMenu.previewRow")} shortcut="Space" onClick={onPreview} />
        <Separator />
        <MenuItem icon={Copy} label={t("contextMenu.copyCell")} shortcut="⌘C" onClick={onCopyCell} />
        <MenuItem icon={ClipboardCopy} label={t("contextMenu.copyRow")} onClick={onCopyRow} />
        <MenuItem icon={FileCode} label={t("contextMenu.copyAsInsert")} onClick={onCopyAsInsert} />
        <Separator />
        {onDownloadPage && (
          <>
            <MenuItem icon={Download} label={t("contextMenu.downloadCSV")} onClick={onDownloadPage} />
            <Separator />
          </>
        )}
        <MenuItem icon={RefreshCw} label={t("common.refresh")} shortcut="⌘R" onClick={onRefresh} />
        <Separator />
        <MenuItem icon={Trash2} label={t("contextMenu.deleteRow")} onClick={onDeleteRow} danger />
      </div>
    </>,
    document.body
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
        "w-full flex items-center gap-2 px-3 py-1.5 text-[length:var(--size-font-xs)] transition-colors",
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
